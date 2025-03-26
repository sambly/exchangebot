package strategy

import (
	"context"
	"fmt"
	"time"

	"github.com/sambly/exchangeService/pkg/exchange"
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/base"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
)

type Strategy interface {
	OnMarket(ms exModel.MarketsStat)
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
		AssetsPrices: assetsPrices,
		Periods:      periods,
		Pairs:        pairs,
		Notification: notify,
	}

	for _, option := range options {
		option(ctrlStr)
	}

	return ctrlStr, nil
}

func (cs *ControllerStrategy) build() error {
	baseStrategy, err := base.NewStrategy(cs.AssetsPrices, cs.Periods, cs.Pairs, cs.Notification)
	if err != nil {
		return err
	}
	baseStrategy.WithTelegramMenu()
	cs.WithStrategy(baseStrategy)
	return nil
}

func (cs *ControllerStrategy) WithStrategy(strategy Strategy) *ControllerStrategy {
	cs.Strategies = append(cs.Strategies, strategy)
	return cs
}

func (cs *ControllerStrategy) StartAll(ctx context.Context) error {

	if err := cs.build(); err != nil {
		return err
	}
	for _, strategy := range cs.Strategies {
		if err := strategy.Start(ctx); err != nil {
			return fmt.Errorf("failed to start strategy: %w", err)
		}
	}
	return nil
}
