package backtest

import (
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/strategy/executor"
	"github.com/sambly/exchangebot/internal/strategy/sales"
	"github.com/sambly/exchangebot/internal/strategy/sales/simplesale"
	"github.com/sambly/exchangebot/internal/strategy/signal"
)

// stubDetector даёт сигнал, когда свеча выросла сильнее порога.
// Логика детекции в бэктестере не тестируется - у него её и нет.
type stubDetector struct {
	risePercent float64
	level       int
}

func (d stubDetector) DetectCandle(pair, period string, prev, current exModel.Candle) (signal.Signal, bool) {
	if prev.Close == 0 {
		return signal.Signal{}, false
	}
	change := current.Close/prev.Close*100 - 100
	if change < d.risePercent {
		return signal.Signal{}, false
	}
	return signal.Signal{
		Source:        "stub",
		Pair:          pair,
		Period:        period,
		Time:          current.Time,
		Direction:     signal.DirectionUp,
		Level:         d.level,
		ChangePercent: change,
		// Волатильность не задана: политика выхода откатится на проценты
		// из конфига - в тесте они и проверяются.
	}, true
}

func testEntry() *executor.Config {
	return &executor.Config{
		OnUp:                "buy",
		OnDown:              "skip",
		MinLevel:            2,
		MaxPositions:        5,
		PairCooldownMinutes: 60,
	}
}

func testExits() *simplesale.StrategySimpleSale {
	return &simplesale.StrategySimpleSale{
		Config: &simplesale.Config{
			IDName: "salesimple",
			// Vol-пороги подобраны так, что при волатильности 1.0 (volDetector)
			// план совпадает с процентным откатом: тейк 2%, стоп 1.5%.
			TakeProfitVol:        2.0,
			StopLossVol:          1.5,
			TakeProfitPercent:    2.0,
			StopLossPercent:      1.5,
			MinTakeProfitPercent: 0.5,
			MaxTakeProfitPercent: 20.0,
			MinStopLossPercent:   0.5,
			MaxStopLossPercent:   10.0,
			MaxHoldPeriods:       4,
		},
	}
}

func testEngine(fee float64) *Engine {
	return New(stubDetector{risePercent: 3.0, level: 2}, testEntry(), testExits(), Options{
		Period:         "15m",
		PeriodDuration: 15 * time.Minute,
		Periods:        map[string]time.Duration{"15m": 15 * time.Minute},
		FeePercent:     fee,
	})
}

// bar - свеча без теней: open=close=price, high/low задаются отдельно при
// необходимости.
func bar(pair string, t time.Time, price float64) exModel.Candle {
	return exModel.Candle{Pair: pair, Time: t, Open: price, Close: price, High: price, Low: price}
}

var t0 = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

func at(i int) time.Time { return t0.Add(time.Duration(i) * 15 * time.Minute) }

// Полный цикл: сигнал -> вход по close -> тейк на следующем баре по уровню
// плана, а не по максимуму свечи. Комиссия снимается за обе стороны.
func TestRunTakeProfitWithFees(t *testing.T) {
	candles := []exModel.Candle{
		bar("BTCUSDT", at(0), 100),
		bar("BTCUSDT", at(1), 100),
		bar("BTCUSDT", at(2), 104), // +4% - сигнал, вход по 104
		{Pair: "BTCUSDT", Time: at(3), Open: 104, Close: 105, High: 107, Low: 104},
	}

	report := testEngine(0.1).Run(candles)

	if len(report.Trades) != 1 {
		t.Fatalf("ожидалась 1 сделка, получено %d", len(report.Trades))
	}
	trade := report.Trades[0]

	if trade.Reason != "take-profit" {
		t.Fatalf("причина выхода %q, ожидался take-profit", trade.Reason)
	}
	// Тейк 2% от входа 104: gross ровно +2%, а не +2.88% (High 107).
	// Выходим по СВОЕМУ уровню, а не по лучшей цене свечи.
	if trade.GrossPct < 1.99 || trade.GrossPct > 2.01 {
		t.Fatalf("gross %+.3f%%, ожидалось +2%% (выход по уровню тейка)", trade.GrossPct)
	}
	// Net = gross - 2 * 0.1
	if diff := trade.GrossPct - trade.NetPct; diff < 0.199 || diff > 0.201 {
		t.Fatalf("комиссия съела %.3f%%, ожидалось 0.2%% (две стороны по 0.1%%)", diff)
	}
}

// Бар зацепил и стоп и тейк - засчитывается СТОП. Порядок цен внутри бара
// неизвестен, и бэктест обязан занижать результат, а не завышать.
func TestRunAmbiguousBarCountsAsStop(t *testing.T) {
	candles := []exModel.Candle{
		bar("BTCUSDT", at(0), 100),
		bar("BTCUSDT", at(1), 100),
		bar("BTCUSDT", at(2), 104), // вход по 104
		// High достаёт тейк (+2% = 106.08), Low достаёт стоп (-1.5% = 102.44)
		{Pair: "BTCUSDT", Time: at(3), Open: 104, Close: 104, High: 107, Low: 102},
	}

	report := testEngine(0).Run(candles)

	if len(report.Trades) != 1 {
		t.Fatalf("ожидалась 1 сделка, получено %d", len(report.Trades))
	}
	if report.Trades[0].Reason != "stop-loss" {
		t.Fatalf("двусмысленный бар должен закрываться стопом, получено %q", report.Trades[0].Reason)
	}
	if report.Trades[0].GrossPct > -1.49 {
		t.Fatalf("стоп -1.5%%, получено %+.3f%%", report.Trades[0].GrossPct)
	}
}

// Сигнальный бар не проверяется на выход: high/low этой свечи сложились ДО
// входа по её close, выходить по ним - заглядывание в прошлое.
func TestRunNoExitOnEntryBar(t *testing.T) {
	candles := []exModel.Candle{
		bar("BTCUSDT", at(0), 100),
		bar("BTCUSDT", at(1), 100),
		// Сигнальная свеча с глубокой тенью вниз: Low 95 пробил бы стоп
		{Pair: "BTCUSDT", Time: at(2), Open: 100, Close: 104, High: 104, Low: 95},
	}

	report := testEngine(0).Run(candles)

	if len(report.Trades) != 1 {
		t.Fatalf("ожидалась 1 сделка (end-of-data), получено %d", len(report.Trades))
	}
	if report.Trades[0].Reason != "end-of-data" {
		t.Fatalf("выход на сигнальном баре запрещён, ожидался end-of-data, получено %q", report.Trades[0].Reason)
	}
}

// Одна позиция на пару и cooldown после закрытия - те же риск-правила, что у
// боевого исполнителя.
//
// Тейк здесь широкий (10%), чтобы позиция дожила до следующего сигнала: с узким
// тейком выход всегда срабатывает раньше детекции на том же баре, и состояние
// "пара занята" не наступает.
func TestRunRiskRules(t *testing.T) {
	exits := testExits()
	exits.Config.TakeProfitPercent = 10.0
	exits.Config.StopLossPercent = 8.0
	exits.Config.MaxHoldPeriods = 0 // без таймаута, тест не про него

	engine := New(stubDetector{risePercent: 3.0, level: 2}, testEntry(), exits, Options{
		Period:         "15m",
		PeriodDuration: 15 * time.Minute,
		Periods:        map[string]time.Duration{"15m": 15 * time.Minute},
	})

	candles := []exModel.Candle{
		bar("BTCUSDT", at(0), 100),
		bar("BTCUSDT", at(1), 100),
		bar("BTCUSDT", at(2), 104), // сигнал, вход по 104 (тейк 10% = 114.4)
		bar("BTCUSDT", at(3), 108), // +3.8% - снова сигнал, но позиция открыта
		bar("BTCUSDT", at(4), 115), // тейк пробит - закрытие; сигнал +6.5% на этом же баре уже на cooldown
		bar("BTCUSDT", at(5), 119), // +3.5% - сигнал, но cooldown (60 минут) ещё идёт
	}

	report := engine.Run(candles)

	if len(report.Trades) != 1 {
		t.Fatalf("ожидалась 1 сделка, получено %d", len(report.Trades))
	}
	if report.Trades[0].Reason != "take-profit" {
		t.Fatalf("ожидался take-profit, получено %q", report.Trades[0].Reason)
	}
	if report.Rejected["по паре уже есть открытая позиция"] != 1 {
		t.Errorf("сигнал при открытой позиции должен отклоняться, rejected=%v", report.Rejected)
	}
	if report.Rejected["пара на cooldown после закрытия"] != 2 {
		t.Errorf("сигналы на cooldown должны отклоняться, rejected=%v", report.Rejected)
	}
}

// Таймаут: движения не случилось, позиция закрывается по close бара после
// дедлайна (maxHoldPeriods=4 по 15m = час).
func TestRunTimeout(t *testing.T) {
	candles := []exModel.Candle{
		bar("BTCUSDT", at(0), 100),
		bar("BTCUSDT", at(1), 100),
		bar("BTCUSDT", at(2), 104), // вход
		bar("BTCUSDT", at(3), 104.5),
		bar("BTCUSDT", at(4), 104.5),
		bar("BTCUSDT", at(5), 104.5),
		bar("BTCUSDT", at(6), 104.5),
		bar("BTCUSDT", at(7), 104.8), // дедлайн (at(2)+1h = at(6)) позади
	}

	report := testEngine(0).Run(candles)

	if len(report.Trades) != 1 {
		t.Fatalf("ожидалась 1 сделка, получено %d", len(report.Trades))
	}
	if report.Trades[0].Reason != "timeout" {
		t.Fatalf("ожидался timeout, получено %q", report.Trades[0].Reason)
	}
}

// Проскальзывание всегда против нас: вход дороже, выход дешевле.
func TestRunSlippage(t *testing.T) {
	engine := New(stubDetector{risePercent: 3.0, level: 2}, testEntry(), testExits(), Options{
		Period:          "15m",
		PeriodDuration:  15 * time.Minute,
		Periods:         map[string]time.Duration{"15m": 15 * time.Minute},
		SlippagePercent: 0.05,
	})

	candles := []exModel.Candle{
		bar("BTCUSDT", at(0), 100),
		bar("BTCUSDT", at(1), 100),
		bar("BTCUSDT", at(2), 104),
		{Pair: "BTCUSDT", Time: at(3), Open: 104, Close: 105, High: 108, Low: 104},
	}

	report := engine.Run(candles)

	if len(report.Trades) != 1 {
		t.Fatalf("ожидалась 1 сделка, получено %d", len(report.Trades))
	}
	// Без проскальзывания тейк дал бы ровно +2%; с ним - меньше
	if report.Trades[0].GrossPct >= 2.0 {
		t.Fatalf("проскальзывание должно резать результат, gross %+.3f%%", report.Trades[0].GrossPct)
	}
}

// stubDetector с волатильностью - лимитному входу нужен масштаб отступа
type volDetector struct {
	stubDetector
	volatility float64
}

func (d volDetector) DetectCandle(pair, period string, prev, current exModel.Candle) (signal.Signal, bool) {
	sig, ok := d.stubDetector.DetectCandle(pair, period, prev, current)
	sig.Volatility = d.volatility
	return sig, ok
}

// Лимитный вход: заявка на offset*vol ниже закрытия, исполняется только если
// откат случился, и по цене заявки - не по low бара.
func TestRunLimitEntryFillsOnPullback(t *testing.T) {
	engine := New(volDetector{stubDetector{risePercent: 3.0, level: 2}, 1.0}, testEntry(), testExits(), Options{
		Period:         "15m",
		PeriodDuration: 15 * time.Minute,
		Periods:        map[string]time.Duration{"15m": 15 * time.Minute},
		EntryOffsetVol: 1.0, // 1 волатильность = 1% ниже закрытия
		EntryTTLBars:   2,
	})

	candles := []exModel.Candle{
		bar("BTCUSDT", at(0), 100),
		bar("BTCUSDT", at(1), 100),
		bar("BTCUSDT", at(2), 104), // сигнал: заявка на 104*0.99 = 102.96
		// Откат: Low 102.5 дотянулся до 102.96 - исполнение по 102.96
		{Pair: "BTCUSDT", Time: at(3), Open: 104, Close: 103, High: 104, Low: 102.5},
		// Рост до тейка (+2% от 102.96 = 105.02)
		{Pair: "BTCUSDT", Time: at(4), Open: 103, Close: 105.5, High: 105.5, Low: 103},
	}

	report := engine.Run(candles)

	if report.LimitPlaced != 1 || report.LimitFilled != 1 {
		t.Fatalf("заявка должна исполниться: placed=%d filled=%d", report.LimitPlaced, report.LimitFilled)
	}
	if len(report.Trades) != 1 {
		t.Fatalf("ожидалась 1 сделка, получено %d", len(report.Trades))
	}
	trade := report.Trades[0]
	if trade.Reason != "take-profit" {
		t.Fatalf("ожидался take-profit, получено %q", trade.Reason)
	}
	// Вход по 102.96 вместо 104: тейк 2% от лучшей цены
	if trade.GrossPct < 1.99 || trade.GrossPct > 2.01 {
		t.Fatalf("gross %+.3f%%, ожидалось +2%% от цены заявки", trade.GrossPct)
	}
}

// Откат не случился - заявка снимается по TTL, сделки нет.
func TestRunLimitEntryExpiresWithoutPullback(t *testing.T) {
	engine := New(volDetector{stubDetector{risePercent: 3.0, level: 2}, 1.0}, testEntry(), testExits(), Options{
		Period:         "15m",
		PeriodDuration: 15 * time.Minute,
		Periods:        map[string]time.Duration{"15m": 15 * time.Minute},
		EntryOffsetVol: 1.0,
		EntryTTLBars:   2,
	})

	candles := []exModel.Candle{
		bar("BTCUSDT", at(0), 100),
		bar("BTCUSDT", at(1), 100),
		bar("BTCUSDT", at(2), 104), // заявка на 102.96
		// Цена уходит вверх без отката
		{Pair: "BTCUSDT", Time: at(3), Open: 104, Close: 106, High: 106, Low: 103.9},
		{Pair: "BTCUSDT", Time: at(4), Open: 106, Close: 108, High: 108, Low: 105.9},
		bar("BTCUSDT", at(5), 108),
	}

	report := engine.Run(candles)

	if report.LimitExpired != 1 {
		t.Fatalf("заявка должна сняться по TTL, expired=%d", report.LimitExpired)
	}
	if len(report.Trades) != 0 {
		t.Fatalf("без исполнения заявки сделок быть не должно, получено %d", len(report.Trades))
	}
}

// --- частичный тейк-профит ---

// stubExitsWithPartial - минимальная реализация sales.Sales с ОДНИМ уровнем
// частичного тейка. Не переиспользует structsale специально: тест должен
// проверять только взвешивание partial+final в самом бэктестере, не
// смешиваясь с трейлингом/структурными уровнями structsale.
type stubExitsWithPartial struct {
	takeProfit, stopLoss               float64
	partialAtFraction, partialFraction float64
}

func (s stubExitsWithPartial) Name() string { return "stub-partial" }

func (s stubExitsWithPartial) Plan(sig signal.Signal) (float64, float64, time.Duration) {
	return s.takeProfit, s.stopLoss, 0
}

func (s stubExitsWithPartial) ShouldExit(price float64, at time.Time, position sales.Position) (sales.ExitReason, bool) {
	entry := position.Order.PriceCreated
	profit := price/entry*100 - 100
	if profit >= position.TakeProfitPercent {
		return sales.ExitTakeProfit, true
	}
	if profit <= -position.StopLossPercent {
		return sales.ExitStopLoss, true
	}
	return "", false
}

func (s stubExitsWithPartial) Execute(ms exModel.MarketsStat, position sales.Position) bool {
	return false
}

func (s stubExitsWithPartial) PartialTakeProfit(position sales.Position) (float64, float64, bool) {
	if s.partialAtFraction <= 0 {
		return 0, 0, false
	}
	return position.TakeProfitPercent * s.partialAtFraction, s.partialFraction, true
}

// Частичное закрытие на полпути к тейку, потом полный тейк по остатку -
// должно свернуться в ОДНУ сделку со взвешенным результатом, а не в две.
func TestRunPartialTakeProfitWeightsSingleTrade(t *testing.T) {
	exits := stubExitsWithPartial{takeProfit: 10.0, stopLoss: 8.0, partialAtFraction: 0.5, partialFraction: 0.5}

	engine := New(stubDetector{risePercent: 3.0, level: 2}, testEntry(), exits, Options{
		Period:         "15m",
		PeriodDuration: 15 * time.Minute,
		Periods:        map[string]time.Duration{"15m": 15 * time.Minute},
	})

	candles := []exModel.Candle{
		bar("BTCUSDT", at(0), 100),
		bar("BTCUSDT", at(1), 100),
		bar("BTCUSDT", at(2), 104), // вход по 104; тейк 10% = 114.4, частичный уровень 5% = 109.2
		// Достаёт частичный уровень (109.2), но не полный тейк (114.4)
		{Pair: "BTCUSDT", Time: at(3), Open: 104, Close: 108, High: 110, Low: 104},
		// Достаёт полный тейк
		{Pair: "BTCUSDT", Time: at(4), Open: 108, Close: 114, High: 115, Low: 108},
	}

	report := engine.Run(candles)

	if report.PartialFills != 1 {
		t.Fatalf("ожидалось 1 частичное срабатывание, получено %d", report.PartialFills)
	}
	if len(report.Trades) != 1 {
		t.Fatalf("частичное+финальное закрытие должны свернуться в ОДНУ сделку, получено %d", len(report.Trades))
	}

	trade := report.Trades[0]
	if trade.Reason != "take-profit" {
		t.Fatalf("причина финального закрытия %q, ожидался take-profit", trade.Reason)
	}

	// Частичный кусок: 50% объёма по +5.0% (109.2/104-1). Финальный: 50% по
	// +10.0% (114.4/104-1, статичный уровень тейка). Взвешенно: 0.5*5+0.5*10=7.5%.
	want := 7.5
	if trade.GrossPct < want-0.05 || trade.GrossPct > want+0.05 {
		t.Fatalf("gross %+.3f%%, ожидалось ~%.1f%% (взвешенно: половина по 5%%, половина по 10%%)", trade.GrossPct, want)
	}
}

// Без настроенного частичного тейка (partialAtFraction=0) поведение должно
// быть ровно таким же, как без этой фичи вообще - никакого скрытого влияния
// на обычные сделки.
func TestRunNoPartialTakeProfitWhenDisabled(t *testing.T) {
	exits := stubExitsWithPartial{takeProfit: 10.0, stopLoss: 8.0} // partialAtFraction=0 - выключено

	engine := New(stubDetector{risePercent: 3.0, level: 2}, testEntry(), exits, Options{
		Period:         "15m",
		PeriodDuration: 15 * time.Minute,
		Periods:        map[string]time.Duration{"15m": 15 * time.Minute},
	})

	candles := []exModel.Candle{
		bar("BTCUSDT", at(0), 100),
		bar("BTCUSDT", at(1), 100),
		bar("BTCUSDT", at(2), 104),
		{Pair: "BTCUSDT", Time: at(3), Open: 104, Close: 108, High: 110, Low: 104},
		{Pair: "BTCUSDT", Time: at(4), Open: 108, Close: 114, High: 115, Low: 108},
	}

	report := engine.Run(candles)

	if report.PartialFills != 0 {
		t.Fatalf("частичный тейк выключен, ожидалось 0 срабатываний, получено %d", report.PartialFills)
	}
	if len(report.Trades) != 1 {
		t.Fatalf("ожидалась 1 сделка, получено %d", len(report.Trades))
	}
	// Без частичного тейка - обычный полный тейк по 10% от входа 104.
	want := 10.0
	if report.Trades[0].GrossPct < want-0.05 || report.Trades[0].GrossPct > want+0.05 {
		t.Fatalf("gross %+.3f%%, ожидалось ~%.1f%%", report.Trades[0].GrossPct, want)
	}
}

// Фильтр режима: если аномальна большая доля пар одновременно - это движение
// рынка, входы такта пропускаются.
func TestRunRegimeFilter(t *testing.T) {
	engine := New(stubDetector{risePercent: 3.0, level: 2}, testEntry(), testExits(), Options{
		Period:            "15m",
		PeriodDuration:    15 * time.Minute,
		Periods:           map[string]time.Duration{"15m": 15 * time.Minute},
		MaxAnomalousShare: 0.5, // больше половины пар аномальны - не входим
	})

	pairs := []string{"AAAUSDT", "BBBUSDT", "CCCUSDT"}
	candles := make([]exModel.Candle, 0)
	for _, p := range pairs {
		candles = append(candles,
			bar(p, at(0), 100),
			bar(p, at(1), 100),
			bar(p, at(2), 104), // все три пары аномальны разом: 100% > 50%
		)
	}

	report := engine.Run(candles)

	if len(report.Trades) != 0 {
		t.Fatalf("при рыночном движении входов быть не должно, получено %d сделок", len(report.Trades))
	}
	if report.Rejected["regime-filter"] != 3 {
		t.Fatalf("все 3 сигнала должны отклониться фильтром режима, rejected=%v", report.Rejected)
	}
}
