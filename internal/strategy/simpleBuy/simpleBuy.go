package simplebuy

import (
	"context"
	"sync"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/base"
	"github.com/sambly/exchangebot/internal/strategy/sales"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
)

type StrategySimpleBuy struct {
	Config       *Config
	Notification *notification.Notification
	TelegramMenu *StrategySimpleBuyMenu

	AssetsPrices    *prices.AssetsPrices
	OrderController *order.OrderService

	StrategyBaseResult chan base.StrategyBaseResult
	ordersMu           sync.RWMutex
	Orders             map[string][]order.Order
	configMu           sync.RWMutex
	Sale               sales.Sales
}

func NewStrategy(
	notify *notification.Notification,
	assetsPrices *prices.AssetsPrices,
	orderController *order.OrderService,

) (*StrategySimpleBuy, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	str := &StrategySimpleBuy{
		Config:             cfg,
		Notification:       notify,
		AssetsPrices:       assetsPrices,
		OrderController:    orderController,
		StrategyBaseResult: make(chan base.StrategyBaseResult),
		Orders:             make(map[string][]order.Order),
	}

	orderController.AddOrdersDependencies(str.UpdateOrders)
	return str, nil
}

func (s *StrategySimpleBuy) WithTelegramMenu() *StrategySimpleBuy {
	tlgMenu := NewStrategyMenu(s.Config.Name, s.Config.IDName, s)
	s.TelegramMenu = tlgMenu
	return s
}

func (s *StrategySimpleBuy) WithSaleStrategy(sale sales.Sales) *StrategySimpleBuy {
	s.Sale = sale
	return s
}

func (str *StrategySimpleBuy) GetTelegramMenu() model.WindowHandler {
	return str.TelegramMenu
}

// IsStrategyEnabled безопасно читает состояние стратегии
func (str *StrategySimpleBuy) IsStrategyEnabled() bool {
	str.configMu.RLock()
	defer str.configMu.RUnlock()
	return str.Config.StrategyEnable
}

// SetStrategyEnabled безопасно устанавливает состояние стратегии
func (str *StrategySimpleBuy) SetStrategyEnabled(enabled bool) {
	str.configMu.Lock()
	defer str.configMu.Unlock()
	str.Config.StrategyEnable = enabled
}

func (str *StrategySimpleBuy) Start(ctx context.Context) error {
	for {
		select {
		case baseResult := <-str.StrategyBaseResult:
			if str.IsStrategyEnabled() {
				if err := str.execute(ctx, baseResult); err != nil {
					return err
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (str *StrategySimpleBuy) execute(ctx context.Context, baseResult base.StrategyBaseResult) error {
	if str.TelegramMenu == nil {
		return nil
	}
	go func() {
		orderNew, err := str.TelegramMenu.SendMessageBuy(ctx, baseResult)
		if err != nil {
			return
		}

		str.ordersMu.Lock()
		if _, ok := str.Orders[orderNew.Pair]; !ok {
			str.Orders[orderNew.Pair] = []order.Order{orderNew}
		} else {
			str.Orders[orderNew.Pair] = append(str.Orders[orderNew.Pair], orderNew)
		}
		str.ordersMu.Unlock()
	}()

	return nil
}

func (str *StrategySimpleBuy) OnMarket(ms exModel.MarketsStat) {
	if str.Sale == nil {
		return
	}

	if !str.IsStrategyEnabled() {
		return
	}

	str.ordersMu.RLock()
	orders, ok := str.Orders[ms.Pair]
	str.ordersMu.RUnlock()
	if !ok {
		return
	}

	var remainingOrders []order.Order
	for _, order := range orders {
		if !str.Sale.Execute(ms, order) {
			remainingOrders = append(remainingOrders, order)
		}
	}
	str.ordersMu.Lock()
	str.Orders[ms.Pair] = remainingOrders
	str.ordersMu.Unlock()
}

func (str *StrategySimpleBuy) UpdateOrders(updatedOrder order.Order) {

	str.ordersMu.RLock()
	orders, ok := str.Orders[updatedOrder.Pair]
	str.ordersMu.RUnlock()
	if !ok {
		return
	}

	found := false
	var newOrders []order.Order
	for _, ord := range orders {
		if ord.ID == updatedOrder.ID {
			found = true
			if updatedOrder.Status == order.OrderStatusTypeClose {
				// Удаляем ордер только если статус закрыт
				continue
			}
			newOrders = append(newOrders, updatedOrder)
		} else {
			newOrders = append(newOrders, ord)
		}
	}

	// Если ордер не найден по ID, ничего не меняем
	if found {
		str.ordersMu.Lock()
		str.Orders[updatedOrder.Pair] = newOrders
		str.ordersMu.Unlock()
	}
}
