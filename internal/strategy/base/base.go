package base

import (
	"context"
	"fmt"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/signal"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	"github.com/sambly/exchangebot/internal/toggle"
)

var baseLogger = logger.AddFields(map[string]interface{}{
	"package": "base",
})

type StrategyBase struct {
	Config       *Config
	Notification *notification.Notification
	TelegramMenu *StrategyBaseMenu

	// NotificationEnable переключается из телеграм-меню, а читается горутиной
	// стратегии - поэтому живёт отдельно от Config, за мьютексом.
	NotificationEnable *toggle.Bool

	Periods      map[string]time.Duration
	AssetsPrices *prices.AssetsPrices

	subscribers []chan signal.Signal
}

// Subscribe подписывает исполнителя на сигналы стратегии.
// Тип сигнала общий для всех детекторов - см. пакет signal.
func (str *StrategyBase) Subscribe(ch chan signal.Signal) {
	str.subscribers = append(str.subscribers, ch)
}

func (str *StrategyBase) signalFrom(pair, period string, changePercent float64) signal.Signal {
	direction := signal.DirectionUp
	if changePercent < 0 {
		direction = signal.DirectionDown
	}

	return signal.Signal{
		Source:        str.Config.IDName,
		Pair:          pair,
		Period:        period,
		Time:          time.Now(),
		Direction:     direction,
		Level:         1,
		Strength:      changePercent,
		ChangePercent: changePercent,
		// base не считает статистику по паре, поэтому волатильность неизвестна.
		// Политика выхода в этом случае откатится на проценты из конфига.
		Volatility: 0,
		Reason:     fmt.Sprintf("%s %s изменение %.2f%%", str.Config.IDName, period, changePercent),
	}
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
		AssetsPrices:       assetsPrices,
		Periods:            periods,
		Config:             cfg,
		Notification:       notify,
		NotificationEnable: toggle.New(cfg.NotificationEnable),
	}
	return str, nil
}

func (s *StrategyBase) WithTelegramMenu() *StrategyBase {
	tlgMenu := NewStrategyMenu(s.Config.Name, s.Config.IDName, s)
	s.TelegramMenu = tlgMenu
	return s
}

func (str *StrategyBase) Start(ctx context.Context) error {

	updates := str.AssetsPrices.Subscribe()

	for {
		select {
		case <-updates:
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

			// Читаем под локом AssetsPrices: раньше здесь была гонка с
			// горутиной, которая пересчитывает цены раз в минуту.
			cp, ok := assets.GetChangePrices(pair, period)
			if !ok {
				continue
			}

			if cp.ChangePercent >= str.Config.WeightProcents[period] {
				// Отправка сообщения об изменении цены
				if str.NotificationEnable.Get() {
					str.NotificationWeightPercent(pair, period, cp.ChangePercent)
				}
				// Уведомляем подписчиков-исполнителей
				str.publish(str.signalFrom(pair, period, cp.ChangePercent))
			}
		}
	}
}

// publish рассылает сигнал НЕблокирующе. Раньше на каждого подписчика
// поднималась горутина с таймаутом в секунду - при залипшем потребителе это
// плодило горутины пачками.
func (str *StrategyBase) publish(sig signal.Signal) {
	for _, sub := range str.subscribers {
		select {
		case sub <- sig:
		default:
			baseLogger.Warnf("подписчик не успевает, сигнал %s %s отброшен", sig.Pair, sig.Period)
		}
	}
}

func (str *StrategyBase) GetTelegramMenu() model.WindowHandler {
	return str.TelegramMenu
}

func (str *StrategyBase) OnMarket(ms exModel.MarketsStat) {

}
