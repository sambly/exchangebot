package prices

import (
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
)

// seedCandles строит n свечей одной пары с шагом step, начиная с from, с ценой
// close, чередующейся между base и base*(1+swing) - даёт ненулевой, стабильный
// разброс для сидирования волатильности.
func seedCandles(pair string, from time.Time, step time.Duration, n int, base, swing float64) []exModel.Candle {
	out := make([]exModel.Candle, 0, n)
	price := base
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			price = base
		} else {
			price = base * (1 + swing)
		}
		out = append(out, exModel.Candle{Pair: pair, Time: from.Add(step * time.Duration(i)), Close: price})
	}
	return out
}

// Без сидирования и без набранной вручную истории GetVolatility должен честно
// отдавать ok=false, а не 0 как будто посчитано.
func TestGetVolatilityNotReadyWithoutHistory(t *testing.T) {
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute}, &stubRepo{})

	if _, ok := ap.GetVolatility("BTCUSDT", "15m"); ok {
		t.Fatal("GetVolatility без истории должен вернуть ok=false")
	}
}

// Сидирование из БД при старте (тот же приём, что у anomaly.seedHistory):
// достаточно свечей periода - и GetVolatility сразу готов, без ожидания
// накопления вживую.
func TestGetVolatilitySeedsFromDB(t *testing.T) {
	const period = "15m"
	from := time.Now().Add(-time.Duration(volatilityWindowSize+2) * 15 * time.Minute)

	repo := &stubRepo{
		periodCandles: seedCandles("BTCUSDT", from, 15*time.Minute, volatilityWindowSize+2, 100, 0.02),
	}

	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{period: 15 * time.Minute}, repo)

	vol, ok := ap.GetVolatility("BTCUSDT", period)
	if !ok {
		t.Fatal("после сидирования GetVolatility должен быть готов")
	}
	if vol <= 0 {
		t.Fatalf("волатильность должна быть положительной при колебании цены, получено %v", vol)
	}
}

// Разрыв в данных (пропущенные свечи) не должен попадать в историю как одна
// выборка - иначе разброс считается по фиктивному, слишком большому шагу.
func TestGetVolatilitySeedSkipsGaps(t *testing.T) {
	const period = "15m"
	from := time.Now().Add(-time.Duration(volatilityWindowSize+2) * 15 * time.Minute)

	candles := seedCandles("BTCUSDT", from, 15*time.Minute, volatilityWindowSize+2, 100, 0.02)
	// Разрыв в 10 периодов у одной свечи - выборка через эту дыру не должна учитываться
	candles[5].Time = candles[4].Time.Add(10 * 15 * time.Minute)

	repo := &stubRepo{periodCandles: candles}
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{period: 15 * time.Minute}, repo)

	// Не должно паниковать и не должно ошибочно валиться в ok=false из-за
	// одной дыры - остальных выборок достаточно для minSamples.
	if _, ok := ap.GetVolatility("BTCUSDT", period); !ok {
		t.Fatal("одна дыра в данных не должна лишать волатильность готовности при достаточном остатке выборок")
	}
}

// recordVolatility пишет в историю не чаще раза в period - соседние минутные
// значения ChangePercent почти идентичны (окно сдвигается на минуту из
// period), и частая запись занизила бы разброс (та же защита от
// автокорреляции, что у anomaly.PeriodState.nextSampleAt).
func TestRecordVolatilityThrottledPerPeriod(t *testing.T) {
	const period = "15m"
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{period: 15 * time.Minute}, &stubRepo{})

	state := ap.volatility["BTCUSDT"][period]
	before := state.record.Len()

	now := time.Now()
	ap.recordVolatility("BTCUSDT", period, 1.0, now)
	afterFirst := state.record.Len()

	ap.recordVolatility("BTCUSDT", period, 2.0, now.Add(time.Minute))
	afterSecond := state.record.Len()

	if afterFirst != before+1 {
		t.Fatalf("первая запись должна добавить одну выборку: было %d, стало %d", before, afterFirst)
	}
	if afterSecond != afterFirst {
		t.Fatalf("запись раньше nextSampleAt не должна добавляться: было %d, стало %d", afterFirst, afterSecond)
	}

	ap.recordVolatility("BTCUSDT", period, 3.0, now.Add(16*time.Minute))
	if got := state.record.Len(); got != afterFirst+1 {
		t.Fatalf("запись после истечения period должна добавиться: было %d, стало %d", afterFirst, got)
	}
}

// feedVolatility прогоняет values через recordVolatility с шагом ровно period
// (throttling каждый раз пропускает выборку) - имитирует накопление живой
// истории без ожидания реального времени.
func feedVolatility(ap *AssetsPrices, pair, period string, duration time.Duration, values []float64) {
	now := time.Now()
	for _, v := range values {
		ap.recordVolatility(pair, period, v, now)
		now = now.Add(duration)
	}
}

// Без истории режим не считается - как и GetVolatility, честно ok=false.
func TestGetVolatilityRegimeNotReadyWithoutHistory(t *testing.T) {
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute}, &stubRepo{})

	if _, ok := ap.GetVolatilityRegime("BTCUSDT", "15m"); ok {
		t.Fatal("GetVolatilityRegime без истории должен вернуть ok=false")
	}
}

// Долгая история из крупных качелей, а самые свежие выборки - почти плоские:
// это и есть сжатие, regime должен быть заметно меньше 1.
func TestGetVolatilityRegimeDetectsSqueeze(t *testing.T) {
	const period = "15m"
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{period: 15 * time.Minute}, &stubRepo{})

	values := make([]float64, 0, volatilityWindowSize)
	for i := 0; i < volatilityWindowSize-regimeShortWindow; i++ {
		if i%2 == 0 {
			values = append(values, 6)
		} else {
			values = append(values, -6)
		}
	}
	for i := 0; i < regimeShortWindow; i++ {
		if i%2 == 0 {
			values = append(values, 0.1)
		} else {
			values = append(values, -0.1)
		}
	}
	feedVolatility(ap, "BTCUSDT", period, 15*time.Minute, values)

	regime, ok := ap.GetVolatilityRegime("BTCUSDT", period)
	if !ok {
		t.Fatal("GetVolatilityRegime должен быть готов после достаточного числа выборок")
	}
	if regime >= 0.5 {
		t.Fatalf("ожидалось заметное сжатие (regime < 0.5), получено %v", regime)
	}
}

// Зеркальный случай: долгая история спокойная, самые свежие выборки - резкий
// разгон. Это расширение, regime должен быть заметно больше 1.
func TestGetVolatilityRegimeDetectsExpansion(t *testing.T) {
	const period = "15m"
	ap := newTestPrices(t, []string{"BTCUSDT"}, map[string]time.Duration{period: 15 * time.Minute}, &stubRepo{})

	values := make([]float64, 0, volatilityWindowSize)
	for i := 0; i < volatilityWindowSize-regimeShortWindow; i++ {
		if i%2 == 0 {
			values = append(values, 0.1)
		} else {
			values = append(values, -0.1)
		}
	}
	for i := 0; i < regimeShortWindow; i++ {
		if i%2 == 0 {
			values = append(values, 8)
		} else {
			values = append(values, -8)
		}
	}
	feedVolatility(ap, "BTCUSDT", period, 15*time.Minute, values)

	regime, ok := ap.GetVolatilityRegime("BTCUSDT", period)
	if !ok {
		t.Fatal("GetVolatilityRegime должен быть готов после достаточного числа выборок")
	}
	if regime <= 2 {
		t.Fatalf("ожидалось заметное расширение (regime > 2), получено %v", regime)
	}
}
