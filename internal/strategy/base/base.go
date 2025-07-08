package base

import (
	"context"
	"fmt"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
)

type StrategyBase struct {
	Config       *Config
	Notification *notification.Notification
	TelegramMenu *StrategyBaseMenu

	Periods      map[string]time.Duration
	AssetsPrices *prices.AssetsPrices

	subscribers []chan StrategyBaseResult
}

type StrategyBaseResult struct {
	//TODO  Executed зачем ?
	Executed bool
	Data     BaseResult
}

type BaseResult struct {
	Pair          string
	Period        string
	ChangePercent float64
}

func NewStrategy(assetsPrices *prices.AssetsPrices, periods map[string]time.Duration, pairs []string, notify *notification.Notification) (*StrategyBase, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	if cfg.AllPairs {
		cfg.Pairs = pairs
	}

	str := &StrategyBase{
		AssetsPrices: assetsPrices,
		Periods:      periods,
		Config:       cfg,
		Notification: notify,
	}
	return str, nil
}

func (s *StrategyBase) WithTelegramMenu() *StrategyBase {
	tlgMenu := NewStrategyMenu(s.Config.Name, s.Config.IDName, s)
	s.TelegramMenu = tlgMenu
	return s
}

func (str *StrategyBase) Start(ctx context.Context) error {

	for {
		select {
		case <-str.AssetsPrices.UpdateChanel:
			str.changePrices()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (str *StrategyBase) changePrices() {

	if !str.Config.StrategyEnable {
		return
	}

	for _, pair := range str.Config.Pairs {
		for period := range str.Periods {
			assets := str.AssetsPrices

			if _, ok := assets.ChangePricesDataset[pair]; !ok {
				break
			}

			if assets.ChangePricesDataset[pair][period].Fill {
				if assets.ChangePrices[pair][period].ChangePercent >= str.Config.WeightProcents[period] {
					// Отправка сообщения об изменении цены
					if str.Config.NotificationEnable {
						str.NotificationWeightPercent(pair, period, assets.ChangePrices[pair][period].ChangePercent)
					}
					result := StrategyBaseResult{
						Executed: true,
						Data: BaseResult{
							Pair:          pair,
							Period:        period,
							ChangePercent: assets.ChangePrices[pair][period].ChangePercent,
						},
					}
					// Уведомляем подписчиков
					str.notifySubscribers(result)
				}
			}
		}
	}
}

func (str *StrategyBase) Subscribe(ch chan StrategyBaseResult) {
	str.subscribers = append(str.subscribers, ch)
}

func (str *StrategyBase) notifySubscribers(result StrategyBaseResult) {
	for _, sub := range str.subscribers {
		go func(sub chan StrategyBaseResult) {
			select {
			case sub <- result:
				// Успешно отправили
			case <-time.After(1 * time.Second):
				fmt.Println("Timeout sending result to subscriber, skipping...")
			}
		}(sub)
	}
}

func (str *StrategyBase) GetTelegramMenu() model.WindowHandler {
	return str.TelegramMenu
}

func (str *StrategyBase) OnMarket(ms exModel.MarketsStat) {

}
