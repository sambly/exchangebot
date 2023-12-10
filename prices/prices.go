package prices

import (
	"main/model"
	"main/notification"
)

type AsetsPrices struct {
	Pairs          []string
	Periods        []string
	WeightProcents map[string]float64
	MarketsStat    map[string]*model.MarketsStat
	ChangePrices   map[string]map[string]*ChangeData
	Notification   *notification.Notification
}

type ChangeData struct {
	LastPrice           float64
	LastVolume          float64
	СhangePercent       float64
	ChangePercentVolume float64
}

func NewAssetsPrices(pairs, periods []string, weightProcents map[string]float64, notification *notification.Notification) *AsetsPrices {
	asetsPrices := &AsetsPrices{
		Pairs:          pairs,
		Periods:        periods,
		WeightProcents: weightProcents,
		MarketsStat:    make(map[string]*model.MarketsStat),
		ChangePrices:   make(map[string]map[string]*ChangeData),
		Notification:   notification,
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
		changeData := ap.ChangePrices[pair][period]

		// Инициализация для всех переодов
		if period == "" {
			for _, periodInit := range ap.Periods {
				if marketStat.Price > 0 {
					ap.ChangePrices[pair][periodInit].LastPrice = ap.MarketsStat[pair].Price
					ap.ChangePrices[pair][periodInit].LastVolume = marketStat.Volume
				}
			}
		} else {
			if changeData.LastPrice > 0 {
				changeData.СhangePercent = (marketStat.Price / changeData.LastPrice * 100) - 100
				changeData.ChangePercentVolume = (marketStat.Volume / changeData.LastVolume * 100) - 100
				changeData.LastPrice = marketStat.Price
				changeData.LastVolume = marketStat.Volume

				// if changeData.СhangePercent >= changeData.WeightPercent {
				// 	a.telegram.NotificationWeightPercent(pair, period, changeData.СhangePercent)
				// }

			} else {
				if marketStat.Price > 0 {
					changeData.LastPrice = marketStat.Price
					changeData.LastVolume = marketStat.Volume
				}
			}
		}

		if changeData.СhangePercent >= ap.WeightProcents[period] {
			ap.Notification.NotificationWeightPercent(pair, period, changeData.СhangePercent)

		}

	}

}
