package prices

import (
	"fmt"
	"main/model"
	"sync"
)

type AsetsPrices struct {
	mtx         sync.Mutex
	MarketsStat map[string]*model.MarketsStat
	Change      map[string]*ChangeData // Словарь где ключ элемент periods, "ch3_m" value изменения за этот период
}

type ChangeData struct {
	LastPrice           float64
	LastVolume          float64
	ChangePercent       float64
	ChangePercentVolume float64
	WeightPercent       float64
}

func NewAssetsPrices() (*AsetsPrices, error) {
	asetsPrices := &AsetsPrices{
		MarketsStat: make(map[string]*model.MarketsStat),
		Change:      map[string]*ChangeData{},
	}
	return asetsPrices, nil
}

func (ap *AsetsPrices) OnMarket(ms model.MarketsStat) {
	ap.mtx.Lock()
	defer ap.mtx.Lock()

	if _, ok := ap.MarketsStat[ms.Pair]; !ok {
		ap.MarketsStat[ms.Pair] = &model.MarketsStat{}
	}

	fmt.Println("OnMarket true")
	ap.MarketsStat[ms.Pair].Price = ms.Price
	ap.MarketsStat[ms.Pair].Time = ms.Time
	ap.MarketsStat[ms.Pair].Price = ms.Price
	ap.MarketsStat[ms.Pair].Ch24 = ms.Ch24
	ap.MarketsStat[ms.Pair].Volume = ms.Volume

}
