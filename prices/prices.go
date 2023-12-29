package prices

import (
	"database/sql"
	"fmt"
	"main/database"
	"main/model"
	"main/notification"
)

type AsetsPrices struct {
	Pairs          []string
	Periods        []string
	WeightProcents map[string]float64
	MarketsStat    map[string]*model.MarketsStat
	ChangePrices   map[string]map[string]*ChangeData
	ChangeDelta    map[string]map[string][]*ChangeDelta
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
	VolumeDelta float64
	VolumeBuy   float64
	VolumeAsk   float64
	Trades      int64
	TradesDelta int64
	TradesBuy   int64
	TradesAsk   int64
}

func NewAssetsPrices(pairs, periods []string, weightProcents map[string]float64, lenghtTime int64, db *sql.DB, notification *notification.Notification) *AsetsPrices {
	asetsPrices := &AsetsPrices{
		Pairs:          pairs,
		Periods:        periods,
		WeightProcents: weightProcents,
		MarketsStat:    make(map[string]*model.MarketsStat),
		ChangePrices:   make(map[string]map[string]*ChangeData),
		ChangeDelta:    make(map[string]map[string][]*ChangeDelta),
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
			//asetsPrices.ChangeDelta[pair][period] = ChangeDelta{}
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

	for _, pair := range ap.Pairs {
		minute := 1
		volume := map[string]float64{"5m": 0, "30m": 0, "1h": 0, "4h": 0, "1d": 0}
		for _, candle := range candles {
			if candle.Pair == pair {

				for key := range volume {
					volume[key] += candle.Volume
				}

				switch {

				case minute%5 == 0:
					ap.ChangeDelta[pair]["5m"] = append(ap.ChangeDelta[pair]["5m"], &ChangeDelta{Volume: volume["5m"]})
					volume["5m"] = 0
				case minute%30 == 0:
					ap.ChangeDelta[pair]["30m"] = append(ap.ChangeDelta[pair]["30m"], &ChangeDelta{Volume: volume["30m"]})
					volume["30m"] = 0
				case minute%60 == 0:
					ap.ChangeDelta[pair]["1h"] = append(ap.ChangeDelta[pair]["1h"], &ChangeDelta{Volume: volume["1h"]})
					volume["1h"] = 0
				case minute%240 == 0:
					ap.ChangeDelta[pair]["4h"] = append(ap.ChangeDelta[pair]["4h"], &ChangeDelta{Volume: volume["4h"]})
					volume["4h"] = 0
				case minute%720 == 0:
					ap.ChangeDelta[pair]["1d"] = append(ap.ChangeDelta[pair]["1d"], &ChangeDelta{Volume: volume["1d"]})
					volume["1d"] = 0
				}

				minute += 1
			}

		}

	}

}
