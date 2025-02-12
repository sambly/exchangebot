package base

import (
	"context"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
)

type Strategy struct {
	Config       *Config
	Notification *notification.Notification

	Periods      map[string]time.Duration
	AssetsPrices *prices.AsetsPrices
}

func NewStrategy(assetsPrices *prices.AsetsPrices, periods map[string]time.Duration, pairs []string, notify *notification.Notification) (*Strategy, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	if cfg.AllPairs {
		cfg.Pairs = pairs
	}

	str := &Strategy{
		AssetsPrices: assetsPrices,
		Periods:      periods,
		Config:       cfg,
		Notification: notify,
	}
	return str, nil
}

func (str *Strategy) Start(ctx context.Context) error {

	for {
		select {
		case <-str.AssetsPrices.UpdateChanel:
			str.changePrices()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (str *Strategy) changePrices() {

	if !str.Config.NotificationEnable {
		return
	}

	for _, pair := range str.Config.Pairs {
		for period, _ := range str.Periods {
			assets := str.AssetsPrices

			if _, ok := assets.ChangePricesDataset[pair]; !ok {
				break
			}

			if assets.ChangePricesDataset[pair][period].Fill {
				// Отправка сообщения об изменении цены
				if assets.ChangePrices[pair][period].ChangePercent >= str.Config.WeightProcents[period] {
					str.NotificationWeightPercent(pair, period, assets.ChangePrices[pair][period].ChangePercent)
				}
			}
		}
	}

}

func (str *Strategy) OnMarket(ms exModel.MarketsStat) {}
