// Package stat содержит общие робастные статистические примитивы. Сейчас
// это одна вещь - скользящее окно значений метрики и z-score по нему
// (медиана + MAD вместо среднего и stddev, устойчивее к тяжёлым хвостам
// распределений, типичным для крипты). Вынесено из internal/strategy/anomaly,
// чтобы им мог пользоваться и любой другой модуль, которому нужно понять
// "насколько текущее значение метрики необычно относительно своей же
// недавней истории" - например internal/depth для z-score имбаланса стакана -
// не завязываясь при этом на внутренности детектора аномалий.
package stat

import (
	"math"
	"sort"
	"sync"
)

// Константы перевода робастных оценок разброса в шкалу стандартного отклонения
// нормального распределения. Благодаря им пороги z остаются в привычной
// "сигма"-шкале, хотя разброс считается робастно.
const (
	// 1 / 0.6745: MAD нормального распределения = 0.6745 * sigma
	madToSigma = 1.4826
	// 1 / 0.7979: среднее абсолютное отклонение нормального = 0.7979 * sigma
	meanADToSigma = 1.2533
)

// MetricRecord хранит историю значений для одной метрики
type MetricRecord struct {
	mu     sync.RWMutex
	values []float64 // кольцевой буфер
	maxLen int
	// minSamples - минимум значений в буфере, до которого z-score не считается.
	// Без этого порога на 2-3 значениях разброс оценивается по шуму: два случайно
	// близких значения дают крошечный разброс, и следующее рядовое значение
	// получает z=10. Практически это означало бы шквал ложных срабатываний
	// сразу после старта, пока буфер не наберётся.
	minSamples int
	// scaleFloor - нижняя граница разброса. У метрик, которые почти всегда
	// стоят на месте (стейблкоины по цене, неликвид по объёму, стакан без
	// движения), MAD вырождается почти в ноль, и любое шевеление даёт z в
	// десятки. Пол разброса переводит такой z обратно в осмысленный диапазон.
	scaleFloor float64
}

func NewMetricRecord(maxLen, minSamples int, scaleFloor float64) *MetricRecord {
	if minSamples < 2 {
		minSamples = 2
	}
	if minSamples > maxLen {
		minSamples = maxLen
	}
	return &MetricRecord{
		values:     make([]float64, 0, maxLen),
		maxLen:     maxLen,
		minSamples: minSamples,
		scaleFloor: scaleFloor,
	}
}

func (mr *MetricRecord) Add(value float64) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.values = append(mr.values, value)
	if len(mr.values) > mr.maxLen {
		mr.values = mr.values[1:]
	}
}

func (mr *MetricRecord) Len() int {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	return len(mr.values)
}

// median возвращает медиану копии среза (сам срез не переупорядочивается)
func median(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// Scale - робастная оценка типичного разброса метрики (в тех же единицах, что
// и сама метрика). Это знаменатель z-score.
//
// Наружу нужна политике выхода: «типичное движение метрики» - честная мера
// того, что для неё много, а что шум.
func (mr *MetricRecord) Scale() float64 {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if len(mr.values) < mr.minSamples {
		return 0
	}
	return mr.scaleLocked()
}

// ZScore вычисляет робастный z-score для value ОТНОСИТЕЛЬНО ТЕКУЩЕЙ истории,
// не включая само value. Вызывающий код обязан вызвать ZScore ДО Add, иначе
// выброс попадает в собственную базу и занижает свой же z-score.
//
// Используются медиана и MAD (median absolute deviation), а не среднее и
// стандартное отклонение: у крипты тяжёлые хвосты, и одно прошлое экстремальное
// значение, осевшее в буфере, раздувает stddev настолько, что следующая реальная
// аномалия уже не проходит порог. Медиана и MAD к таким выбросам нечувствительны.
func (mr *MetricRecord) ZScore(value float64) float64 {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if len(mr.values) < mr.minSamples {
		return 0
	}

	scale := mr.scaleLocked()
	if scale == 0 {
		return 0
	}

	return (value - median(mr.values)) / scale
}

// scaleLocked - робастная оценка разброса: MAD, приведённый к шкале сигмы.
func (mr *MetricRecord) scaleLocked() float64 {
	_, scale := medianAndScale(mr.values, mr.scaleFloor)
	return scale
}

// medianAndScale - медиана и робастный разброс (MAD, приведённый к шкале
// сигмы) ОДНОМОМЕНТНОГО среза значений. Та же математика, что у
// MetricRecord.scaleLocked, но без истории по времени - для случаев, где
// сравнивать нужно значения внутри одного снапшота (например, объёмы уровней
// стакана прямо сейчас), а не текущее значение против прошлого.
func medianAndScale(values []float64, scaleFloor float64) (med, scale float64) {
	med = median(values)
	if len(values) == 0 {
		return 0, 0
	}

	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = math.Abs(v - med)
	}

	scale = median(deviations) * madToSigma

	// MAD вырождается в 0, если больше половины значений одинаковы (например,
	// объём стабильно нулевой). Тогда падаем на среднее абсолютное отклонение,
	// оно устойчивее к вырождению, но всё ещё робастнее stddev.
	if scale == 0 {
		sum := 0.0
		for _, d := range deviations {
			sum += d
		}
		scale = (sum / float64(len(deviations))) * meanADToSigma
	}

	if scale < scaleFloor {
		scale = scaleFloor
	}
	return med, scale
}

// MedianAndScale - экспортированная версия medianAndScale, для пакетов без
// истории по времени, которым нужна робастная медиана+разброс одного среза
// значений (см. internal/entrysetup - объёмы уровней стакана в снапшоте).
func MedianAndScale(values []float64, scaleFloor float64) (med, scale float64) {
	return medianAndScale(values, scaleFloor)
}
