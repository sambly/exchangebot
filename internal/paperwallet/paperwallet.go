package paperwallet

import (
	"errors"
	"fmt"
	"sync"

	"github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/order"
)

var (
	ErrInvalidQuantity = errors.New("invalid quantity")
)

type PaperWallet struct {
	sync.Mutex
	OrdersActive  map[string][]*order.Order
	OrdersHistory map[string][]*order.Order
	MarketsStat   map[string]*model.MarketsStat
	Pnl           *Pnl
}

type Pnl struct {
	CountOrdersActive int
	Profit            float64
}

func NewPaperWallet() *PaperWallet {

	return &PaperWallet{
		OrdersActive:  make(map[string][]*order.Order),
		OrdersHistory: make(map[string][]*order.Order),
		MarketsStat:   make(map[string]*model.MarketsStat),
		Pnl:           &Pnl{},
	}
}

func (p *PaperWallet) AddOrderActive(o *order.Order) {
	if _, ok := p.OrdersActive[o.Pair]; !ok {
		p.OrdersActive[o.Pair] = []*order.Order{o}
	} else {
		p.OrdersActive[o.Pair] = append(p.OrdersActive[o.Pair], o)
	}
}

func (p *PaperWallet) AddOrderHistory(o *order.Order) {
	if _, ok := p.OrdersHistory[o.Pair]; !ok {
		p.OrdersHistory[o.Pair] = []*order.Order{o}
	} else {
		p.OrdersHistory[o.Pair] = append(p.OrdersHistory[o.Pair], o)
	}
}

func (p *PaperWallet) RemoveOrderActive(pair string, id int64) {
	if orders, ok := p.OrdersActive[pair]; ok {
		for i, order := range orders {
			if id == order.ID {
				p.OrdersActive[pair] = append(orders[:i], orders[i+1:]...)
				return
			}
		}
	}
}

func (p *PaperWallet) GetOrdersActive() (orders map[string][]*order.Order) {
	p.Lock()
	defer p.Unlock()
	return p.OrdersActive
}

func (p *PaperWallet) GetOrdersHistory() (orders map[string][]*order.Order) {
	p.Lock()
	defer p.Unlock()
	return p.OrdersHistory
}

func (p *PaperWallet) GetActiveOrdersBySymbol(symbol string) []*order.Order {
	p.Lock()
	defer p.Unlock()
	return p.OrdersActive[symbol]
}

func (p *PaperWallet) GetHistoryOrdersBySymbol(symbol string) []*order.Order {
	p.Lock()
	defer p.Unlock()
	return p.OrdersHistory[symbol]
}

func (p *PaperWallet) CreateOrderMarket(side order.SideType, pair string, size float64) (*order.Order, error) {
	p.Lock()
	defer p.Unlock()

	if size == 0 {
		return &order.Order{}, ErrInvalidQuantity
	}

	marketStat, ok := p.MarketsStat[pair]
	if !ok {
		return nil, fmt.Errorf("market data not available for pair: %s", pair)
	}
	if marketStat == nil {
		return nil, fmt.Errorf("nil market data for pair: %s", pair)
	}

	order := order.Order{
		TimeCreated:  p.MarketsStat[pair].Time,
		Time:         p.MarketsStat[pair].Time,
		Pair:         pair,
		Side:         side,
		Type:         order.OrderTypeMarket,
		Status:       order.OrderStatusTypeActive,
		PriceCreated: p.MarketsStat[pair].Price,
		Price:        p.MarketsStat[pair].Price,
		Quantity:     size,
		Profit:       0,
	}

	p.AddOrderActive(&order)
	return &order, nil
}

func (p *PaperWallet) ClosePosition(id int64) (*order.Order, error) {
	p.Lock()
	defer p.Unlock()

	for pair, orders := range p.OrdersActive {
		for _, o := range orders {
			if o.ID == id {
				if p.MarketsStat[pair].Price == 0 || o.PriceCreated == 0 {
					return nil, fmt.Errorf("error цена пары равна 0")
				}
				o.Time = p.MarketsStat[pair].Time
				o.Status = order.OrderStatusTypeClose
				o.Price = p.MarketsStat[pair].Price
				if o.Side == order.SideTypeBuy {
					o.Profit = (o.Price / o.PriceCreated * 100) - 100
				}
				if o.Side == order.SideTypeSell {
					o.Profit = (o.PriceCreated / o.Price * 100) - 100
				}
				p.AddOrderHistory(o)
				p.RemoveOrderActive(o.Pair, o.ID)

				return o, nil
			}
		}
	}

	return nil, nil
}

func (w *PaperWallet) SetCountOrdersActive(count int) {
	w.Lock()
	defer w.Unlock()

	if w.Pnl == nil {
		w.Pnl = &Pnl{}
	}
	w.Pnl.CountOrdersActive = count
}

// TODO проверить реализацию тяп ляп сделал
func (w *PaperWallet) CalculatePNL() (count int, profit float64) {
	w.Lock()
	defer w.Unlock()

	var totalProfit float64
	countActive := 0

	for _, orders := range w.OrdersActive {
		countActive += len(orders)
		for _, order := range orders {
			totalProfit += order.Profit
		}
	}
	w.Pnl.CountOrdersActive = countActive
	w.Pnl.Profit = totalProfit
	return w.Pnl.CountOrdersActive, w.Pnl.Profit
}
