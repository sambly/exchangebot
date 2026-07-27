package entrysetup

import (
	"sort"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
)

const (
	// levelsLookbackCandles - сколько свечей периода разбирать в поисках
	// разворотных точек. Больше - глубже история (уровни надёжнее, т.к.
	// подтверждены не одним случайным движением), но и найденный уровень может
	// оказаться дальше от текущей цены, чем реально пригодится.
	levelsLookbackCandles = 100

	// levelsSwingWindow - свеча считается разворотной точкой (свечной
	// "фрактал"), если её high/low - локальный экстремум среди
	// levelsSwingWindow соседей С КАЖДОЙ стороны. Больше - меньше уровней, но
	// каждый значимее; меньше - уровней больше, но часть - шум одной свечи.
	levelsSwingWindow = 5
)

// PriceLevel - разворотная точка на графике цены (swing high/low).
type PriceLevel struct {
	Price float64
	Time  time.Time
	// DistancePercent - расстояние от текущей цены, всегда >=0.
	DistancePercent float64
}

// PriceLevels - ближайшие уровни поддержки/сопротивления ПО ИСТОРИИ ЦЕНЫ, а
// не по стакану (см. Walls).
//
// В отличие от стен в стакане, эти уровни нельзя снять за секунду выставленной
// и тут же отменённой заявкой - они уже СОСТОЯЛИСЬ: цена реально
// разворачивалась здесь хотя бы раз за levelsLookbackCandles свечей. Плата за
// эту устойчивость - запаздывание: только что сформировавшийся уровень (или
// уровень внутри последних levelsSwingWindow свечей, которым ещё не с чем
// сравниться справа) пакет не увидит.
type PriceLevels struct {
	Pair   string
	Period string

	Support    PriceLevel
	HasSupport bool

	Resistance    PriceLevel
	HasResistance bool
}

// GetPriceLevels ищет ближайшие к текущей цене swing high/low за
// levelsLookbackCandles свечей периода. false, если по паре+периоду нет
// вообще ни одного уровня ни с одной стороны.
func (s *AssetsSetup) GetPriceLevels(pair, period string) (PriceLevels, bool) {
	if s.prices == nil {
		return PriceLevels{}, false
	}

	duration, ok := s.prices.Periods[period]
	if !ok || duration <= 0 {
		return PriceLevels{}, false
	}

	from := time.Now().Add(-time.Duration(levelsLookbackCandles) * duration)
	candles, err := s.prices.GetPeriodCandles(period, from)
	if err != nil {
		return PriceLevels{}, false
	}

	return levelsFromCandles(pair, period, filterPair(candles, pair))
}

// GetAllPriceLevels - GetPriceLevels сразу по всем отслеживаемым парам и
// периодам, для таблицы "по рынку целиком".
//
// В отличие от GetAllQuality, свечи запрашиваются ОДИН РАЗ НА ПЕРИОД (не на
// пару): GetPeriodCandles и так возвращает свечи всех пар сразу, и запрашивать
// его в цикле по парам значило бы N раз тянуть из БД один и тот же набор,
// выбрасывая из него все пары кроме одной.
//
// PriceLevels, в отличие от Walls/Quality, не зависит от стакана - поэтому
// здесь пары берутся из prices.Pairs, а не из depth.Pairs.
func (s *AssetsSetup) GetAllPriceLevels() map[string]map[string]PriceLevels {
	if s.prices == nil {
		return make(map[string]map[string]PriceLevels)
	}

	result := make(map[string]map[string]PriceLevels, len(s.prices.Pairs))
	for _, pair := range s.prices.Pairs {
		result[pair] = make(map[string]PriceLevels, len(s.prices.Periods))
	}

	for period, duration := range s.prices.Periods {
		if duration <= 0 {
			continue
		}

		from := time.Now().Add(-time.Duration(levelsLookbackCandles) * duration)
		candles, err := s.prices.GetPeriodCandles(period, from)
		if err != nil {
			continue
		}

		byPair := make(map[string][]exModel.Candle)
		for _, c := range candles {
			if _, tracked := result[c.Pair]; tracked {
				byPair[c.Pair] = append(byPair[c.Pair], c)
			}
		}

		for pair, series := range byPair {
			if levels, ok := levelsFromCandles(pair, period, series); ok {
				result[pair][period] = levels
			}
		}
	}

	return result
}

// filterPair - копия candles, относящаяся только к pair. GetPeriodCandles
// отдаёт свечи всех пар вперемешку (SelectCandlesFromPeriod), фильтровать
// нужно самому.
func filterPair(candles []exModel.Candle, pair string) []exModel.Candle {
	series := make([]exModel.Candle, 0, len(candles))
	for _, c := range candles {
		if c.Pair == pair {
			series = append(series, c)
		}
	}
	return series
}

// levelsFromCandles - расчёт уровней по уже отфильтрованной и НЕсортированной
// серии свечей одной пары. Вынесено отдельно от GetPriceLevels, чтобы
// GetAllPriceLevels могла один раз получить свечи периода и раздать их по
// парам, не завязываясь при этом на детали чтения из БД.
func levelsFromCandles(pair, period string, series []exModel.Candle) (PriceLevels, bool) {
	if len(series) < levelsSwingWindow*2+1 {
		return PriceLevels{}, false
	}
	sort.Slice(series, func(i, j int) bool { return series[i].Time.Before(series[j].Time) })

	current := series[len(series)-1].Close
	if current <= 0 {
		return PriceLevels{}, false
	}

	levels := PriceLevels{Pair: pair, Period: period}

	// resistance/support ищем как "ближайший", а не "самый выраженный":
	// дальний уровень бесполезен для стопа/тейка, даже если он трогался чаще -
	// тот же принцип, что у findWall в стенах стакана.
	for i := levelsSwingWindow; i < len(series)-levelsSwingWindow; i++ {
		if isSwingHigh(series, i) {
			price := series[i].High
			if price > current && (!levels.HasResistance || price < levels.Resistance.Price) {
				levels.Resistance = PriceLevel{
					Price:           price,
					Time:            series[i].Time,
					DistancePercent: (price - current) / current * 100,
				}
				levels.HasResistance = true
			}
		}
		if isSwingLow(series, i) {
			price := series[i].Low
			if price < current && price > 0 && (!levels.HasSupport || price > levels.Support.Price) {
				levels.Support = PriceLevel{
					Price:           price,
					Time:            series[i].Time,
					DistancePercent: (current - price) / current * 100,
				}
				levels.HasSupport = true
			}
		}
	}

	return levels, levels.HasSupport || levels.HasResistance
}

// isSwingHigh - High свечи i строго больше High всех соседей в пределах
// levelsSwingWindow. Ties (соседняя свеча с тем же High) намеренно не считаются
// разворотом: на плоском участке иначе получилась бы "стена" разворотных точек
// подряд, а разворот там один - в лучшем случае.
func isSwingHigh(series []exModel.Candle, i int) bool {
	for j := i - levelsSwingWindow; j <= i+levelsSwingWindow; j++ {
		if j != i && series[j].High >= series[i].High {
			return false
		}
	}
	return true
}

// isSwingLow - зеркально isSwingHigh, по Low.
func isSwingLow(series []exModel.Candle, i int) bool {
	for j := i - levelsSwingWindow; j <= i+levelsSwingWindow; j++ {
		if j != i && series[j].Low <= series[i].Low {
			return false
		}
	}
	return true
}
