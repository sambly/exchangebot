package prices

import (
	"math"
	"sort"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/stat"
)

// Параметры трекера buy-активности. Та же изоляция, что и у volatility.go:
// своя, отдельная от internal/strategy/anomaly история. Там уже считается
// z-score по VolumeBuy/TradesBuy, но он не наружу - и намеренно подавляется,
// если цена ещё не сдвинулась (см. classify() в anomaly.go). Именно момент
// "покупают активнее обычного, а цена ещё не в курсе" и нужен этому трекеру -
// заводить для этого зависимость от чужого, настраиваемого из телеграма и
// уже провалидированного бэктестом детектора значило бы привязать дашборд к
// поведению, которое может измениться по причинам, не имеющим отношения к
// дашборду.
const (
	activityWindowSize = 30
	activityMinSamples = 20
	// activityScaleFloor - в шкале stat.LogRatio, не в %: log(1.05) ~= 0.05,
	// то есть пол разброса - это движение уровня "на 5%".
	activityScaleFloor = 0.05
)

// activityState - история buy-side активности одной пары+периода: объём и
// число сделок, инициированных покупателем. Две метрики, а не одна - они
// обычно скоррелированы, но не всегда (одна крупная заявка даёт всплеск
// объёма без всплеска числа сделок, и наоборот); берётся максимум |z| по
// обеим (см. GetBuyActivityZScore), тот же принцип, что у activityZ в
// anomaly.detect.
type activityState struct {
	volumeBuy *stat.MetricRecord
	tradesBuy *stat.MetricRecord

	// nextSampleAt - см. volatilityState.nextSampleAt: та же защита от
	// автокорреляции, ChangeDelta пересчитывается каждую минуту скользящим
	// окном, в историю пишем не чаще раза в period.
	nextSampleAt time.Time
}

func (ap *AssetsPrices) initActivity() {
	ap.activity = make(map[string]map[string]*activityState, len(ap.Pairs))
	for _, pair := range ap.Pairs {
		ap.activity[pair] = make(map[string]*activityState, len(ap.PeriodsDelta))
		for period := range ap.PeriodsDelta {
			ap.activity[pair][period] = &activityState{
				volumeBuy: stat.NewMetricRecord(activityWindowSize, activityMinSamples, activityScaleFloor),
				tradesBuy: stat.NewMetricRecord(activityWindowSize, activityMinSamples, activityScaleFloor),
			}
		}
	}
	ap.seedActivity()
}

// seedActivity - см. seedVolatility: свеча периода = одна выборка истории,
// без сидирования трекер на длинных периодах ждал бы боевой готовности сутками.
func (ap *AssetsPrices) seedActivity() {
	now := time.Now()

	for period, duration := range ap.PeriodsDelta {
		if duration <= 0 {
			continue
		}

		from := now.Add(-time.Duration(activityWindowSize+1) * duration)
		candles, err := ap.repo.SelectCandlesFromPeriod(period, from)
		if err != nil {
			pricesLogger.Errorf("не удалось прочитать свечи %s для сидирования buy-активности: %v", period, err)
			continue
		}

		byPair := make(map[string][]exModel.Candle)
		for _, c := range candles {
			if _, tracked := ap.activity[c.Pair]; !tracked {
				continue
			}
			byPair[c.Pair] = append(byPair[c.Pair], c)
		}

		for pair, series := range byPair {
			sort.Slice(series, func(i, j int) bool { return series[i].Time.Before(series[j].Time) })

			state := ap.activity[pair][period]
			for i := 1; i < len(series); i++ {
				prev, cur := series[i-1], series[i]

				gap := cur.Time.Sub(prev.Time)
				if gap < duration/2 || gap > duration*3/2 {
					continue
				}

				state.volumeBuy.Add(stat.LogRatio(checkValuesDividing(cur.ActiveBuyVolume, prev.ActiveBuyVolume)))
				state.tradesBuy.Add(stat.LogRatio(checkValuesDividing(float64(cur.AmountTradeBuy), float64(prev.AmountTradeBuy))))
			}
			state.nextSampleAt = now.Add(duration)
		}
	}
}

// recordActivity - см. recordVolatility: пишем в историю не чаще раза в
// period, вызывается из recalcDelta сразу после пересчёта ChangeDelta.
func (ap *AssetsPrices) recordActivity(pair, period string, delta ChangeDelta, now time.Time) {
	byPeriod, ok := ap.activity[pair]
	if !ok {
		return
	}
	state, ok := byPeriod[period]
	if !ok || state == nil || now.Before(state.nextSampleAt) {
		return
	}

	state.volumeBuy.Add(stat.LogRatio(delta.VolumeBuy))
	state.tradesBuy.Add(stat.LogRatio(delta.TradesBuy))
	state.nextSampleAt = now.Add(ap.PeriodsDelta[period])
}

// GetBuyActivityZScore - на сколько активность покупателей (объём и число
// сделок, инициированных покупателем) сейчас необычна для этой пары
// относительно её собственной истории. Положительный z - покупают активнее
// обычного; в отличие от GetVolatilityRegime у этого показателя есть знак.
//
// "Текущее" значение читается напрямую из ChangeDelta (живое, обновляется
// каждую минуту), а не из истории - тот же приём, что у
// depth.GetImbalanceZScore: история копится реже (см. nextSampleAt), проверка
// может идти чаще.
func (ap *AssetsPrices) GetBuyActivityZScore(pair, period string) (float64, bool) {
	byPeriod, ok := ap.activity[pair]
	if !ok {
		return 0, false
	}
	state, ok := byPeriod[period]
	if !ok || state == nil {
		return 0, false
	}
	if state.volumeBuy.Len() < activityMinSamples && state.tradesBuy.Len() < activityMinSamples {
		return 0, false
	}

	delta, ok := ap.GetChangeDelta(pair, period)
	if !ok {
		return 0, false
	}

	zVolume := state.volumeBuy.ZScore(stat.LogRatio(delta.VolumeBuy))
	zTrades := state.tradesBuy.ZScore(stat.LogRatio(delta.TradesBuy))

	if math.Abs(zTrades) > math.Abs(zVolume) {
		return zTrades, true
	}
	return zVolume, true
}
