package order

import (
	"encoding/json"
	"sync"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
)

type Status string

var orderLogger = logger.AddFieldsEmpty()

type Repository interface {
	GetAll() ([]Order, error)
	Create(o *Order) error
	ClosePosition(id int64, updateData *Order) error
	CreateInfo(ordersInfo *model.OrderInfo) error
}

type TradeState interface {
	AddOrderActive(o *Order)
	AddOrderHistory(o *Order)
	RemoveOrderActive(pair string, id int64)
	GetOrdersActive() (orders map[string][]*Order)
	GetOrdersHistory() (orders map[string][]*Order)
	CreateOrderMarket(side SideType, pair string, size float64) (*Order, error)
	ClosePosition(id int64) (*Order, error)
	SetCountOrdersActive(count int)
	GetActiveOrdersBySymbol(symbol string) []*Order
	GetHistoryOrdersBySymbol(symbol string) []*Order
	CalculatePNL() (count int, profit float64)
}

type OrderService struct {
	mtx sync.Mutex

	repo  Repository
	state TradeState

	assetsPrices   *prices.AsetsPrices
	socketsMessage *notification.SocketsMessage
}

var ctrLogger = logger.AddFieldsEmpty()

func NewOrderService(
	repo Repository,
	state TradeState,
	socketsMessage *notification.SocketsMessage,
	assetsPrices *prices.AsetsPrices,
) (*OrderService, error) {

	ctrl := &OrderService{
		repo:           repo,
		state:          state,
		assetsPrices:   assetsPrices,
		socketsMessage: socketsMessage,
	}

	orders, err := repo.GetAll()
	if err != nil {
		return nil, err
	}
	countOrdersActive := 0
	for _, o := range orders {
		if o.Status == OrderStatusTypeActive {
			state.AddOrderActive(&o)
			countOrdersActive++
		}

		if o.Status == OrderStatusTypeClose {
			state.AddOrderHistory(&o)
		}

	}

	state.SetCountOrdersActive(countOrdersActive)

	return ctrl, nil

}

func (c *OrderService) CreateOrderMarket(deal model.Deal, size float64) (*Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	pair := deal.Pair

	var sideType SideType
	if deal.SideType == "buy" {
		sideType = SideTypeBuy
	}
	if deal.SideType == "sell" {
		sideType = SideTypeSell
	}

	order, err := c.state.CreateOrderMarket(sideType, pair, size)
	if err != nil {
		return nil, err
	}

	if err := c.repo.Create(order); err != nil {
		return nil, err
	}

	orderLogger.Debugf("Creating market order for pair: %s, side: %s, size: %f", pair, deal.SideType, size)

	mkStat := c.assetsPrices.MarketsStat[pair]
	chData := c.assetsPrices.ChangePrices[pair]
	dFast := c.assetsPrices.ChangeDelta[pair]

	mkStatJSON, err := json.Marshal(mkStat)
	if err != nil {
		ctrLogger.Errorf("error jsonmarshal mkStatJSON : %v", err)
	}
	chDataJSON, err := json.Marshal(chData)
	if err != nil {
		ctrLogger.Errorf("error jsonmarshal chDataJSON : %v", err)
	}
	dFastJSON, err := json.Marshal(dFast)
	if err != nil {
		ctrLogger.Errorf("error jsonmarshal dFastJSON : %v", err)
	}

	orderInfo := &model.OrderInfo{
		IdOrder:      uint(order.ID),
		Frame:        deal.Frame,
		Strategy:     deal.Strategy,
		Comment:      deal.Comment,
		MarketsStat:  mkStatJSON,
		ChangePrices: chDataJSON,
		DeltaFast:    dFastJSON,
	}

	if err := c.repo.CreateInfo(orderInfo); err != nil {
		return nil, err
	}

	return order, err
}

func (c *OrderService) ClosePosition(id int64) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	order, err := c.state.ClosePosition(id)
	if err != nil {
		return err
	}

	if err := c.repo.ClosePosition(id, order); err != nil {
		return err
	}
	return nil
}

func (c *OrderService) OnMarket(ms exModel.MarketsStat) {

	for _, order := range c.state.GetActiveOrdersBySymbol(ms.Pair) {
		order.Price = ms.Price
		if order.Side == SideTypeBuy {
			order.Profit = (ms.Price / order.PriceCreated * 100) - 100
		}
		if order.Side == SideTypeSell {
			order.Profit = (order.PriceCreated / ms.Price * 100) - 100
		}
		// Обновления цены webSocket
		messageOrder, _ := json.Marshal(map[string]interface{}{"order": order})
		c.socketsMessage.SendData(messageOrder)
	}

	// Подсчет PNL
	_, profit := c.state.CalculatePNL()
	messageOrder, _ := json.Marshal(map[string]interface{}{"pnl": profit})
	c.socketsMessage.SendData(messageOrder)

}
