package prices

import (
	"database/sql"
	"main/database"
	"main/model"
	"main/notification"
	"slices"
)

type AsetsPrices struct {
	Pairs          []string
	Periods        []string
	PeriodsDelta   []string
	WeightProcents map[string]float64
	MarketsStat    map[string]*model.MarketsStat
	ChangePrices   map[string]map[string]*ChangeData
	ChangeDelta    map[string]map[string][]ChangeDelta
	DeltaFast      map[string]map[string]*DeltaFast
	database       *sql.DB
	Notification   *notification.Notification
	LengthOfTime   int64
}

type ChangeData struct {
	LastPrice           float64
	LastVolume          float64
	СhangePercent       float64
	ChangePercentVolume float64
}
type ChangeDelta struct {
	Time        int64
	Volume      float64
	VolumeBuy   float64
	VolumeAsk   float64
	Trades      int64
	TradesBuy   int64
	TradesAsk   int64
	MinuteCount int32

	// For Candles
	Open  float64
	High  float64
	Low   float64
	Close float64
}

func (cd *ChangeDelta) Clear() {
	cd.Volume = 0
	cd.VolumeBuy = 0
	cd.VolumeAsk = 0
	cd.Trades = 0
	cd.TradesBuy = 0
	cd.TradesAsk = 0
	cd.MinuteCount = 0
}

type DeltaFast struct {
	Volume float64
	Trades float64
}

func (df *DeltaFast) Clear() {
	df.Volume = 0
	df.Trades = 0
}

func NewAssetsPrices(pairs, periods []string, weightProcents map[string]float64, lenghtTime int64, db *sql.DB, notification *notification.Notification) *AsetsPrices {
	asetsPrices := &AsetsPrices{
		Pairs:          pairs,
		Periods:        periods,
		PeriodsDelta:   []string{"5m", "30m", "1h", "4h", "1d"},
		WeightProcents: weightProcents,
		MarketsStat:    make(map[string]*model.MarketsStat),
		ChangePrices:   make(map[string]map[string]*ChangeData),
		ChangeDelta:    make(map[string]map[string][]ChangeDelta),
		DeltaFast:      make(map[string]map[string]*DeltaFast),
		database:       db,
		Notification:   notification,
		LengthOfTime:   lenghtTime,
	}
	for _, pair := range pairs {
		asetsPrices.MarketsStat[pair] = &model.MarketsStat{Pair: pair}
	}
	for _, pair := range pairs {
		if _, ok := asetsPrices.ChangePrices[pair]; !ok {
			asetsPrices.ChangePrices[pair] = map[string]*ChangeData{}
			asetsPrices.ChangeDelta[pair] = map[string][]ChangeDelta{}
			asetsPrices.DeltaFast[pair] = map[string]*DeltaFast{}
		}
		for _, period := range periods {
			asetsPrices.ChangePrices[pair][period] = &ChangeData{}
		}
		for _, period := range asetsPrices.PeriodsDelta {
			asetsPrices.ChangeDelta[pair][period] = []ChangeDelta{}
			asetsPrices.DeltaFast[pair][period] = &DeltaFast{}
		}

	}

	return asetsPrices
}

func (ap *AsetsPrices) OnMarket(ms model.MarketsStat) {

	if _, ok := ap.MarketsStat[ms.Pair]; !ok {
		ap.MarketsStat[ms.Pair] = &model.MarketsStat{}
	}

	ap.MarketsStat[ms.Pair].Pair = ms.Pair
	ap.MarketsStat[ms.Pair].Price = ms.Price
	ap.MarketsStat[ms.Pair].Time = ms.Time
	ap.MarketsStat[ms.Pair].Ch24 = ms.Ch24
	ap.MarketsStat[ms.Pair].Volume = ms.Volume

}

func (ap *AsetsPrices) UpdateChanges(period string) {

	for _, pair := range ap.Pairs {

		marketStat := ap.MarketsStat[pair]

		// Инициализация для всех переодов
		if period == "" {
			for _, periodInit := range ap.Periods {
				if marketStat.Price > 0 {
					ap.ChangePrices[pair][periodInit].LastPrice = ap.MarketsStat[pair].Price
					ap.ChangePrices[pair][periodInit].LastVolume = marketStat.Volume
				}
			}
		} else {
			changeData := ap.ChangePrices[pair][period]
			if changeData.LastPrice > 0 {
				changeData.СhangePercent = (marketStat.Price / changeData.LastPrice * 100) - 100
				changeData.ChangePercentVolume = (marketStat.Volume / changeData.LastVolume * 100) - 100
				changeData.LastPrice = marketStat.Price
				changeData.LastVolume = marketStat.Volume

				if changeData.СhangePercent >= ap.WeightProcents[period] {
					ap.Notification.NotificationWeightPercent(pair, period, changeData.СhangePercent)
				}

			} else {
				if marketStat.Price > 0 {
					changeData.LastPrice = marketStat.Price
					changeData.LastVolume = marketStat.Volume
				}
			}
		}
	}
}

func (ap *AsetsPrices) UpdateDelta() error {

	candles, err := database.SelectCandlesTable(ap.database)
	if err != nil {
		return err
	}

	frame := map[string]map[string]ChangeDelta{}

	for _, pair := range ap.Pairs {
		frame[pair] = map[string]ChangeDelta{}
		for _, period := range ap.PeriodsDelta {
			frame[pair][period] = ChangeDelta{}
			ap.ChangeDelta[pair][period] = nil
			ap.DeltaFast[pair][period].Clear()
		}
	}

	for _, candle := range candles {

		pair := candle.Pair
		if idx := slices.Index(ap.Pairs, pair); idx >= 0 {

			for key := range frame[pair] {
				frameCope := frame[pair][key]
				frameCope.Volume += candle.Volume
				frameCope.VolumeBuy += candle.ActiveBuyVolume
				frameCope.VolumeAsk += candle.ActiveAskVolume
				frameCope.Trades += candle.AmountTrade
				frameCope.TradesBuy += candle.AmountTradeBuy
				frameCope.TradesAsk += candle.AmountTradeAsk
				frameCope.MinuteCount += 1
				frameCope.Time = candle.Time.Unix()

				// for candles
				if frameCope.Open == 0 {
					frameCope.Open = candle.Open
				}
				if frameCope.High < candle.High {
					frameCope.High = candle.High
				}
				if frameCope.Low == 0 {
					frameCope.Low = candle.Low
				}
				if frameCope.Low > candle.Low {
					frameCope.Low = candle.Low
				}
				frameCope.Close = candle.Low

				frame[pair][key] = frameCope

			}

			if frame[pair]["5m"].MinuteCount == 5 {
				ap.ChangeDelta[pair]["5m"] = append(ap.ChangeDelta[pair]["5m"], frame[pair]["5m"])
				frame[pair]["5m"] = ChangeDelta{}
			}
			if frame[pair]["30m"].MinuteCount == 30 {
				ap.ChangeDelta[pair]["30m"] = append(ap.ChangeDelta[pair]["30m"], frame[pair]["30m"])
				frame[pair]["30m"] = ChangeDelta{}
			}
			if frame[pair]["1h"].MinuteCount == 60 {
				ap.ChangeDelta[pair]["1h"] = append(ap.ChangeDelta[pair]["1h"], frame[pair]["1h"])
				frame[pair]["1h"] = ChangeDelta{}
			}
			if frame[pair]["4h"].MinuteCount == 240 {
				ap.ChangeDelta[pair]["4h"] = append(ap.ChangeDelta[pair]["4h"], frame[pair]["4h"])
				frame[pair]["4h"] = ChangeDelta{}
			}
			if frame[pair]["1d"].MinuteCount == 720 {
				ap.ChangeDelta[pair]["1d"] = append(ap.ChangeDelta[pair]["1d"], frame[pair]["1d"])
				frame[pair]["1d"] = ChangeDelta{}
			}
		}

	}

	for _, pair := range ap.Pairs {
		for _, period := range ap.PeriodsDelta {
			val := ap.ChangeDelta[pair][period]
			if len(ap.ChangeDelta[pair][period]) >= 2 {
				ap.DeltaFast[pair][period].Volume = ap.ChangeDelta[pair][period][len(val)-1].Volume/ap.ChangeDelta[pair][period][len(val)-2].Volume*100 - 100
				ap.DeltaFast[pair][period].Trades = float64(val[len(val)-1].Trades)/float64(val[len(val)-2].Trades)*100 - 100
			}
		}

	}
	return nil
}
