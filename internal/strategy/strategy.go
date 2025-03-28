package strategy

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sambly/exchangeService/pkg/exchange"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/base"
	simplebuy "github.com/sambly/exchangebot/internal/strategy/simpleBuy"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
)

type Strategy interface {
	Start(ctx context.Context) error
	GetTelegramMenu() model.WindowHandler
}

type Option func(*ControllerStrategy)

type ControllerStrategy struct {
	Strategies      []Strategy
	Notification    *notification.Notification
	Periods         map[string]time.Duration
	Pairs           []string
	AssetsPrices    *prices.AsetsPrices
	OrderController *order.Controller
	PaperWallet     *exchange.PaperWallet
}

func NewControllerStrategy(
	assetsPrices *prices.AsetsPrices,
	periods map[string]time.Duration,
	pairs []string,
	notify *notification.Notification,
	orderController *order.Controller,
	paperWallet *exchange.PaperWallet,
	options ...Option) (*ControllerStrategy, error) {

	ctrlStr := &ControllerStrategy{
		AssetsPrices:    assetsPrices,
		Periods:         periods,
		Pairs:           pairs,
		Notification:    notify,
		OrderController: orderController,
		PaperWallet:     paperWallet,
	}

	for _, option := range options {
		option(ctrlStr)
	}

	if err := ctrlStr.build(); err != nil {
		return nil, err
	}

	return ctrlStr, nil
}

func (cs *ControllerStrategy) build() error {
	baseStrategy, err := base.NewStrategy(cs.AssetsPrices, cs.Periods, cs.Pairs, cs.Notification)
	if err != nil {
		return err
	}
	baseStrategy.WithTelegramMenu()
	cs.AddStrategy(baseStrategy)

	simpleBuyStrategy, err := simplebuy.NewStrategy(cs.Notification, cs.AssetsPrices, cs.OrderController, cs.PaperWallet)
	if err != nil {
		return err
	}
	simpleBuyStrategy.WithTelegramMenu()
	baseStrategy.Subscribe(simpleBuyStrategy.StrategyBaseResult)
	cs.AddStrategy(simpleBuyStrategy)

	return nil
}

func (cs *ControllerStrategy) AddStrategy(strategy Strategy) *ControllerStrategy {
	cs.Strategies = append(cs.Strategies, strategy)
	return cs
}

func (cs *ControllerStrategy) StartAll(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, strategy := range cs.Strategies {
		strategy := strategy
		wg.Add(1)

		go func() {
			defer wg.Done()
			if err := strategy.Start(ctx); err != nil {
				fmt.Printf("Failed to start strategy: %v\n", err)
			}
		}()
	}

	wg.Wait()
	return nil
}
