package simpleindicator

import (
	"context"
	"fmt"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	rsitrendcross "github.com/sambly/exchangebot/internal/indicator/rsi_trend_cross"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
)

type Strategy struct {
	config       *Config
	notification *notification.Notification

	pairs        []string
	periods      map[string]time.Duration
	assetsPrices *prices.AssetsPrices
	indicator    *rsitrendcross.Indicator
}

func NewStrategy(
	assetsPrices *prices.AssetsPrices,
	notify *notification.Notification,
	indicator *rsitrendcross.Indicator,
	pairs []string,
) (*Strategy, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	if cfg.AllPairs {
		cfg.Pairs = pairs
	}
	str := &Strategy{
		config:       cfg,
		notification: notify,
		pairs:        cfg.Pairs,
		assetsPrices: assetsPrices,
		indicator:    indicator,
	}
	return str, nil
}

func (str *Strategy) Start(ctx context.Context) error {
	updateAsset := str.assetsPrices.BroadcasterUpdateAssets.Subscribe()
	for {
		select {
		case <-updateAsset:
			// Дожидаемся наверняка записи в базу данных
			time.Sleep(2 * time.Second)
			str.IndicatorCheck()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Strategy) IndicatorCheck() {
	startTime := time.Now()
	for _, pair := range s.pairs {
		candles, err := s.assetsPrices.Repo.GetCandlesBySymbol(pair, "1h", s.indicator.MinBars)
		if err != nil {
			fmt.Printf("❌ Failed to load candles for %s: %v\n", pair, err)
		}

		if s.indicator.CheckData(candles) {
			signalBuy, signalSell := s.indicator.Execute(candles, true)
			if signalBuy || signalSell {
				go s.Notify(signalBuy, signalSell, pair, "1h")
			}
		}

	}
	totalDuration := time.Since(startTime)
	fmt.Printf("\n🕒 Общее время выполнения rsitrendcross(): %v\n", totalDuration)
}

func (s *Strategy) GetTelegramMenu() model.WindowHandler {
	return nil
}

func (s *Strategy) OnMarket(ms exModel.MarketsStat) {}
