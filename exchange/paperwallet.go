package exchange

import (
	"context"
	"fmt"
	"main/model"
	"sync"
)

type PaperWallet struct {
	sync.Mutex
	ctx           context.Context
	OrdersActive  map[string][]*model.Order
	OrdersHistory map[string][]*model.Order
	MarketsStat   map[string]*model.MarketsStat
	Pnl           *Pnl
}

type Pnl struct {
	CountOrdersActive int
	Profit            float64
}

func NewPaperWallet(ctx context.Context) *PaperWallet {

	return &PaperWallet{
		ctx:           ctx,
		OrdersActive:  make(map[string][]*model.Order),
		OrdersHistory: make(map[string][]*model.Order),
		MarketsStat:   make(map[string]*model.MarketsStat),
		Pnl:           &Pnl{},
	}
}

func (p *PaperWallet) AddOrderActive(order *model.Order) {
	if _, ok := p.OrdersActive[order.Pair]; !ok {
		p.OrdersActive[order.Pair] = []*model.Order{order}
	} else {
		p.OrdersActive[order.Pair] = append(p.OrdersActive[order.Pair], order)
	}
}

func (p *PaperWallet) AddOrderHistory(order *model.Order) {
	if _, ok := p.OrdersHistory[order.Pair]; !ok {
		p.OrdersHistory[order.Pair] = []*model.Order{order}
	} else {
		p.OrdersHistory[order.Pair] = append(p.OrdersHistory[order.Pair], order)
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

func (p *PaperWallet) GetOrdersActive() (orders map[string][]*model.Order) {
	return p.OrdersActive
}

func (p *PaperWallet) GetOrdersHistory() (orders map[string][]*model.Order) {
	return p.OrdersHistory
}

func (p *PaperWallet) CreateOrderMarket(side model.SideType, pair string, size float64) (*model.Order, error) {
	p.Lock()
	defer p.Unlock()

	if size == 0 {
		return &model.Order{}, ErrInvalidQuantity
	}

	order := model.Order{
		TimeCreated:  p.MarketsStat[pair].Time,
		Time:         p.MarketsStat[pair].Time,
		Pair:         pair,
		Side:         side,
		Type:         model.OrderTypeMarket,
		Status:       model.OrderStatusTypeActive,
		PriceCreated: p.MarketsStat[pair].Price,
		Price:        p.MarketsStat[pair].Price,
		Quantity:     size,
		Profit:       0,
	}

	p.AddOrderActive(&order)
	return &order, nil
}

func (p *PaperWallet) ClosePosition(id int64) (*model.Order, error) {
	p.Lock()
	defer p.Unlock()

	for pair, orders := range p.OrdersActive {
		for _, order := range orders {
			if order.ID == id {
				if p.MarketsStat[pair].Price == 0 || order.PriceCreated == 0 {
					return nil, fmt.Errorf("error цена пары равна 0")
				}
				order.Time = p.MarketsStat[pair].Time
				order.Status = model.OrderStatusTypeClose
				order.Price = p.MarketsStat[pair].Price
				if order.Side == model.SideTypeBuy {
					order.Profit = (order.Price / order.PriceCreated * 100) - 100
				}
				if order.Side == model.SideTypeSell {
					order.Profit = (order.PriceCreated / order.Price * 100) - 100
				}
				p.AddOrderHistory(order)
				p.RemoveOrderActive(order.Pair, order.ID)

				return order, nil
			}
		}
	}

	return nil, nil
}
