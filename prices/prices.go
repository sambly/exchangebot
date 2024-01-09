package prices

import (
	"database/sql"
	"fmt"
	"main/database"
	"main/model"
	"main/notification"
	"slices"
)

type AsetsPrices struct {
	Pairs          []string
	Periods        []string
	WeightProcents map[string]float64
	MarketsStat    map[string]*model.MarketsStat
	ChangePrices   map[string]map[string]*ChangeData
	ChangeDelta    map[string]map[string][]*ChangeDelta
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
	Volume      float64
	VolumeBuy   float64
	VolumeAsk   float64
	Trades      int64
	TradesBuy   int64
	TradesAsk   int64
	MinuteCount int32
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
	Trades int64
}

func NewAssetsPrices(pairs, periods []string, weightProcents map[string]float64, lenghtTime int64, db *sql.DB, notification *notification.Notification) *AsetsPrices {
	asetsPrices := &AsetsPrices{
		Pairs:          pairs,
		Periods:        periods,
		WeightProcents: weightProcents,
		MarketsStat:    make(map[string]*model.MarketsStat),
		ChangePrices:   make(map[string]map[string]*ChangeData),
		ChangeDelta:    make(map[string]map[string][]*ChangeDelta),
		DeltaFast:      make(map[string]map[string]*DeltaFast),
		database:       db,
		Notification:   notification,
		LengthOfTime:   lenghtTime,
	}
	for _, pair := range pairs {
		asetsPrices.MarketsStat[pair] = &model.MarketsStat{Pair: pair}
	}
	for _, pair := range pairs {
		for _, period := range periods {
			if _, ok := asetsPrices.ChangePrices[pair]; !ok {
				asetsPrices.ChangePrices[pair] = map[string]*ChangeData{}
				asetsPrices.ChangeDelta[pair] = map[string][]*ChangeDelta{}
			}
			asetsPrices.ChangePrices[pair][period] = &ChangeData{}
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
	ap.MarketsStat[ms.Pair].Price = ms.Price
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

func (ap *AsetsPrices) UpdateDelta() {

	//rang := 10080 // minut (неделя)

	candles, err := database.SelectCandlesTable(ap.database)
	if err != nil {
		fmt.Println(err)
	}

	frame := map[string]map[string]*ChangeDelta{}

	for _, candle := range candles {

		if idx := slices.Index(ap.Pairs, candle.Pair); idx >= 0 {

			pair := candle.Pair

			if _, ok := frame[pair]; !ok {
				frame[pair] = map[string]*ChangeDelta{}
				frame[pair]["5m"] = &ChangeDelta{}
				frame[pair]["30m"] = &ChangeDelta{}
				frame[pair]["1h"] = &ChangeDelta{}
				frame[pair]["4h"] = &ChangeDelta{}
				frame[pair]["1d"] = &ChangeDelta{}
			}

			for key := range frame[pair] {
				frame[pair][key].Volume += candle.Volume
				frame[pair][key].VolumeBuy += candle.ActiveBuyVolume
				frame[pair][key].VolumeAsk += candle.ActiveAskVolume
				frame[pair][key].Trades += candle.AmountTrade
				frame[pair][key].TradesBuy += candle.AmountTradeBuy
				frame[pair][key].TradesAsk += candle.AmountTradeAsk
				frame[pair][key].MinuteCount += 1
			}

			switch {
			case frame[pair]["5m"].MinuteCount%5 == 0:
				ap.ChangeDelta[pair]["5m"] = append(ap.ChangeDelta[pair]["5m"], frame[pair]["5m"])
				frame[pair]["5m"].Clear()

			case frame[pair]["30m"].MinuteCount%30 == 0:
				ap.ChangeDelta[pair]["30m"] = append(ap.ChangeDelta[pair]["30m"], frame[pair]["30m"])
				frame[pair]["30m"].Clear()

			case frame[pair]["1h"].MinuteCount%60 == 0:
				ap.ChangeDelta[pair]["1h"] = append(ap.ChangeDelta[pair]["1h"], frame[pair]["1h"])
				frame[pair]["1h"].Clear()

			case frame[pair]["4h"].MinuteCount%240 == 0:
				ap.ChangeDelta[pair]["4h"] = append(ap.ChangeDelta[pair]["4h"], frame[pair]["4h"])
				frame[pair]["4h"].Clear()

			case frame[pair]["1d"].MinuteCount%720 == 0:
				ap.ChangeDelta[pair]["1d"] = append(ap.ChangeDelta[pair]["1d"], frame[pair]["1d"])
				frame[pair]["1d"].Clear()
			}
		}
	}

	period := []string{"5m", "30m", "1h", "4h", "1d"}

	for _, pair := range ap.Pairs {
		if _, ok := ap.DeltaFast[pair]; !ok {
			ap.DeltaFast[pair] = map[string]*DeltaFast{}
			ap.DeltaFast[pair]["5m"] = &DeltaFast{}
			ap.DeltaFast[pair]["30m"] = &DeltaFast{}
			ap.DeltaFast[pair]["1h"] = &DeltaFast{}
			ap.DeltaFast[pair]["4h"] = &DeltaFast{}
			ap.DeltaFast[pair]["1d"] = &DeltaFast{}
		}

		for _, per := range period {
			if len(ap.ChangeDelta[pair][per]) > 2 {
				ap.DeltaFast[pair][per].Volume = ap.ChangeDelta[pair][per][len(ap.ChangeDelta[pair][per])-1].Volume - ap.ChangeDelta[pair][per][len(ap.ChangeDelta[pair][per])-2].Volume
				ap.DeltaFast[pair][per].Trades = ap.ChangeDelta[pair][per][len(ap.ChangeDelta[pair][per])-1].Trades - ap.ChangeDelta[pair][per][len(ap.ChangeDelta[pair][per])-2].Trades
			}
		}

	}

}
