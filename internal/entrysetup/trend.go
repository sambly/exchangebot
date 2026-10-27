package entrysetup

import (
	"sort"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
)

const (
	// trendLookbackCandles - сколько последних свечей периода разбирать для
	// оценки силы тренда. Заметно короче levelsLookbackCandles (100) -
	// трендовость это свойство ТЕКУЩЕГО режима рынка, а не всей доступной
	// истории; слишком длинное окно усредняет разные режимы в одно число и
	// запаздывает с переключением боковик↔тренд.
	trendLookbackCandles = 20
)

// GetTrendStrength - efficiency ratio (Kaufman) по последним
// trendLookbackCandles свечам периода: |цена сейчас - цена N свечей назад| /
// сумма |изменений| каждой свечи за то же окно. 1.0 - цена прошла весь путь
// по прямой (сильный тренд), 0.0 - вернулась туда же, откуда вышла, пройдя
// произвольно длинный зигзагообразный путь (чистый шум/боковик).
//
// В отличие от GetVolatilityRegime (та ось - "спокойно/бурно", АМПЛИТУДА
// движения относительно своей же нормы), это ось "трендово/боково" -
// НАПРАВЛЕННОСТЬ движения. Пара может быть одновременно и волатильной, и
// боковой (мечется широко, но никуда не идёт), и наоборот. Оба индикатора
// нужны по отдельности, не заменяют друг друга.
//
// Выбран efficiency ratio, а не классический ADX(14): ADX считается через
// Wilder-сглаживание, для которого нужно копить состояние (предыдущее
// сглаженное значение) между вызовами - то же усложнение, которого
// сознательно избежали PriceLevels (см. levelsFromCandles). Efficiency ratio -
// чистая функция уже полученного среза свечей, без состояния между вызовами.
func (s *AssetsSetup) GetTrendStrength(pair, period string) (float64, bool) {
	if s.prices == nil {
		return 0, false
	}

	duration, ok := s.prices.Periods[period]
	if !ok || duration <= 0 {
		return 0, false
	}

	from := time.Now().Add(-time.Duration(trendLookbackCandles) * duration)
	candles, err := s.prices.GetPeriodCandles(period, from)
	if err != nil {
		return 0, false
	}

	return trendStrengthFromCandles(filterPair(candles, pair))
}

// trendStrengthFromCandles - расчёт по уже отфильтрованной и НЕсортированной
// серии свечей одной пары. Вынесено отдельно от GetTrendStrength по тому же
// принципу, что levelsFromCandles - тестируется без БД и без AssetsSetup.
func trendStrengthFromCandles(series []exModel.Candle) (float64, bool) {
	if len(series) < 2 {
		return 0, false
	}
	sort.Slice(series, func(i, j int) bool { return series[i].Time.Before(series[j].Time) })

	net := series[len(series)-1].Close - series[0].Close
	if net < 0 {
		net = -net
	}

	var sum float64
	for i := 1; i < len(series); i++ {
		delta := series[i].Close - series[i-1].Close
		if delta < 0 {
			delta = -delta
		}
		sum += delta
	}
	if sum <= 0 {
		return 0, false
	}

	ratio := net / sum
	// Клип на случай накопленной погрешности плавающей точки - по построению
	// (net - это модуль ОДНОЙ разности из тех же слагаемых, что просуммированы
	// в sum по модулю) ratio математически не может быть > 1.
	if ratio > 1 {
		ratio = 1
	}
	return ratio, true
}
