package simplesale

import (
	"math"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/strategy/sales"
	"github.com/sambly/exchangebot/internal/strategy/signal"
)

var saleLogger = logger.AddFields(map[string]interface{}{
	"package": "simplesale",
})

type StrategySimpleSale struct {
	Config          *Config
	OrderController *order.OrderService
}

func NewStrategy(orderController *order.OrderService) (*StrategySimpleSale, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	return &StrategySimpleSale{Config: cfg, OrderController: orderController}, nil
}

// Plan считает план выхода в единицах ВОЛАТИЛЬНОСТИ ПАРЫ, а не в абсолютных
// процентах.
//
// Фиксированные проценты одинаково неверны для BTC и для мемкоина: 1% для BTC -
// событие, для мемкоина - шум внутри минуты. Сигнал приносит с собой типичное
// движение пары за период (робастная сигма, та же, из которой считается z), и
// пороги ставятся кратно ей: тейк = takeProfitVol сигм, стоп = stopLossVol сигм.
//
// Если волатильность неизвестна (её не знает, например, стратегия base) -
// откатываемся на проценты из конфига.
func (str *StrategySimpleSale) Plan(sig signal.Signal) (takeProfit, stopLoss float64, hold time.Duration) {
	cfg := str.Config

	if sig.Volatility > 0 {
		takeProfit = sig.Volatility * cfg.TakeProfitVol
		stopLoss = sig.Volatility * cfg.StopLossVol
	} else {
		takeProfit = cfg.TakeProfitPercent
		stopLoss = cfg.StopLossPercent
	}

	// Границы: тейк ниже комиссий бессмыслен, стоп шире разумного - это уже
	// не стоп, а надежда.
	takeProfit = clamp(takeProfit, cfg.MinTakeProfitPercent, cfg.MaxTakeProfitPercent)
	stopLoss = clamp(stopLoss, cfg.MinStopLossPercent, cfg.MaxStopLossPercent)

	return takeProfit, stopLoss, time.Duration(cfg.MaxHoldPeriods) * periodDuration(sig.Period)
}

// Execute проверяет позицию на текущей цене: тейк, стоп, таймаут.
//
// Стоп-лосса раньше не было вообще - позиция в минусе висела вечно, потому что
// закрытие происходило только при достижении прибыли. Это и была главная
// проблема "продажи наугад".
func (str *StrategySimpleSale) Execute(ms exModel.MarketsStat, position sales.Position) bool {
	entry := position.Order.PriceCreated
	if entry == 0 || ms.Price == 0 {
		return false
	}

	profit := (ms.Price/entry)*100 - 100
	if position.Order.Side == order.SideTypeSell {
		profit = -profit
	}

	reason, exit := str.exitReason(profit, position)
	if !exit {
		return false
	}

	deal := order.Deal{
		Strategy:   str.Config.IDName,
		Comment:    string(reason),
		ExitReason: string(reason),
	}
	if err := str.OrderController.ClosePosition(position.Order.ID, deal); err != nil {
		saleLogger.Errorf("не удалось закрыть позицию id=%d (%s): %v", position.Order.ID, reason, err)
		return false
	}

	saleLogger.Infof("exit pair=%s id=%d reason=%s profit=%+.2f%% tp=%.2f%% sl=-%.2f%%",
		position.Order.Pair, position.Order.ID, reason, profit,
		position.TakeProfitPercent, position.StopLossPercent)

	return true
}

func (str *StrategySimpleSale) exitReason(profit float64, position sales.Position) (sales.ExitReason, bool) {
	switch {
	case profit >= position.TakeProfitPercent:
		return sales.ExitTakeProfit, true
	case profit <= -position.StopLossPercent:
		return sales.ExitStopLoss, true
	case !position.Deadline.IsZero() && time.Now().After(position.Deadline):
		// Сигнал протух: движения, которого мы ждали, не случилось.
		// Держать позицию дальше не за что.
		return sales.ExitTimeout, true
	}
	return "", false
}

func clamp(value, min, max float64) float64 {
	if min > 0 {
		value = math.Max(value, min)
	}
	if max > 0 {
		value = math.Min(value, max)
	}
	return value
}

// periodDuration переводит имя периода в длительность. Держим здесь, а не
// тянем карту периодов из главного конфига: политике выхода нужен только
// масштаб времени сигнала.
func periodDuration(period string) time.Duration {
	switch period {
	case "1m":
		return time.Minute
	case "3m":
		return 3 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "1d":
		return 12 * time.Hour // в конфиге приложения "1d" - это 12 часов
	default:
		return time.Hour
	}
}
