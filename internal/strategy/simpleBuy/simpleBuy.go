package simplebuy

import (
	"context"
	"fmt"

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

	AssetsPrices    *prices.AsetsPrices
	OrderController *order.OrderService

	StrategyBaseResult chan base.StrategyBaseResult
	Orders             map[string][]*order.Order
	Sale               sales.Sales
}

func NewStrategy(
	notify *notification.Notification,
	assetsPrices *prices.AsetsPrices,
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
		Orders:             make(map[string][]*order.Order),
	}
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

func (str *StrategySimpleBuy) Start(ctx context.Context) error {
	for {
		select {
		case baseResult := <-str.StrategyBaseResult:
			// TODO добавить сюда ctx
			if err := str.execute(baseResult); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (str *StrategySimpleBuy) execute(baseResult base.StrategyBaseResult) error {
	if str.TelegramMenu == nil {
		return nil
	}
	go func() {
		orderNew, err := str.TelegramMenu.SendMessageBuy(baseResult)
		if err != nil {
			fmt.Printf("Ошибка SendMessageBuy %v\n", err)
			return
		}

		if orderNew == nil {
			fmt.Println("Таймаут - ордер не создан")
		} else {
			if _, ok := str.Orders[orderNew.Pair]; !ok {
				str.Orders[orderNew.Pair] = []*order.Order{orderNew}
			} else {
				str.Orders[orderNew.Pair] = append(str.Orders[orderNew.Pair], orderNew)
			}

			fmt.Println("Ордер создан:", orderNew)
		}
	}()

	return nil
}

func (str *StrategySimpleBuy) OnMarket(ms exModel.MarketsStat) {

	if str.Sale == nil {
		return
	}

	orders, ok := str.Orders[ms.Pair]
	if !ok {
		return
	}

	var remainingOrders []*order.Order
	// если позиция закрыта удаляем ордер из слайса ордеров стратегии
	for _, order := range orders {
		if !str.Sale.Execute(ms, order) {
			remainingOrders = append(remainingOrders, order)
		}
	}

	str.Orders[ms.Pair] = remainingOrders

}
