package simplebuy

import (
	"context"
	"fmt"

	"github.com/sambly/exchangeService/pkg/exchange"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/base"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
)

type StrategySimpleBuy struct {
	Config       *Config
	Notification *notification.Notification
	TelegramMenu *StrategySimpleBuyMenu

	AssetsPrices    *prices.AsetsPrices
	OrderController *order.Controller

	StrategyBaseResult chan base.StrategyBaseResult
}

func NewStrategy(
	notify *notification.Notification,
	assetsPrices *prices.AsetsPrices,
	orderController *order.Controller,
	paperWallet *exchange.PaperWallet,

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
	}
	return str, nil
}

func (s *StrategySimpleBuy) WithTelegramMenu() *StrategySimpleBuy {
	tlgMenu := NewStrategyMenu("SimpleBuy стратегия", "strategiesSimpleBuy", s)
	s.TelegramMenu = tlgMenu
	return s
}

func (str *StrategySimpleBuy) GetTelegramMenu() model.WindowHandler {
	return str.TelegramMenu
}

func (str *StrategySimpleBuy) Start(ctx context.Context) error {
	for {
		select {
		case baseResult := <-str.StrategyBaseResult:
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

	order, err := str.TelegramMenu.SendMessageBuy(baseResult)
	if err != nil {
		fmt.Println("ОШИБКА ЖЕ ЕСТЬ")
		return err
	}

	if order == nil {
		fmt.Println("Таймаут - ордер не создан")
	} else {
		fmt.Println("Ордер создан:", order)
	}

	return nil
}
