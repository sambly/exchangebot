package strategy

import (
	"context"
	"errors"
	"sync"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/anomaly"
	"github.com/sambly/exchangebot/internal/strategy/base"
	"github.com/sambly/exchangebot/internal/strategy/executor"
	"github.com/sambly/exchangebot/internal/strategy/sales/simplesale"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
)

type Strategy interface {
	Start(ctx context.Context) error
	GetTelegramMenu() model.WindowHandler
	OnMarket(ms exModel.MarketsStat)
}

type Option func(*ControllerStrategy)

type ControllerStrategy struct {
	TelegramEnable  bool
	Strategies      []Strategy
	Notification    *notification.Notification
	Periods         map[string]time.Duration
	Pairs           []string
	AssetsPrices    *prices.AssetsPrices
	OrderController *order.OrderService
}

var strategyLogger = logger.AddFields(map[string]interface{}{
	"package": "strategy",
})

func NewControllerStrategy(
	cfg *config.Config,
	assetsPrices *prices.AssetsPrices,
	periods map[string]time.Duration,
	pairs []string,
	notify *notification.Notification,
	orderController *order.OrderService,
	options ...Option) (*ControllerStrategy, error) {

	ctrlStr := &ControllerStrategy{
		TelegramEnable:  cfg.Telegram.Enable,
		AssetsPrices:    assetsPrices,
		Periods:         periods,
		Pairs:           pairs,
		Notification:    notify,
		OrderController: orderController,
	}

	for _, option := range options {
		option(ctrlStr)
	}

	if err := ctrlStr.build(); err != nil {
		return nil, err
	}

	return ctrlStr, nil
}

// build собирает граф стратегий.
//
// Три роли, и они не пересекаются:
//   - ДЕТЕКТОРЫ (base, anomaly) - находят события и публикуют сигналы;
//   - ИСПОЛНИТЕЛЬ (executor) - решает, открывать ли позицию по сигналу;
//   - ПОЛИТИКА ВЫХОДА (simplesale) - решает, когда закрывать.
//
// Кто на кого подписан, решается ЗДЕСЬ, а не внутри пакетов. Поэтому новый
// детектор не тащит за собой торговую логику, а новая торговая логика не
// трогает детекторы: executor не импортирует ни anomaly, ни base - только
// signal.
func (cs *ControllerStrategy) build() error {

	baseStrategy, err := base.NewStrategy(cs.AssetsPrices, cs.Periods, cs.Pairs, cs.Notification)
	if err != nil {
		return err
	}
	if cs.TelegramEnable {
		baseStrategy.WithTelegramMenu()
	}
	cs.AddStrategy(baseStrategy)

	anomalyStrategy, err := anomaly.NewStrategy(cs.AssetsPrices, cs.Periods, cs.Pairs, cs.Notification)
	if err != nil {
		return err
	}
	cs.AddStrategy(anomalyStrategy)

	tradeExecutor, err := executor.New(cs.Notification, cs.OrderController)
	if err != nil {
		return err
	}
	if cs.TelegramEnable {
		tradeExecutor.WithTelegramMenu()
	}

	exitPolicy, err := simplesale.NewStrategy(cs.OrderController)
	if err != nil {
		return err
	}
	tradeExecutor.WithSaleStrategy(exitPolicy)

	// Исполнитель слушает оба детектора. Какие сигналы он реально берёт в работу -
	// решают фильтры в его конфиге (sources, minLevel, onUp/onDown).
	baseStrategy.Subscribe(tradeExecutor.Signals)
	anomalyStrategy.Subscribe(tradeExecutor.Signals)

	cs.AddStrategy(tradeExecutor)

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
