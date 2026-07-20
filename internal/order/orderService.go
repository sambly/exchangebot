package order

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

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

	// ClearSalePolicyForActiveOrders сбрасывает StrategySell у всех активных
	// ордеров. Вызывается один раз при старте (см. NewOrderService): Executor
	// всегда поднимается с пустой картой позиций (см. ARCHITECTURE.md) и не
	// восстанавливает наблюдение за уже открытыми сделками, поэтому значение,
	// записанное при входе в прошлом запуске, вводило бы в заблуждение - будто
	// позицию по-прежнему кто-то ведёт.
	ClearSalePolicyForActiveOrders() error
}

type TradeState interface {
	AddOrderActive(o *Order)
	AddOrderHistory(o *Order)
	GetActiveOrdersBySymbol(symbol string) []*Order
	GetHistoryOrdersBySymbol(symbol string) []*Order
	GetOrdersActiveCopy() (orders map[string][]Order)
	GetOrdersHistoryCopy() (orders map[string][]Order)
	CreateOrderMarket(deal Deal) (*Order, error)
	ClosePosition(id int64, deal Deal) (*Order, error)
	UpdateOrdersPrice(pair string, price float64) []Order
	CalculatePNL() (count int, profit float64)
}

// pushInterval - как часто разрешено пушить обновления ордеров в веб-сокеты.
// OnMarket вызывается на каждом тике каждой пары (сотни раз в секунду), и пуш
// на каждый тик не нужен: интерфейс всё равно не перерисовывается чаще.
const pushInterval = time.Second

type OrderService struct {
	mtx sync.Mutex

	Repo  Repository
	State TradeState

	// orders - все ордера, которые есть в системе
	// ordersDependencies - функции, которые будут вызваны при обновлении ордера(для обновления ордеров в стратегиях)
	Orders             []*Order
	ordersDependencies []func(Order)

	// Троттлинг пушей в веб-сокеты: отдельный мьютекс, чтобы горячий OnMarket
	// не конкурировал с mtx, под которым идут создание и закрытие ордеров.
	pushMu   sync.Mutex
	lastPush map[string]time.Time

	assetsPrices   *prices.AssetsPrices
	socketsMessage *notification.SocketsMessage
}

func NewOrderService(
	repo Repository,
	state TradeState,
	socketsMessage *notification.SocketsMessage,
	assetsPrices *prices.AssetsPrices,
) (*OrderService, error) {

	os := &OrderService{
		Repo:           repo,
		State:          state,
		assetsPrices:   assetsPrices,
		socketsMessage: socketsMessage,
		Orders:         make([]*Order, 0),
		lastPush:       make(map[string]time.Time),
	}

	orders, err := repo.GetAll()
	if err != nil {
		return nil, err
	}

	// Executor всегда поднимается с пустой картой позиций и не восстанавливает
	// наблюдение за уже открытыми сделками (см. ARCHITECTURE.md) - поэтому
	// StrategySell, записанный в прошлом запуске при входе, теперь врёт: будто
	// позицию по-прежнему кто-то ведёт. Чистим и в БД, и в только что
	// загруженных объектах, чтобы UI сразу показал актуальное состояние.
	if err := repo.ClearSalePolicyForActiveOrders(); err != nil {
		orderLogger.Errorf("clear sale policy for active orders: %v", err)
	} else {
		for _, o := range orders {
			if o.Status == OrderStatusTypeActive {
				o.StrategySell = ""
			}
		}
	}

	os.Orders = orders

	for _, o := range orders {
		if o.Status == OrderStatusTypeActive {
			state.AddOrderActive(o)
		}
		if o.Status == OrderStatusTypeClose {
			state.AddOrderHistory(o)
		}
	}
	return os, nil
}

// CreateOrderMarket создаёт ордер. Мьютекс защищает только список os.Orders:
// держать его во время записи в БД и JSON-маршалинга значило бы блокировать всё,
// что этот мьютекс делит - включая закрытие позиций.
func (os *OrderService) CreateOrderMarket(deal Deal) (Order, error) {

	pair := deal.Pair

	order, err := os.State.CreateOrderMarket(deal)
	if err != nil {
		return Order{}, err
	}

	if err := os.Repo.Create(order); err != nil {
		return Order{}, err
	}

	os.mtx.Lock()
	os.Orders = append(os.Orders, order)
	os.mtx.Unlock()

	messageOrder, _ := json.Marshal(map[string]interface{}{"orderAdd": order})
	os.socketsMessage.SendData(messageOrder)

	orderLogger.Debugf("Creating market order for pair: %s, side: %s, size: %f", pair, deal.SideType, deal.Size)

	mkStat, err := os.assetsPrices.GetMarketsStatForPair(pair)
	if err != nil {
		orderLogger.Errorf("error GetMarketsStatForPair : %v", err)
	}
	chData, err := os.assetsPrices.GetChPriceForPair(pair)
	if err != nil {
		orderLogger.Errorf("error GetChPriceForPair : %v", err)
	}
	dFast, err := os.assetsPrices.GetChangeDeltaForPair(pair)
	if err != nil {
		orderLogger.Errorf("error GetChangeDeltaForPair : %v", err)
	}

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
		Executor:     deal.Executor,
		Comment:      deal.Comment,
		MarketsStat:  mkStatJSON,
		ChangePrices: chDataJSON,
		DeltaFast:    dFastJSON,

		Level:      deal.Level,
		Strength:   deal.Strength,
		Volatility: deal.Volatility,
		TakeProfit: deal.TakeProfit,
		StopLoss:   deal.StopLoss,
	}

	if err := os.Repo.CreateInfo(orderInfo); err != nil {
		orderLogger.Errorf("error CreateInfo : %v", err)
	}

	return *order, nil
}

func (os *OrderService) ClosePosition(id int64, deal Deal) error {

	// Атомарность закрытия обеспечивает сам TradeState: он под своим мьютексом
	// убирает ордер из активных, поэтому второй одновременный вызов с тем же id
	// получит ошибку "не найден", а не закроет позицию дважды.
	order, err := os.State.ClosePosition(id, deal)
	if err != nil {
		return err
	}
	// Страховка: реализация TradeState не должна отдавать (nil, nil), но ниже
	// идёт разыменование, и цена ошибки здесь - паника всего процесса.
	if order == nil {
		return fmt.Errorf("close position id=%d: ордер не найден", id)
	}

	if err := os.Repo.ClosePosition(id, order); err != nil {
		return err
	}

	os.mtx.Lock()
	for i, o := range os.Orders {
		if o.ID == id {
			os.Orders[i] = order
			break
		}
	}
	os.mtx.Unlock()

	os.updateOrdersDependencies(*order)

	messageOrder, _ := json.Marshal(map[string]interface{}{"orderDelete": order})
	os.socketsMessage.SendData(messageOrder)

	return nil
}

// OnMarket вызывается из горутины фида на каждом тике каждой пары.
//
// Здесь НЕЛЬЗЯ брать общий os.mtx и нельзя делать ничего блокирующего:
// observer'ы вызываются синхронно внутри чтения WebSocket биржи (см. Notify в
// exchangeService), поэтому задержка тут - это задержка чтения сокета Binance,
// а такие соединения биржа отваливает по таймауту.
func (os *OrderService) OnMarket(ms exModel.MarketsStat) {
	// Мутация ордеров происходит внутри TradeState, под его собственным
	// мьютексом, и возвращает копии - наружу указатели не утекают.
	updated := os.State.UpdateOrdersPrice(ms.Pair, ms.Price)
	if len(updated) == 0 {
		return
	}

	if !os.allowPush(ms.Pair) {
		return
	}

	for i := range updated {
		messageOrder, err := json.Marshal(map[string]interface{}{"orderUpdate": updated[i]})
		if err != nil {
			continue
		}
		os.socketsMessage.SendData(messageOrder)
	}

	_, profit := os.State.CalculatePNL()
	if messagePNL, err := json.Marshal(map[string]interface{}{"pnl": profit}); err == nil {
		os.socketsMessage.SendData(messagePNL)
	}
}

// allowPush троттлит пуш обновлений по паре до одного раза в pushInterval
func (os *OrderService) allowPush(pair string) bool {
	os.pushMu.Lock()
	defer os.pushMu.Unlock()

	now := time.Now()
	if last, ok := os.lastPush[pair]; ok && now.Sub(last) < pushInterval {
		return false
	}
	os.lastPush[pair] = now
	return true
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
	// Снимаем копию списка под мьютексом, а колбэки зовём уже без него: они
	// уходят в стратегии, и держать там наш мьютекс незачем.
	os.mtx.Lock()
	deps := make([]func(Order), len(os.ordersDependencies))
	copy(deps, os.ordersDependencies)
	os.mtx.Unlock()

	for _, dep := range deps {
		dep(order)
	}
}
