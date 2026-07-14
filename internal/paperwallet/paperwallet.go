package paperwallet

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
)

var (
	ErrInvalidQuantity = errors.New("invalid quantity")
	// ErrNoMarketData - по паре ещё не приходило ни одного тика, открывать
	// сделку не по чему: цена и время неизвестны.
	ErrNoMarketData = errors.New("нет рыночных данных по паре")
)

// FeePercent - комиссия за ОДНУ сторону сделки, % (Binance spot taker = 0.1).
// Списывается дважды: вход + выход.
//
// Без неё бумажный кошелёк систематически врал в плюс на ~0.2% за круг: сделка,
// закрытая по тейку +0.5%, отчитывалась как +0.5%, хотя реальная биржа отдала бы
// +0.3%. На стратегиях с частыми сделками и узкими тейками это разница между
// "слегка плюсовая" и "убыточная" - именно её и должен показывать форвард-тест.
const FeePercent = 0.1

// netProfit - профит ПОСЛЕ комиссий обеих сторон, в процентах от входа.
//
// Открытым позициям тоже показывается чистый результат "если закрыть сейчас":
// комиссия входа уже уплачена, комиссия выхода неизбежна, и прятать их до
// закрытия значило бы показывать в интерфейсе прибыль, которой нет.
func netProfit(grossPercent float64) float64 {
	return grossPercent - 2*FeePercent
}

type PaperWallet struct {
	sync.Mutex
	ordersActive  map[string][]*order.Order
	ordersHistory map[string][]*order.Order
	assetsPrices  *prices.AssetsPrices
	pnl           *pnl
}

type pnl struct {
	countOrdersActive int
	profit            float64
}

func NewPaperWallet(
	assetsPrices *prices.AssetsPrices,
) *PaperWallet {

	return &PaperWallet{
		ordersActive:  make(map[string][]*order.Order),
		ordersHistory: make(map[string][]*order.Order),
		assetsPrices:  assetsPrices,
		pnl:           &pnl{},
	}
}

func (p *PaperWallet) AddOrderActive(o *order.Order) {
	p.Lock()
	defer p.Unlock()
	p.addOrderActive(o)
}

func (p *PaperWallet) addOrderActive(o *order.Order) {
	if o == nil {
		return
	}
	if _, ok := p.ordersActive[o.Pair]; !ok {
		p.ordersActive[o.Pair] = []*order.Order{o}
	} else {
		p.ordersActive[o.Pair] = append(p.ordersActive[o.Pair], o)
	}
}

func (p *PaperWallet) AddOrderHistory(o *order.Order) {
	p.Lock()
	defer p.Unlock()
	p.addOrderHistory(o)
}

func (p *PaperWallet) addOrderHistory(o *order.Order) {
	if o == nil {
		return
	}
	if _, ok := p.ordersHistory[o.Pair]; !ok {
		p.ordersHistory[o.Pair] = []*order.Order{o}
	} else {
		p.ordersHistory[o.Pair] = append(p.ordersHistory[o.Pair], o)
	}
}

func (p *PaperWallet) removeOrderActive(pair string, id int64) {
	if orders, ok := p.ordersActive[pair]; ok {
		for i, order := range orders {
			if id == order.ID {
				p.ordersActive[pair] = append(orders[:i], orders[i+1:]...)
				return
			}
		}
	}
}

func (p *PaperWallet) GetOrdersActiveCopy() map[string][]order.Order {
	p.Lock()
	defer p.Unlock()

	ordersCopy := make(map[string][]order.Order, len(p.ordersActive))
	for symbol, orders := range p.ordersActive {
		symbolOrders := make([]order.Order, len(orders))
		for i, o := range orders {
			symbolOrders[i] = *o
		}
		ordersCopy[symbol] = symbolOrders
	}
	return ordersCopy
}

func (p *PaperWallet) GetOrdersHistoryCopy() map[string][]order.Order {
	p.Lock()
	defer p.Unlock()

	ordersCopy := make(map[string][]order.Order, len(p.ordersHistory))
	for symbol, orders := range p.ordersHistory {
		symbolOrders := make([]order.Order, len(orders))
		for i, o := range orders {
			symbolOrders[i] = *o
		}
		ordersCopy[symbol] = symbolOrders
	}
	return ordersCopy
}

func (p *PaperWallet) GetActiveOrdersBySymbol(symbol string) []*order.Order {
	p.Lock()
	defer p.Unlock()
	return p.ordersActive[symbol]
}

func (p *PaperWallet) GetHistoryOrdersBySymbol(symbol string) []*order.Order {
	p.Lock()
	defer p.Unlock()
	return p.ordersHistory[symbol]
}

func (p *PaperWallet) CreateOrderMarket(deal order.Deal) (*order.Order, error) {
	p.Lock()
	defer p.Unlock()

	pair := deal.Pair
	size := deal.Size
	side := deal.SideType
	strategy := deal.Strategy

	if size == 0 {
		return nil, ErrInvalidQuantity
	}

	marketStat, err := p.assetsPrices.GetMarketsStatForPair(pair)
	if err != nil {
		return nil, err
	}

	// По паре ещё не приходило ни одного тика: цена и время нулевые.
	//
	// Раньше такой ордер молча создавался с ценой 0 и датой 0000-00-00, а падал
	// уже в MySQL ("Incorrect datetime value") - причём ошибка не доходила до
	// интерфейса, и сделка просто "не появлялась".
	if marketStat.Price == 0 || marketStat.Time.IsZero() {
		return nil, fmt.Errorf("%w: %s", ErrNoMarketData, pair)
	}

	now := time.Now()

	order := order.Order{
		// Время СОЗДАНИЯ - это момент сделки по нашим часам, а не время события
		// на бирже: последнее относится к цене, а не к ордеру.
		TimeCreated:  now,
		Time:         now,
		Pair:         pair,
		Side:         side,
		Type:         order.OrderTypeMarket,
		Status:       order.OrderStatusTypeActive,
		PriceCreated: marketStat.Price,
		Price:        marketStat.Price,
		Quantity:     size,
		Profit:       0,
		StrategyBuy:  strategy,
		Executor:     deal.Executor,
	}

	p.addOrderActive(&order)
	return &order, nil
}

func (p *PaperWallet) ClosePosition(id int64, deal order.Deal) (*order.Order, error) {
	p.Lock()
	defer p.Unlock()

	for pair, orders := range p.ordersActive {
		for _, o := range orders {
			if o.ID == id {

				marketStat, err := p.assetsPrices.GetMarketsStatForPair(pair)
				if err != nil {
					return &order.Order{}, err
				}

				if marketStat.Price == 0 || o.PriceCreated == 0 {
					return nil, fmt.Errorf("error цена пары равна 0")
				}
				o.Time = marketStat.Time
				o.Status = order.OrderStatusTypeClose
				o.Price = marketStat.Price
				if o.Side == order.SideTypeBuy {
					o.Profit = netProfit((o.Price / o.PriceCreated * 100) - 100)
				}
				if o.Side == order.SideTypeSell {
					o.Profit = netProfit((o.PriceCreated / o.Price * 100) - 100)
				}
				o.StrategySell = deal.Strategy
				o.ExitReason = deal.ExitReason
				p.addOrderHistory(o)
				p.removeOrderActive(o.Pair, o.ID)

				return o, nil
			}
		}
	}

	// Раньше здесь возвращалось (nil, nil) - "ошибки нет, но и ордера нет".
	// Вызывающий код это принимал за успех и разыменовывал nil.
	return nil, fmt.Errorf("активный ордер id=%d не найден", id)
}

// UpdateOrdersPrice проставляет активным ордерам пары текущую цену и профит,
// возвращая КОПИИ обновлённых ордеров.
//
// Раньше это делал OrderService: он брал у кошелька указатели на ордера и писал
// в них, не держа его мьютекс, - то есть гонка с GetOrdersActiveCopy. Мутация
// должна происходить там же, где живёт блокировка, а наружу уходить копии.
func (p *PaperWallet) UpdateOrdersPrice(pair string, price float64) []order.Order {
	p.Lock()
	defer p.Unlock()

	orders := p.ordersActive[pair]
	if len(orders) == 0 {
		return nil
	}

	updated := make([]order.Order, 0, len(orders))
	for _, o := range orders {
		o.Price = price

		if o.PriceCreated != 0 && price != 0 {
			switch o.Side {
			case order.SideTypeBuy:
				o.Profit = netProfit((price / o.PriceCreated * 100) - 100)
			case order.SideTypeSell:
				o.Profit = netProfit((o.PriceCreated / price * 100) - 100)
			}
		}

		updated = append(updated, *o)
	}
	return updated
}

func (p *PaperWallet) CalculatePNL() (count int, profit float64) {
	p.Lock()
	defer p.Unlock()

	countActive := 0
	totalProfit := 0.0

	for _, orders := range p.ordersActive {
		countActive += len(orders)
		for _, order := range orders {
			totalProfit += order.Profit
		}
	}
	p.pnl.countOrdersActive = countActive
	p.pnl.profit = totalProfit
	return countActive, totalProfit
}
