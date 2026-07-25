package stat

import (
	"math"
	"testing"
)

func filledRecord(t *testing.T, values []float64, minSamples int, scaleFloor float64) *MetricRecord {
	t.Helper()

	rec := NewMetricRecord(len(values)+10, minSamples, scaleFloor)
	for _, v := range values {
		rec.Add(v)
	}
	return rec
}

// Пока в буфере меньше minSamples значений, z-score не считается вообще.
// Без этого порога разброс оценивается по шуму: два случайно близких значения
// дают крошечный MAD, и следующее рядовое значение получает z в десятки.
func TestZScoreRequiresMinSamples(t *testing.T) {
	rec := NewMetricRecord(30, 20, 0)

	for i := 0; i < 19; i++ {
		rec.Add(float64(i%3) * 0.1)
		if z := rec.ZScore(100); z != 0 {
			t.Fatalf("при %d значениях z-score должен быть 0, получено %v", rec.Len(), z)
		}
	}

	rec.Add(0.1)
	if z := rec.ZScore(100); z == 0 {
		t.Fatal("при достижении minSamples z-score должен считаться")
	}
}

// Медиана и MAD устойчивы к выбросам: прошлый всплеск, осевший в буфере, не
// должен "ослеплять" следующую такую же оценку. На mean/stddev он бы раздул
// разброс, и вторая волна аномалии не прошла бы порог.
func TestZScoreRobustToOutlierInHistory(t *testing.T) {
	values := make([]float64, 0, 30)
	for i := 0; i < 30; i++ {
		values = append(values, 0.1*float64(i%3))
	}
	rec := filledRecord(t, values, 20, 0)

	before := rec.ZScore(10)
	rec.Add(10) // выброс попал в историю
	after := rec.ZScore(10)

	if math.Abs(before-after) > 0.01*math.Abs(before) {
		t.Fatalf("выброс в истории изменил z-score: было %.2f, стало %.2f", before, after)
	}
}

// Пол разброса гасит "плоские" ряды: если метрика почти не движется, MAD
// вырождается, и любое шевеление честно даёт z в единицы сигм - хотя по сути
// это шум.
func TestScaleFloorSuppressesFlatSeries(t *testing.T) {
	values := make([]float64, 0, 30)
	for i := 0; i < 30; i++ {
		values = append(values, 0.001*float64(i%3)) // дрейф в тысячные доли
	}

	noFloor := filledRecord(t, values, 20, 0)
	withFloor := filledRecord(t, values, 20, 0.35)

	if z := noFloor.ZScore(0.01); math.Abs(z) < 3 {
		t.Fatalf("без пола разброса шум должен давать большой z, получено %.2f", z)
	}
	if z := withFloor.ZScore(0.01); math.Abs(z) >= 3 {
		t.Fatalf("с полом разброса шум 0.01 не должен быть аномалией, получено z=%.2f", z)
	}
	if z := withFloor.ZScore(3.0); math.Abs(z) < 4 {
		t.Fatalf("настоящее движение 3.0 должно оставаться аномалией, получено z=%.2f", z)
	}
}

// MAD вырождается в ноль, если больше половины значений одинаковы.
// Тогда должен использоваться запасной разброс, а не деление на ноль.
func TestZScoreZeroMADFallback(t *testing.T) {
	values := make([]float64, 0, 30)
	for i := 0; i < 25; i++ {
		values = append(values, 0)
	}
	for i := 0; i < 5; i++ {
		values = append(values, 1)
	}
	rec := filledRecord(t, values, 20, 0)

	z := rec.ZScore(10)
	if math.IsNaN(z) || math.IsInf(z, 0) {
		t.Fatalf("z-score выродился: %v", z)
	}
	if z == 0 {
		t.Fatal("при вырожденном MAD должен работать запасной разброс, а не нулевой z")
	}
}
