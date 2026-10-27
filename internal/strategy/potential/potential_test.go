package potential

import (
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/entrysetup"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/signal"
	"github.com/sambly/exchangebot/internal/toggle"
)

// --- confirmStreak: тот же принцип, что у anomaly.confirm(), проверяется тем
// же способом (см. anomaly_test.go) ---

// Одного тика недостаточно, пока checks > 1 - иначе confirm-стрик ничем не
// отличается от его отсутствия.
func TestConfirmStreakRequiresConsecutiveSameSide(t *testing.T) {
	state := &pairPeriodState{}

	if _, ok := confirmStreak(state, "BUY", 1, 3); ok {
		t.Fatal("после 1 тика из 3 требуемых стрик не должен считаться подтверждённым")
	}
	if _, ok := confirmStreak(state, "BUY", 1, 3); ok {
		t.Fatal("после 2 тиков из 3 требуемых стрик не должен считаться подтверждённым")
	}
	level, ok := confirmStreak(state, "BUY", 1, 3)
	if !ok || level != 1 {
		t.Fatalf("после 3 тиков подряд ожидался confirmed level=1, получено level=%d ok=%v", level, ok)
	}
}

// Смена стороны прерывает стрик - иначе "подтверждение" унаследовалось бы от
// уже неактуального направления (та же логика, что у anomaly.confirm()).
func TestConfirmStreakResetsOnSideFlip(t *testing.T) {
	state := &pairPeriodState{}

	confirmStreak(state, "BUY", 2, 3)
	confirmStreak(state, "BUY", 2, 3)
	confirmStreak(state, "SELL", 2, 3) // флип - счётчик должен начаться заново

	if _, ok := confirmStreak(state, "SELL", 2, 3); ok {
		t.Fatal("после флипа + 2 тиков (из 3 требуемых) стрик не должен подтверждаться")
	}
	if _, ok := confirmStreak(state, "SELL", 2, 3); !ok {
		t.Fatal("после флипа + 3 тиков подряд стрик должен подтвердиться")
	}
}

// Отдаётся ПИКОВЫЙ уровень за окно подтверждения, а не последний/усреднённый -
// иначе сильная минута, за которой пришла послабее, читалась бы как более
// слабый сигнал, чем он реально был (тот же принцип, что у anomaly.confirm()).
func TestConfirmStreakTracksPeakLevel(t *testing.T) {
	state := &pairPeriodState{}

	confirmStreak(state, "BUY", 1, 3)
	confirmStreak(state, "BUY", 3, 3) // пик
	level, ok := confirmStreak(state, "BUY", 2, 3)

	if !ok || level != 3 {
		t.Fatalf("ожидался пиковый level=3, получено level=%d ok=%v", level, ok)
	}
}

// checks<=1 означает "подтверждать с первого тика" - тот же смысл, что у
// anomaly.confirm() при ConfirmChecks=1.
func TestConfirmStreakSingleCheckConfirmsImmediately(t *testing.T) {
	state := &pairPeriodState{}

	level, ok := confirmStreak(state, "BUY", 2, 1)
	if !ok || level != 2 {
		t.Fatalf("при checks=1 первый тик должен подтверждаться немедленно, level=%d ok=%v", level, ok)
	}
}

// --- checkAndPublish: интеграционный тест на реальном AssetsSetup ---
//
// OFI/VPIN требуют накопления внутреннего состояния через приватные поля
// depth/prices пакетов (см. depth_test.go/vpin_test.go) - недоступно отсюда.
// Levels же считается заново на каждый вызов из истории свечей через репозиторий
// (см. entrysetup/levels_test.go, тот же приём) - единственный компонент,
// который реалистично довести до "готов" из внешнего пакета. MinTotal/MinAgree
// в тестах ниже занижены до 1, чтобы проверять именно ОРКЕСТРАЦИЮ
// (liquidPairs -> composite -> confirm -> cooldown -> publish), а не бизнес-
// правило "сколько компонентов должно быть готово" - оно уже покрыто
// composite_test.go.

type fakeCandleRepo struct {
	candles []exModel.Candle
}

func (r fakeCandleRepo) SelectMarketStateTimev2(time.Time) ([]exModel.Candle, error) { return nil, nil }
func (r fakeCandleRepo) SelectDeltaPeriod(string, string) ([]model.ChangeDeltaForCandle, error) {
	return nil, nil
}
func (r fakeCandleRepo) SelectCandlesFromPeriod(string, time.Time) ([]exModel.Candle, error) {
	return r.candles, nil
}

// flatCandlesWithLevels - тот же приём, что у flatCandles в
// internal/entrysetup/levels_test.go: плоская история с одним выраженным
// всплеском (resistance) и провалом (support) около текущей цены.
func flatCandlesWithLevels(pair string, from time.Time, step time.Duration, n int, base float64) []exModel.Candle {
	out := make([]exModel.Candle, 0, n)
	for i := 0; i < n; i++ {
		c := exModel.Candle{Pair: pair, Time: from.Add(step * time.Duration(i)), Close: base, High: base, Low: base}
		if i == n/2-10 {
			c.Low = base * 0.9 // support
		}
		if i == n/2+10 {
			c.High = base * 1.2 // resistance дальше support - LevelsSide=BUY
		}
		out = append(out, c)
	}
	out[n-1].Close = base
	return out
}

func newPotentialTestStrategy(t *testing.T, pair string, candles []exModel.Candle, mktStat exModel.MarketsStat) *PotentialStrategy {
	t.Helper()

	periods := map[string]time.Duration{"15m": 15 * time.Minute}
	ap, err := prices.NewAssetsPrices([]string{pair}, periods, periods, fakeCandleRepo{candles: candles})
	if err != nil {
		t.Fatalf("NewAssetsPrices: %v", err)
	}
	ap.OnMarket(mktStat)

	setup := entrysetup.NewAssetsSetup(ap, nil)

	cfg := &Config{
		Pairs:           []string{pair},
		MinDailyVolume:  0,
		Periods:         []string{"15m"},
		Components:      allComponentsEnabled,
		Thresholds:      ThresholdsConfig{Level1: 0.5, Level2: 1.0, Level3: 1.5},
		MinTotal:        1,
		MinAgree:        1,
		ConfirmChecks:   2,
		CooldownMinutes: 15,
	}

	str := &PotentialStrategy{
		Config:         cfg,
		StrategyEnable: toggle.New(true),
		AssetsPrices:   ap,
		AssetsSetup:    setup,
		states:         map[string]map[string]*pairPeriodState{pair: {"15m": {}}},
	}
	return str
}

func collectSignal(ch chan signal.Signal) *signal.Signal {
	select {
	case sig := <-ch:
		return &sig
	default:
		return nil
	}
}

// Первый тик только НАБИРАЕТ стрик (ConfirmChecks=2) - сигнал не публикуется
// раньше, чем сторона продержится нужное число тиков.
func TestCheckAndPublishRequiresConfirmStreak(t *testing.T) {
	pair := "BTCUSDT"
	from := time.Now().Add(-101 * 15 * time.Minute)
	candles := flatCandlesWithLevels(pair, from, 15*time.Minute, 101, 100)

	str := newPotentialTestStrategy(t, pair, candles, exModel.MarketsStat{Volume: 10_000_000})
	ch := make(chan signal.Signal, 4)
	str.Subscribe(ch)

	str.checkAndPublish()

	if sig := collectSignal(ch); sig != nil {
		t.Fatalf("сигнал не должен публиковаться после первого тика (ConfirmChecks=2), получено %+v", sig)
	}
}

// Второй тик подряд с той же стороной подтверждает стрик - сигнал должен
// уйти подписчику с корректными Pair/Period/Direction/Source.
func TestCheckAndPublishPublishesAfterConfirmStreak(t *testing.T) {
	pair := "BTCUSDT"
	from := time.Now().Add(-101 * 15 * time.Minute)
	candles := flatCandlesWithLevels(pair, from, 15*time.Minute, 101, 100)

	str := newPotentialTestStrategy(t, pair, candles, exModel.MarketsStat{Volume: 10_000_000})
	str.Config.IDName = "potential"
	ch := make(chan signal.Signal, 4)
	str.Subscribe(ch)

	str.checkAndPublish()
	str.checkAndPublish()

	sig := collectSignal(ch)
	if sig == nil {
		t.Fatal("после 2 тиков подряд (ConfirmChecks=2) сигнал должен быть опубликован")
	}
	if sig.Pair != pair || sig.Period != "15m" {
		t.Fatalf("Pair/Period = %s/%s, ожидалось %s/15m", sig.Pair, sig.Period, pair)
	}
	if sig.Source != "potential" {
		t.Fatalf("Source = %q, ожидалось potential", sig.Source)
	}
	// support дальше от цены, чем resistance в flatCandlesWithLevels - нет,
	// наоборот: support=0.9*base ближе (10%), resistance=1.2*base дальше
	// (20%) -> LevelsSide=BUY (см. levelsSideScore: ближний расстояние в
	// стоп, дальний - в тейк, сторона BUY когда стоп(support)<тейк(resistance)).
	if sig.Direction != signal.DirectionUp {
		t.Fatalf("Direction = %s, ожидался UP (Levels даёт BUY)", sig.Direction)
	}
}

// Cooldown должен подавить повторную публикацию того же уровня по той же
// паре+периоду - иначе подписчики (и лог) заливались бы одинаковыми
// сигналами на каждый тик, пока композит держится выше порога.
func TestCheckAndPublishRespectsCooldown(t *testing.T) {
	pair := "BTCUSDT"
	from := time.Now().Add(-101 * 15 * time.Minute)
	candles := flatCandlesWithLevels(pair, from, 15*time.Minute, 101, 100)

	str := newPotentialTestStrategy(t, pair, candles, exModel.MarketsStat{Volume: 10_000_000})
	ch := make(chan signal.Signal, 4)
	str.Subscribe(ch)

	str.checkAndPublish()
	str.checkAndPublish()
	if collectSignal(ch) == nil {
		t.Fatal("сигнал должен быть опубликован после 2 тиков подряд")
	}

	str.checkAndPublish()
	if sig := collectSignal(ch); sig != nil {
		t.Fatalf("повторная публикация того же уровня должна быть подавлена cooldown'ом, получено %+v", sig)
	}
}

// Неликвидная пара (оборот ниже MinDailyVolume) не должна проверяться вовсе -
// тот же фильтр, что у anomaly.liquidPairs.
func TestCheckAndPublishSkipsIlliquidPair(t *testing.T) {
	pair := "BTCUSDT"
	from := time.Now().Add(-101 * 15 * time.Minute)
	candles := flatCandlesWithLevels(pair, from, 15*time.Minute, 101, 100)

	str := newPotentialTestStrategy(t, pair, candles, exModel.MarketsStat{Volume: 1000})
	str.Config.MinDailyVolume = 5_000_000
	ch := make(chan signal.Signal, 4)
	str.Subscribe(ch)

	str.checkAndPublish()
	str.checkAndPublish()

	if sig := collectSignal(ch); sig != nil {
		t.Fatalf("неликвидная пара не должна публиковать сигналы, получено %+v", sig)
	}
}
