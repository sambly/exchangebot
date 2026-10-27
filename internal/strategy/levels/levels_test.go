package levels

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

// --- confirmStreak / levelFromScore: те же тесты, что у potential, тот же
// принцип (см. Go-комментарий у confirmStreak) ---

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

func TestConfirmStreakResetsOnSideFlip(t *testing.T) {
	state := &pairPeriodState{}

	confirmStreak(state, "BUY", 2, 3)
	confirmStreak(state, "BUY", 2, 3)
	confirmStreak(state, "SELL", 2, 3)

	if _, ok := confirmStreak(state, "SELL", 2, 3); ok {
		t.Fatal("после флипа + 2 тиков (из 3 требуемых) стрик не должен подтверждаться")
	}
	if _, ok := confirmStreak(state, "SELL", 2, 3); !ok {
		t.Fatal("после флипа + 3 тиков подряд стрик должен подтвердиться")
	}
}

func TestConfirmStreakTracksPeakLevel(t *testing.T) {
	state := &pairPeriodState{}

	confirmStreak(state, "BUY", 1, 3)
	confirmStreak(state, "BUY", 3, 3)
	level, ok := confirmStreak(state, "BUY", 2, 3)

	if !ok || level != 3 {
		t.Fatalf("ожидался пиковый level=3, получено level=%d ok=%v", level, ok)
	}
}

func TestConfirmStreakSingleCheckConfirmsImmediately(t *testing.T) {
	state := &pairPeriodState{}

	level, ok := confirmStreak(state, "BUY", 2, 1)
	if !ok || level != 2 {
		t.Fatalf("при checks=1 первый тик должен подтверждаться немедленно, level=%d ok=%v", level, ok)
	}
}

func TestLevelFromScoreThresholds(t *testing.T) {
	thresholds := ThresholdsConfig{Level1: 1.5, Level2: 2.5, Level3: 4.0}

	cases := []struct {
		score     float64
		wantLevel int
	}{
		{1.0, 0},
		{1.5, 1},
		{2.0, 1},
		{2.5, 2},
		{3.9, 2},
		{4.0, 3},
		{10.0, 3},
	}

	for _, tc := range cases {
		if got := levelFromScore(tc.score, thresholds); got != tc.wantLevel {
			t.Errorf("levelFromScore(%.2f) = %d, ожидалось %d", tc.score, got, tc.wantLevel)
		}
	}
}

// --- checkAndPublish: интеграционный тест на реальном AssetsSetup, тот же
// приём, что у potential_test.go/entrysetup/levels_test.go - fakeCandleRepo
// строит настоящую свечную историю, без похода в БД. ---

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

// flatCandlesWithLevels - плоская история с одним swing low (support) и
// одним swing high (resistance) около текущей цены - тот же приём, что в
// entrysetup/levels_test.go.
func flatCandlesWithLevels(pair string, from time.Time, step time.Duration, n int, base, supportPrice, resistancePrice float64) []exModel.Candle {
	out := make([]exModel.Candle, 0, n)
	for i := 0; i < n; i++ {
		c := exModel.Candle{Pair: pair, Time: from.Add(step * time.Duration(i)), Close: base, High: base, Low: base}
		if i == n/2-10 {
			c.Low = supportPrice
		}
		if i == n/2+10 {
			c.High = resistancePrice
		}
		out = append(out, c)
	}
	out[n-1].Close = base
	return out
}

func newLevelsTestStrategy(t *testing.T, pair string, candles []exModel.Candle, mktStat exModel.MarketsStat) *LevelsStrategy {
	t.Helper()

	periods := map[string]time.Duration{"15m": 15 * time.Minute}
	ap, err := prices.NewAssetsPrices([]string{pair}, periods, periods, fakeCandleRepo{candles: candles})
	if err != nil {
		t.Fatalf("NewAssetsPrices: %v", err)
	}
	ap.OnMarket(mktStat)

	setup := entrysetup.NewAssetsSetup(ap, nil)

	cfg := &Config{
		IDName:          "levels",
		Pairs:           []string{pair},
		MinDailyVolume:  0,
		Periods:         []string{"15m"},
		Thresholds:      ThresholdsConfig{Level1: 1.0, Level2: 2.0, Level3: 4.0},
		ConfirmChecks:   2,
		CooldownMinutes: 15,
	}

	return &LevelsStrategy{
		Config:         cfg,
		StrategyEnable: toggle.New(true),
		AssetsPrices:   ap,
		AssetsSetup:    setup,
		states:         map[string]map[string]*pairPeriodState{pair: {"15m": {}}},
	}
}

func collectSignal(ch chan signal.Signal) *signal.Signal {
	select {
	case sig := <-ch:
		return &sig
	default:
		return nil
	}
}

// support 8% вниз, resistance 20% вверх -> score = 20/8 = 2.5 -> level 2
// (thresholds level1=1.0, level2=2.0, level3=4.0), сторона BUY (support ближе).
func setupLevelsCandles(pair string, base float64) []exModel.Candle {
	from := time.Now().Add(-101 * 15 * time.Minute)
	return flatCandlesWithLevels(pair, from, 15*time.Minute, 101, base, base*0.92, base*1.20)
}

func TestCheckAndPublishRequiresConfirmStreak(t *testing.T) {
	pair := "BTCUSDT"
	str := newLevelsTestStrategy(t, pair, setupLevelsCandles(pair, 100), exModel.MarketsStat{Volume: 10_000_000})
	ch := make(chan signal.Signal, 4)
	str.Subscribe(ch)

	str.checkAndPublish()

	if sig := collectSignal(ch); sig != nil {
		t.Fatalf("сигнал не должен публиковаться после первого тика (ConfirmChecks=2), получено %+v", sig)
	}
}

func TestCheckAndPublishPublishesAfterConfirmStreak(t *testing.T) {
	pair := "BTCUSDT"
	str := newLevelsTestStrategy(t, pair, setupLevelsCandles(pair, 100), exModel.MarketsStat{Volume: 10_000_000})
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
	if sig.Source != "levels" {
		t.Fatalf("Source = %q, ожидалось levels", sig.Source)
	}
	if sig.Direction != signal.DirectionUp {
		t.Fatalf("Direction = %s, ожидался UP (support ближе, чем resistance)", sig.Direction)
	}
	if sig.Level != 2 {
		t.Fatalf("Level = %d, ожидался 2 (score=2.5 между level2=2.0 и level3=4.0)", sig.Level)
	}
}

func TestCheckAndPublishRespectsCooldown(t *testing.T) {
	pair := "BTCUSDT"
	str := newLevelsTestStrategy(t, pair, setupLevelsCandles(pair, 100), exModel.MarketsStat{Volume: 10_000_000})
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

func TestCheckAndPublishSkipsIlliquidPair(t *testing.T) {
	pair := "BTCUSDT"
	str := newLevelsTestStrategy(t, pair, setupLevelsCandles(pair, 100), exModel.MarketsStat{Volume: 1000})
	str.Config.MinDailyVolume = 5_000_000
	ch := make(chan signal.Signal, 4)
	str.Subscribe(ch)

	str.checkAndPublish()
	str.checkAndPublish()

	if sig := collectSignal(ch); sig != nil {
		t.Fatalf("неликвидная пара не должна публиковать сигналы, получено %+v", sig)
	}
}

// Слишком слабое соотношение (обе стороны рядом, score ниже level1) не
// должно давать сигнал вовсе.
func TestCheckAndPublishSkipsWeakScore(t *testing.T) {
	pair := "BTCUSDT"
	base := 100.0
	from := time.Now().Add(-101 * 15 * time.Minute)
	// support 5% вниз, resistance 6% вверх -> score = 6/5 = 1.2 - слабое
	// соотношение. Score>=1 всегда по построению (min/max), поэтому "ниже
	// порога" проверяется поднятием level1 заведомо выше любого реального
	// score, а не подбором совсем узкого диапазона.
	candles := flatCandlesWithLevels(pair, from, 15*time.Minute, 101, base, base*0.95, base*1.06)

	str := newLevelsTestStrategy(t, pair, candles, exModel.MarketsStat{Volume: 10_000_000})
	str.Config.Thresholds.Level1 = 100.0 // заведомо недостижимо
	ch := make(chan signal.Signal, 4)
	str.Subscribe(ch)

	str.checkAndPublish()
	str.checkAndPublish()

	if sig := collectSignal(ch); sig != nil {
		t.Fatalf("слабое соотношение (ниже level1) не должно публиковать сигнал, получено %+v", sig)
	}
}
