package entrysetup

import (
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
)

// trendCandles строит свечи одной пары с шагом step, Close - из closes по
// порядку (High/Low не важны для trendStrengthFromCandles - она смотрит
// только на Close).
func trendCandles(pair string, from time.Time, step time.Duration, closes []float64) []exModel.Candle {
	out := make([]exModel.Candle, 0, len(closes))
	for i, c := range closes {
		out = append(out, exModel.Candle{Pair: pair, Time: from.Add(step * time.Duration(i)), Close: c})
	}
	return out
}

// Прямая линия - весь путь пройден в одном направлении без единого отката:
// efficiency ratio должен быть ровно 1 (net == sum, каждое слагаемое sum
// того же знака, что и net).
func TestTrendStrengthFromCandlesStraightLineIsMaxTrend(t *testing.T) {
	candles := trendCandles("BTCUSDT", time.Now(), time.Minute, []float64{100, 101, 102, 103, 104, 105})

	ratio, ok := trendStrengthFromCandles(candles)
	if !ok {
		t.Fatal("ожидался готовый результат")
	}
	if ratio != 1.0 {
		t.Fatalf("на безоткатной прямой ожидался ratio=1.0, получено %v", ratio)
	}
}

// Зигзаг, вернувшийся туда же, откуда начал: net == 0, ratio == 0 - чистый
// шум/боковик, несмотря на большой пройденный путь.
func TestTrendStrengthFromCandlesZigzagIsZeroTrend(t *testing.T) {
	candles := trendCandles("BTCUSDT", time.Now(), time.Minute, []float64{100, 105, 100, 105, 100})

	ratio, ok := trendStrengthFromCandles(candles)
	if !ok {
		t.Fatal("ожидался готовый результат")
	}
	if ratio != 0.0 {
		t.Fatalf("на зигзаге, вернувшемся к старту, ожидался ratio=0.0, получено %v", ratio)
	}
}

// Зигзаг с чистым сдвигом должен давать промежуточное значение - не 0 и не 1.
func TestTrendStrengthFromCandlesPartialTrendIsBetweenBounds(t *testing.T) {
	// net = |106-100| = 6, sum = 5+3+2+4+2 = 16, ratio = 6/16 = 0.375
	candles := trendCandles("BTCUSDT", time.Now(), time.Minute, []float64{100, 105, 102, 104, 108, 106})

	ratio, ok := trendStrengthFromCandles(candles)
	if !ok {
		t.Fatal("ожидался готовый результат")
	}
	if ratio <= 0 || ratio >= 1 {
		t.Fatalf("ожидалось промежуточное значение (0;1), получено %v", ratio)
	}
	const want = 6.0 / 16.0
	if diff := ratio - want; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("ratio = %v, ожидалось %v", ratio, want)
	}
}

func TestTrendStrengthFromCandlesNotEnoughCandles(t *testing.T) {
	candles := trendCandles("BTCUSDT", time.Now(), time.Minute, []float64{100})

	if _, ok := trendStrengthFromCandles(candles); ok {
		t.Fatal("на одной свече результата быть не должно")
	}
}

// Ни одного движения (все Close одинаковы) - sum=0, честный ok=false вместо
// деления на ноль.
func TestTrendStrengthFromCandlesNoMovement(t *testing.T) {
	candles := trendCandles("BTCUSDT", time.Now(), time.Minute, []float64{100, 100, 100, 100})

	if _, ok := trendStrengthFromCandles(candles); ok {
		t.Fatal("без единого движения результата быть не должно")
	}
}

// Функция не должна зависеть от порядка свечей на входе - сортирует сама.
func TestTrendStrengthFromCandlesSortsByTime(t *testing.T) {
	now := time.Now()
	ordered := trendCandles("BTCUSDT", now, time.Minute, []float64{100, 101, 102, 103})
	shuffled := []exModel.Candle{ordered[2], ordered[0], ordered[3], ordered[1]}

	want, ok := trendStrengthFromCandles(ordered)
	if !ok {
		t.Fatal("ожидался готовый результат (ordered)")
	}
	got, ok := trendStrengthFromCandles(shuffled)
	if !ok {
		t.Fatal("ожидался готовый результат (shuffled)")
	}
	if got != want {
		t.Fatalf("результат зависит от порядка на входе: ordered=%v shuffled=%v", want, got)
	}
}

// GetTrendStrength читает через тот же GetPeriodCandles/filterPair путь, что
// GetPriceLevels (см. newLevelsSetup/fakeCandleRepo в levels_test.go) - здесь
// проверяется в основном сама проводка (period/prices==nil), не математика.
func TestGetTrendStrengthNilPrices(t *testing.T) {
	s := NewAssetsSetup(nil, nil)
	if _, ok := s.GetTrendStrength("BTCUSDT", "15m"); ok {
		t.Fatal("без AssetsPrices результата быть не должно")
	}
}

func TestGetTrendStrengthUnknownPeriod(t *testing.T) {
	s := newLevelsSetup(t, "BTCUSDT", trendCandles("BTCUSDT", time.Now(), 15*time.Minute, []float64{100, 101, 102}))

	if _, ok := s.GetTrendStrength("BTCUSDT", "1h"); ok {
		t.Fatal("на незнакомом периоде результата быть не должно")
	}
}

func TestGetTrendStrengthReadsCandlesForPair(t *testing.T) {
	from := time.Now().Add(-time.Hour)
	candles := trendCandles("BTCUSDT", from, 15*time.Minute, []float64{100, 101, 102, 103, 104})
	s := newLevelsSetup(t, "BTCUSDT", candles)

	ratio, ok := s.GetTrendStrength("BTCUSDT", "15m")
	if !ok {
		t.Fatal("ожидался готовый результат")
	}
	if ratio != 1.0 {
		t.Fatalf("на безоткатной прямой ожидался ratio=1.0, получено %v", ratio)
	}
}
