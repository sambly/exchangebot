package exchange

import (
	"context"
	"fmt"
	"main/model"
	"sync"
)

type PaperWallet struct {
	sync.Mutex
	ctx         context.Context
	Orders      []*model.Order
	MarketsStat map[string]*model.MarketsStat
}

func NewPaperWallet(ctx context.Context) *PaperWallet {

	return &PaperWallet{
		ctx:         ctx,
		Orders:      make([]*model.Order, 0),
		MarketsStat: make(map[string]*model.MarketsStat),
	}
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
		Quantity:     size,
	}

	p.Orders = append(p.Orders, &order)

	return &order, nil
}

func (p *PaperWallet) ClosePosition(id int64) (*model.Order, error) {
	p.Lock()
	defer p.Unlock()

	for _, order := range p.Orders {
		if order.ID == id {
			if p.MarketsStat[order.Pair].Price == 0 || order.PriceCreated == 0 {
				return nil, fmt.Errorf("error цена пары равна 0")
			}
			order.Time = p.MarketsStat[order.Pair].Time
			order.Status = model.OrderStatusTypeClose
			order.Price = p.MarketsStat[order.Pair].Price
			order.Profit = (order.Price / order.PriceCreated * 100) - 100
			return order, nil
		}
	}

	//TODO Сделать ошику , позиция не найдена

	return nil, nil
}
