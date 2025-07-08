package paperwallet

import (
	"errors"
	"fmt"
	"sync"

	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
)

var (
	ErrInvalidQuantity = errors.New("invalid quantity")
)

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
		return &order.Order{}, ErrInvalidQuantity
	}
	// TODO здесь мне не очень нравится что данные берем с marketStat, актуальные они точно? или может другой способ сделать
	// плюс не совсем корректно брать и время от туда , короче надо изучить
	marketStat, err := p.assetsPrices.GetMarketsStatForPair(pair)
	if err != nil {
		return &order.Order{}, err
	}

	// TODO Ну и вообще брать данные с MarketsStat потокобезопасно?
	// ну время точно поменять надо
	order := order.Order{
		TimeCreated:  marketStat.Time,
		Time:         marketStat.Time,
		Pair:         pair,
		Side:         side,
		Type:         order.OrderTypeMarket,
		Status:       order.OrderStatusTypeActive,
		PriceCreated: marketStat.Price,
		Price:        marketStat.Price,
		Quantity:     size,
		Profit:       0,
		StrategyBuy:  strategy,
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
					o.Profit = (o.Price / o.PriceCreated * 100) - 100
				}
				if o.Side == order.SideTypeSell {
					o.Profit = (o.PriceCreated / o.Price * 100) - 100
				}
				o.StrategySell = deal.Strategy
				p.addOrderHistory(o)
				p.removeOrderActive(o.Pair, o.ID)

				return o, nil
			}
		}
	}

	return nil, nil
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
