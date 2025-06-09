package strategy

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sambly/exchangeService/pkg/exchange"
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/logger"
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
	OnMarket(ms exModel.MarketsStat)
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

var strategyLogger = logger.AddFields(map[string]interface{}{
	"package": "strategy",
})

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

func (cs *ControllerStrategy) OnMarket(ms exModel.MarketsStat) {
	for _, strategy := range cs.Strategies {
		strategy.OnMarket(ms)
	}
}

func (cs *ControllerStrategy) StartAll(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, strategy := range cs.Strategies {
		strategy := strategy
		wg.Add(1)

		go func() {
			defer wg.Done()
			if err := strategy.Start(ctx); err != nil && ctx.Err() != context.Canceled {
				strategyLogger.Errorf("Failed strategy: %v\n", err)
			}
		}()
	}

	wg.Wait()

	if ctx.Err() != nil {
		return ctx.Err()
	} else {
		return errors.New("failed all strategies")
	}
}
