package order

import (
	"encoding/json"
	"sync"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
)

type Status string

var orderLogger = logger.AddFieldsEmpty()

type Repository interface {
	GetAll() ([]*Order, error)
	Create(o *Order) error
	ClosePosition(id int64, updateData *Order) error
	CreateInfo(ordersInfo *OrderInfo) error
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

	orders []*Order

	assetsPrices   *prices.AsetsPrices
	socketsMessage *notification.SocketsMessage
}

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
		orders:         make([]*Order, 0),
	}

	orders, err := repo.GetAll()
	if err != nil {
		return nil, err
	}

	ctrl.orders = orders

	countOrdersActive := 0
	for _, o := range orders {
		if o.Status == OrderStatusTypeActive {
			state.AddOrderActive(o)
			countOrdersActive++
		}

		if o.Status == OrderStatusTypeClose {
			state.AddOrderHistory(o)
		}
	}

	state.SetCountOrdersActive(countOrdersActive)

	return ctrl, nil
}

func (os *OrderService) CreateOrderMarket(deal Deal, size float64) (*Order, error) {
	os.mtx.Lock()
	defer os.mtx.Unlock()

	pair := deal.Pair

	order, err := os.state.CreateOrderMarket(deal.SideType, pair, size)
	if err != nil {
		return nil, err
	}

	order.Strategy = deal.Strategy

	if err := os.repo.Create(order); err != nil {
		return nil, err
	}

	os.orders = append(os.orders, order)

	messageOrder, _ := json.Marshal(map[string]interface{}{"orderAdd": order})
	os.socketsMessage.SendData(messageOrder)

	orderLogger.Debugf("Creating market order for pair: %s, side: %s, size: %f", pair, deal.SideType, size)

	mkStat := os.assetsPrices.MarketsStat[pair]
	chData := os.assetsPrices.ChangePrices[pair]
	dFast := os.assetsPrices.ChangeDelta[pair]

	mkStatJSON, err := json.Marshal(mkStat)
	if err != nil {
		orderLogger.Errorf("error jsonmarshal mkStatJSON : %v", err)
	}
	chDataJSON, err := json.Marshal(chData)
	if err != nil {
		orderLogger.Errorf("error jsonmarshal chDataJSON : %v", err)
	}
	dFastJSON, err := json.Marshal(dFast)
	if err != nil {
		orderLogger.Errorf("error jsonmarshal dFastJSON : %v", err)
	}

	orderInfo := &OrderInfo{
		IdOrder:      uint(order.ID),
		Frame:        deal.Frame,
		Strategy:     deal.Strategy,
		Comment:      deal.Comment,
		MarketsStat:  mkStatJSON,
		ChangePrices: chDataJSON,
		DeltaFast:    dFastJSON,
	}

	// TODO если не создаться orderInfo , то не будет возвращен order , что не правильно думаю
	if err := os.repo.CreateInfo(orderInfo); err != nil {
		return nil, err
	}

	return order, err
}

func (os *OrderService) ClosePosition(id int64) error {
	os.mtx.Lock()
	defer os.mtx.Unlock()

	order, err := os.state.ClosePosition(id)
	if err != nil {
		return err
	}

	if err := os.repo.ClosePosition(id, order); err != nil {
		return err
	}

	for i, o := range os.orders {
		if o.ID == id {
			os.orders[i] = order
			break
		}
	}

	messageOrder, _ := json.Marshal(map[string]interface{}{"orderDelete": order})
	os.socketsMessage.SendData(messageOrder)

	return nil
}

func (os *OrderService) OnMarket(ms exModel.MarketsStat) {

	activeOrders := os.state.GetActiveOrdersBySymbol(ms.Pair)
	if len(activeOrders) > 0 {
		for _, order := range activeOrders {
			order.Price = ms.Price
			if order.Side == SideTypeBuy {
				order.Profit = (ms.Price / order.PriceCreated * 100) - 100
			}
			if order.Side == SideTypeSell {
				order.Profit = (order.PriceCreated / ms.Price * 100) - 100
			}
			messageOrder, _ := json.Marshal(map[string]interface{}{"orderUpdate": order})
			os.socketsMessage.SendData(messageOrder)
		}
		_, profit := os.state.CalculatePNL()
		messageOrder, _ := json.Marshal(map[string]interface{}{"pnl": profit})
		os.socketsMessage.SendData(messageOrder)
	}
}
