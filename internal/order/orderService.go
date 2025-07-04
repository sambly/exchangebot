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
	CreateOrderMarket(deal Deal) (*Order, error)
	ClosePosition(id int64, deal Deal) (*Order, error)
	SetCountOrdersActive(count int)
	GetActiveOrdersBySymbol(symbol string) []*Order
	GetHistoryOrdersBySymbol(symbol string) []*Order
	CalculatePNL() (count int, profit float64)
}

type OrderService struct {
	mtx sync.Mutex

	repo  Repository
	state TradeState

	// orders - все ордера, которые есть в системе
	// ordersDependencies - функции, которые будут вызваны при обновлении ордера(для обновления ордеров в стратегиях)
	orders             []*Order
	ordersDependencies []func(Order)

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

func (os *OrderService) CreateOrderMarket(deal Deal) (Order, error) {
	os.mtx.Lock()
	defer os.mtx.Unlock()

	pair := deal.Pair

	order, err := os.state.CreateOrderMarket(deal)
	if err != nil {
		return Order{}, err
	}

	if err := os.repo.Create(order); err != nil {
		return Order{}, err
	}

	os.orders = append(os.orders, order)

	messageOrder, _ := json.Marshal(map[string]interface{}{"orderAdd": order})
	os.socketsMessage.SendData(messageOrder)

	orderLogger.Debugf("Creating market order for pair: %s, side: %s, size: %f", pair, deal.SideType, deal.Size)

	// Проверить потокобезопаность
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

	if err := os.repo.CreateInfo(orderInfo); err != nil {
		orderLogger.Errorf("error CreateInfo : %v", err)
	}

	return *order, nil
}

func (os *OrderService) ClosePosition(id int64, deal Deal) error {
	os.mtx.Lock()
	defer os.mtx.Unlock()

	order, err := os.state.ClosePosition(id, deal)
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

	os.updateOrdersDependencies(*order)

	messageOrder, _ := json.Marshal(map[string]interface{}{"orderDelete": order})
	os.socketsMessage.SendData(messageOrder)

	return nil
}

func (os *OrderService) OnMarket(ms exModel.MarketsStat) {
	os.mtx.Lock()
	defer os.mtx.Unlock()

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

func (os *OrderService) AddOrdersDependencies(funcDep func(Order)) {
	os.mtx.Lock()
	defer os.mtx.Unlock()

	if funcDep == nil {
		return
	}

	os.ordersDependencies = append(os.ordersDependencies, funcDep)
}

func (os *OrderService) updateOrdersDependencies(order Order) {
	for _, dep := range os.ordersDependencies {
		dep(order)
	}
}
