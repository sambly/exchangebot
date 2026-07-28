// Package structsale - политика выхода, начатая как эксперимент над
// simplesale: та же база (тейк/стоп от волатильности пары, таймаут), плюс
// трейлинг-стоп, масштабирование тейка по силе сигнала и структурный
// стоп/тейк от стакана (entrysetup.GetQuality). simplesale сознательно не
// меняется - оба пакета реализуют один и тот же интерфейс sales.Sales, и
// Executor.WithSaleStrategy принимает любой из них.
//
// Структурная часть (стены стакана) принципиально НЕ бэктестируется: у стены
// нет исторических данных вообще - стакан не пишется в БД. В backtest.go
// exits конструируется с entrySetup=nil, и Plan тогда просто пропускает этот
// шаг, откатываясь на волатильность - как и раньше. Свечные уровни
// (entrysetup.PriceLevels) сюда всё ещё не включены: их прикрутить можно было
// бы и для бэктеста, но это потребовало бы менять сигнатуру Sales.Plan
// (передавать туда историю свечей), а это уже задело бы Executor,
// backtest.Engine и сам simplesale - отдельный, более крупный шаг.
package structsale

import (
	"math"
	"sync"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/entrysetup"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/strategy/sales"
	"github.com/sambly/exchangebot/internal/strategy/signal"
)

var saleLogger = logger.AddFields(map[string]interface{}{
	"package": "structsale",
})

type StrategyStructSale struct {
	Config          *Config
	OrderController *order.OrderService

	// entrySetup - источник структурного стопа/тейка (см. structuralPlan).
	// nil в бэктесте (там его физически не из чего построить - нет истории
	// стакана) и в тестах, которые его не проверяют - Plan в этом случае
	// просто пропускает структурный шаг, откатываясь на волатильность.
	entrySetup *entrysetup.AssetsSetup

	// extreme - лучшая (для BUY - максимальная, для SELL - минимальная) цена,
	// увиденная с момента входа, по ID ордера. Не в sales.Position: Position
	// передаётся ПО ЗНАЧЕНИЮ на каждый вызов (см. Executor.checkPositions -
	// он копирует срез перед проходом), мутировать в ней нечего - переживает
	// повторные вызовы только то, что лежит здесь, в самой стратегии.
	mu      sync.Mutex
	extreme map[int64]float64

	// deadlineExtension - продлённый (см. Config.AdaptiveTimeoutMinProfitPercent)
	// дедлайн по ID ордера, если продление уже выдавалось. Тот же принцип, что
	// у extreme: state здесь, а не в sales.Position, потому что Position
	// передаётся по значению и мутировать её нечем.
	deadlineExtension map[int64]time.Time
}

// NewStrategy - entrySetup может быть nil (бэктест, тесты без структурной
// части): тогда Plan просто никогда не берёт структурный стоп/тейк.
func NewStrategy(orderController *order.OrderService, entrySetup *entrysetup.AssetsSetup) (*StrategyStructSale, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	return &StrategyStructSale{Config: cfg, OrderController: orderController, entrySetup: entrySetup}, nil
}

func (str *StrategyStructSale) Name() string {
	return str.Config.IDName
}

// Plan - см. simplesale.Plan, та же база (волатильность пары, либо откат на
// проценты), плюс масштабирование ТЕЙКА (не стопа) по силе сигнала: чем выше
// Level аномалии, тем дальше цель. Стоп не растёт вместе с тейком - у более
// сильного сигнала есть основания ждать более широкого движения, но это не
// повод рисковать шире на входе.
//
// Дальше, если структурные данные сейчас доступны и достаточно привлекательны
// (см. structuralPlan), они ПОЛНОСТЬЮ ЗАМЕНЯЮТ волатильностный план: стены
// стакана прямо сейчас - более конкретный ориентир, чем статистическая оценка
// типичного движения. Масштабирование по силе сигнала на структурный план не
// распространяется - дистанция до дальней стены такая, какая есть, раздувать
// её произвольным множителем не за что.
func (str *StrategyStructSale) Plan(sig signal.Signal) (takeProfit, stopLoss float64, hold time.Duration) {
	cfg := str.Config

	if sig.Volatility > 0 {
		takeProfit = sig.Volatility * cfg.TakeProfitVol
		stopLoss = sig.Volatility * cfg.StopLossVol
	} else {
		takeProfit = cfg.TakeProfitPercent
		stopLoss = cfg.StopLossPercent
	}

	if cfg.StrengthScalePerLevel > 0 && sig.Level > 1 {
		takeProfit *= 1 + float64(sig.Level-1)*cfg.StrengthScalePerLevel
	}

	if structTake, structStop, ok := str.structuralPlan(sig); ok {
		takeProfit, stopLoss = structTake, structStop
	}

	takeProfit = clamp(takeProfit, cfg.MinTakeProfitPercent, cfg.MaxTakeProfitPercent)
	stopLoss = clamp(stopLoss, cfg.MinStopLossPercent, cfg.MaxStopLossPercent)

	return takeProfit, stopLoss, time.Duration(cfg.MaxHoldPeriods) * periodDuration(sig.Period)
}

// structuralPlan - тейк/стоп от структуры стакана на момент сигнала (см.
// entrysetup.GetQuality): стоп = дистанция до БЛИЖНЕЙ стены, тейк = дистанция
// до ДАЛЬНЕЙ - вне зависимости от того, какая именно стена (Support или
// Resistance) ближе. Quality.Side тут намеренно не используется: Plan
// симметричен по стороне сделки (сторону решает Executor, не эта политика), и
// "стоп/тейк" здесь - обобщённые дистанции "ближе/дальше", а не "сверху/снизу".
//
// ok=false - структурных данных сейчас нет (entrySetup=nil, паре не хватает
// подписки на стакан, или сейчас не обе стены найдены) либо соотношение
// дальней стены к ближней меньше MinStructuralScore: слабая структурная
// картина хуже статистической оценки волатильности, а не лучше.
func (str *StrategyStructSale) structuralPlan(sig signal.Signal) (takeProfit, stopLoss float64, ok bool) {
	if !str.Config.UseStructuralLevels || str.entrySetup == nil {
		return 0, 0, false
	}

	quality, found := str.entrySetup.GetQuality(sig.Pair, sig.Period)
	if !found || quality.Score < str.Config.MinStructuralScore {
		return 0, 0, false
	}

	return quality.TakeDistancePercent, quality.StopDistancePercent, true
}

// Execute - см. simplesale.Execute: то же самое, решение и закрытие разнесены
// через ShouldExit.
func (str *StrategyStructSale) Execute(ms exModel.MarketsStat, position sales.Position) bool {
	reason, exit := str.ShouldExit(ms.Price, time.Now(), position)
	if !exit {
		return false
	}

	isSell := position.Order.Side == order.SideTypeSell
	profit := profitPercentAt(ms.Price, position.Order.PriceCreated, isSell)

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

// ShouldExit - чистое решение, как у simplesale, плюс трейлинг-стоп и
// адаптивный таймаут.
//
// Пока прибыль (от лучшей достигнутой цены с момента входа, а не от текущей)
// не достигла TrailingActivationPercent, позиция ведёт себя ТОЧНО как в
// simplesale: фиксированный тейк, фиксированный стоп, таймаут. После -
// фиксированный тейк перестаёт действовать (даём прибыли расти дальше вместо
// того, чтобы срезать её на первой цели), а эффективным стопом становится
// откат от максимума на TrailingCallbackPercent. Жёсткий стоп от входа
// продолжает действовать в фоне на случай большого разрыва цены.
func (str *StrategyStructSale) ShouldExit(price float64, at time.Time, position sales.Position) (sales.ExitReason, bool) {
	entry := position.Order.PriceCreated
	if entry == 0 || price == 0 {
		return "", false
	}
	isSell := position.Order.Side == order.SideTypeSell

	extreme := str.updateExtreme(position.Order.ID, entry, price, isSell)
	profit := profitPercentAt(price, entry, isSell)
	extremeProfit := profitPercentAt(extreme, entry, isSell)

	// Жёсткий стоп от входа - абсолютный пол риска, проверяется первым и
	// безусловно, даже если трейлинг формально уже активен: движение,
	// ушедшее глубже изначально запланированного риска (например, одним
	// большим гэпом мимо трейлинг-порога), - это стоп-лосс, а не "трейлинг
	// не удержал прибыль".
	if profit <= -position.StopLossPercent {
		str.clearPositionState(position.Order.ID)
		return sales.ExitStopLoss, true
	}

	trailingActive := str.Config.TrailingActivationPercent > 0 && extremeProfit >= str.Config.TrailingActivationPercent

	switch {
	case trailingActive:
		giveBack := extremeProfit - profit
		if giveBack >= str.Config.TrailingCallbackPercent {
			str.clearPositionState(position.Order.ID)
			return sales.ExitTrailingStop, true
		}
	case profit >= position.TakeProfitPercent:
		str.clearPositionState(position.Order.ID)
		return sales.ExitTakeProfit, true
	}

	deadline := str.effectiveDeadline(position)
	if !deadline.IsZero() && at.After(deadline) {
		// Адаптивный таймаут: сделка уже в достаточном плюсе - не обрываем
		// работающую позицию только потому, что истекло время, продлеваем
		// дедлайн (один раз, см. tryExtendDeadline) вместо закрытия.
		if !str.tryExtendDeadline(position, profit) {
			str.clearPositionState(position.Order.ID)
			return sales.ExitTimeout, true
		}
	}

	return "", false
}

// updateExtreme запоминает и возвращает лучшую цену с момента входа для этой
// позиции. Вызывается на каждую проверку - функция монотонна (только
// расширяет экстремум, никогда не сужает), поэтому её можно звать сколько
// угодно раз с любыми ценами одного и того же бара в любом порядке (важно для
// бэктеста: tryExit проверяет один бар до трёх раз подряд - худшая цена,
// лучшая, закрытие - и порядок этих вызовов не хронологический).
func (str *StrategyStructSale) updateExtreme(orderID int64, entry, price float64, isSell bool) float64 {
	str.mu.Lock()
	defer str.mu.Unlock()

	if str.extreme == nil {
		str.extreme = make(map[int64]float64)
	}

	current, ok := str.extreme[orderID]
	if !ok {
		current = entry
	}

	if isSell {
		current = math.Min(current, price)
	} else {
		current = math.Max(current, price)
	}

	str.extreme[orderID] = current
	return current
}

// clearPositionState убирает всё внутреннее состояние закрытой позиции
// (экстремум трейлинга, продление дедлайна). Позиции, закрытые НЕ через эту
// стратегию (руками из веба/telegram, пока structsale её отслеживала),
// оставят запись висеть - это маленькая утечка на завершённую сделку, а не на
// каждый тик, и бот обычно не работает без перезапуска месяцами.
func (str *StrategyStructSale) clearPositionState(orderID int64) {
	str.mu.Lock()
	delete(str.extreme, orderID)
	delete(str.deadlineExtension, orderID)
	str.mu.Unlock()
}

// effectiveDeadline - реальный дедлайн позиции: обычно совпадает с
// position.Deadline, но может быть продлён ОДИН РАЗ (см. tryExtendDeadline).
func (str *StrategyStructSale) effectiveDeadline(position sales.Position) time.Time {
	str.mu.Lock()
	defer str.mu.Unlock()

	if extended, ok := str.deadlineExtension[position.Order.ID]; ok {
		return extended
	}
	return position.Deadline
}

// tryExtendDeadline - см. Config.AdaptiveTimeoutMinProfitPercent. Продлевает
// дедлайн РОВНО ОДИН РАЗ на позицию: если продление для этого ID уже
// выдавалось, возвращает false, даже если прибыль всё ещё выше порога -
// иначе позиция, зависшая в небольшом перманентном плюсе, никогда не
// закрылась бы по времени вовсе.
func (str *StrategyStructSale) tryExtendDeadline(position sales.Position, profit float64) bool {
	cfg := str.Config
	if cfg.AdaptiveTimeoutMinProfitPercent <= 0 || cfg.AdaptiveTimeoutExtensionPeriods <= 0 {
		return false
	}
	if profit < cfg.AdaptiveTimeoutMinProfitPercent {
		return false
	}

	id := position.Order.ID

	str.mu.Lock()
	defer str.mu.Unlock()

	if str.deadlineExtension == nil {
		str.deadlineExtension = make(map[int64]time.Time)
	}
	if _, already := str.deadlineExtension[id]; already {
		return false
	}

	extension := time.Duration(cfg.AdaptiveTimeoutExtensionPeriods) * periodDuration(position.Signal.Period)
	str.deadlineExtension[id] = position.Deadline.Add(extension)
	return true
}

// PartialTakeProfit - см. sales.Sales.PartialTakeProfit. Уровень считается от
// position.TakeProfitPercent - конкретной цели уже посчитанного для этой
// позиции плана (в т.ч. с учётом масштабирования по силе сигнала или
// структурного плана, если они применились в Plan), а не заново от
// волатильности. РАБОТАЕТ ТОЛЬКО В БЭКТЕСТЕ (см. Config.PartialTakeProfit*) -
// Execute/ShouldExit в бою эту фичу не используют.
func (str *StrategyStructSale) PartialTakeProfit(position sales.Position) (distancePercent, fraction float64, ok bool) {
	cfg := str.Config
	if cfg.PartialTakeProfitPercent <= 0 || cfg.PartialTakeProfitFraction <= 0 || cfg.PartialTakeProfitFraction >= 1 {
		return 0, 0, false
	}
	return position.TakeProfitPercent * cfg.PartialTakeProfitPercent, cfg.PartialTakeProfitFraction, true
}

// profitPercentAt - прибыль в процентах от входа при заданной цене. У шорта
// знак обратный: падение цены - это прибыль. Принимает entry/isSell явно
// (не sales.Position), потому что считается и для текущей цены, и для
// запомненного экстремума - разными числами, но по одной формуле.
func profitPercentAt(price, entry float64, isSell bool) float64 {
	if entry == 0 {
		return 0
	}
	profit := (price/entry)*100 - 100
	if isSell {
		profit = -profit
	}
	return profit
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

// periodDuration - см. simplesale.periodDuration, та же таблица.
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
	case "12h":
		return 12 * time.Hour
	default:
		return time.Hour
	}
}
