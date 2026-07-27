package prices

import (
	"sort"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/stat"
)

// Параметры трекера волатильности. Это НЕ детектор аномалий - это грубая,
// общая оценка "сколько эта пара обычно проходит за период" для любого
// потребителя, которому она нужна (см. internal/entrysetup). Точная,
// настраиваемая по периодам история для z-score живёт своя, отдельная - в
// internal/strategy/anomaly, и намеренно не трогается: она уже провалидирована
// бэктестом, а эти константы её не заменяют.
const (
	volatilityWindowSize = 30
	volatilityMinSamples = 20
	volatilityScaleFloor = 0.35

	// regimeShortWindow/regimeShortMinSamples - короткое окно, которое живёт
	// рядом с длинным (volatilityWindowSize) на тех же выборках: сравнение
	// "недавнего" разброса с "типичным" и есть режим волатильности (см.
	// GetVolatilityRegime). Больше окно - более сглаженная (и более
	// запаздывающая) оценка "недавнего".
	regimeShortWindow     = 8
	regimeShortMinSamples = 5
)

// volatilityState - история изменения цены (%) для одной пары+периода.
type volatilityState struct {
	// record - полное окно, для GetVolatility (типичный масштаб движения).
	record *stat.MetricRecord
	// shortRecord - то же самое окно, но короче: та же история, только
	// последние regimeShortWindow выборок - для GetVolatilityRegime.
	shortRecord *stat.MetricRecord
	// nextSampleAt - следующее значение попадёт в историю не раньше этого
	// момента: пишем не чаще раза в period, иначе соседние минутные снимки
	// ChangePrices почти идентичны (окно сдвигается на минуту из period) и
	// разброс занижается - та же автокорреляция, что и в anomaly (см.
	// PeriodState.nextSampleAt там).
	nextSampleAt time.Time
}

// initVolatility создаёт историю для каждой пары+периода и сеет её из БД -
// без сидирования трекер набирал бы историю несколько суток аптайма.
func (ap *AssetsPrices) initVolatility() {
	ap.volatility = make(map[string]map[string]*volatilityState, len(ap.Pairs))
	for _, pair := range ap.Pairs {
		ap.volatility[pair] = make(map[string]*volatilityState, len(ap.Periods))
		for period := range ap.Periods {
			ap.volatility[pair][period] = &volatilityState{
				record:      stat.NewMetricRecord(volatilityWindowSize, volatilityMinSamples, volatilityScaleFloor),
				shortRecord: stat.NewMetricRecord(regimeShortWindow, regimeShortMinSamples, volatilityScaleFloor),
			}
		}
	}
	ap.seedVolatility()
}

// seedVolatility заполняет историю из свечей БД: одна свеча периода = одна
// выборка (close(t)/close(t-P)-1), тот же приём, что у anomaly.seedHistory.
func (ap *AssetsPrices) seedVolatility() {
	now := time.Now()

	for period, duration := range ap.Periods {
		if duration <= 0 {
			continue
		}

		from := now.Add(-time.Duration(volatilityWindowSize+1) * duration)
		candles, err := ap.repo.SelectCandlesFromPeriod(period, from)
		if err != nil {
			pricesLogger.Errorf("не удалось прочитать свечи %s для сидирования волатильности: %v", period, err)
			continue
		}

		byPair := make(map[string][]exModel.Candle)
		for _, c := range candles {
			if _, tracked := ap.volatility[c.Pair]; !tracked {
				continue
			}
			byPair[c.Pair] = append(byPair[c.Pair], c)
		}

		for pair, series := range byPair {
			sort.Slice(series, func(i, j int) bool { return series[i].Time.Before(series[j].Time) })

			state := ap.volatility[pair][period]
			for i := 1; i < len(series); i++ {
				prev, cur := series[i-1], series[i]

				gap := cur.Time.Sub(prev.Time)
				if gap < duration/2 || gap > duration*3/2 {
					continue
				}
				value := checkValuesDividing(cur.Close, prev.Close)
				state.record.Add(value)
				state.shortRecord.Add(value)
			}
			state.nextSampleAt = now.Add(duration)
		}
	}
}

// recordVolatility добавляет свежий ChangePercent в историю, если период с
// прошлой записи уже прошёл. Вызывается из updateChangePrices - там же, где
// changePercent и так уже посчитан.
func (ap *AssetsPrices) recordVolatility(pair, period string, changePercent float64, now time.Time) {
	byPeriod, ok := ap.volatility[pair]
	if !ok {
		return
	}
	state, ok := byPeriod[period]
	if !ok || state == nil || now.Before(state.nextSampleAt) {
		return
	}

	state.record.Add(changePercent)
	state.shortRecord.Add(changePercent)
	state.nextSampleAt = now.Add(ap.Periods[period])
}

// GetVolatility - типичное движение цены пары за период, % (робастная оценка
// разброса: медиана+MAD, устойчива к тяжёлым хвостам, характерным для крипты).
//
// Это грубая ориентировочная оценка "сколько пара обычно проходит", а не
// точная статистика для принятия торговых решений - для этого есть
// internal/strategy/anomaly со своей, отдельно настраиваемой историей.
// ok=false, пока не накопилось volatilityMinSamples выборок.
func (ap *AssetsPrices) GetVolatility(pair, period string) (float64, bool) {
	byPeriod, ok := ap.volatility[pair]
	if !ok {
		return 0, false
	}
	state, ok := byPeriod[period]
	if !ok || state == nil || state.record.Len() < volatilityMinSamples {
		return 0, false
	}
	return state.record.Scale(), true
}

// GetVolatilityRegime - отношение НЕДАВНЕЙ волатильности (последние
// regimeShortWindow выборок) к ТИПИЧНОЙ (volatilityWindowSize выборок).
//
// < 1 - сжатие: пара двигалась в последнее время меньше обычного. Часто
// предшествует выносу - и именно в этот момент стоп можно ставить теснее
// (недавний диапазон был узким), не жертвуя вероятностью, что его выбьет шум.
// > 1 - расширение: пара уже разогналась сильнее обычного.
//
// Как и GetVolatility, это не рекомендация и не порог "хорошо/плохо" - решать,
// что считать сжатием (0.5? 0.7?), должен потребитель.
func (ap *AssetsPrices) GetVolatilityRegime(pair, period string) (float64, bool) {
	byPeriod, ok := ap.volatility[pair]
	if !ok {
		return 0, false
	}
	state, ok := byPeriod[period]
	if !ok || state == nil {
		return 0, false
	}
	if state.record.Len() < volatilityMinSamples || state.shortRecord.Len() < regimeShortMinSamples {
		return 0, false
	}

	long := state.record.Scale()
	if long <= 0 {
		return 0, false
	}
	return state.shortRecord.Scale() / long, true
}
