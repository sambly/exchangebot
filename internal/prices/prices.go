package prices

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/model"

	exModel "github.com/sambly/exchangeService/pkg/model"
)

type Repository interface {
	InsertCandle(candle exModel.Candle, period string) error
	InsertCandles(candles []exModel.Candle, period string) error
	GetCandlesByPeriod(period string) ([]exModel.Candle, error)
	SelectMarketStateTimev2(timeRounding time.Time) ([]exModel.Candle, error)
	SelectDeltaPeriod(pair string, period string) ([]model.ChangeDeltaForCandle, error)
}

type ChangePrices struct {
	LastPrice     float64
	ChangePercent float64
}

type ChangePricesDataset struct {
	dataset []DatasetChangePrices
	Fill    bool
}

type DatasetChangePrices struct {
	Price float64
	Time  time.Time
}
type ChangeDelta struct {
	Time      time.Time `json:"-"`
	Volume    float64   `json:"Volume"`
	VolumeBuy float64   `json:"VolumeBuy"`
	VolumeAsk float64   `json:"VolumeAsk"`
	Trades    float64   `json:"Trades"`
	TradesBuy float64   `json:"TradesBuy"`
	TradesAsk float64   `json:"TradesAsk"`
}

type ChangeDeltaDataset struct {
	dataset []ChangeDelta
	fill    bool
}

type AssetsPrices struct {
	repo Repository

	Pairs        []string
	Periods      map[string]time.Duration
	PeriodsDelta map[string]time.Duration

	UpdateTime   time.Time
	UpdateChanel chan struct{}

	// Актуальные данные для каждой пары. Price, 24ch, Volume
	MarketsStatMu sync.RWMutex
	MarketsStat   map[string]*exModel.MarketsStat

	ChangePricesMu      sync.RWMutex
	ChangePrices        map[string]map[string]*ChangePrices
	ChangePricesDataset map[string]map[string]*ChangePricesDataset

	ChangeDeltaMu      sync.RWMutex
	ChangeDelta        map[string]map[string]*ChangeDelta
	ChangeDeltaDataset map[string]map[string]*ChangeDeltaDataset
}

var pricesLogger = logger.AddFieldsEmpty()

func NewAssetsPrices(pairs []string, periodsChange, periodsDelta map[string]time.Duration, repo Repository) (*AssetsPrices, error) {
	assetsPrices := &AssetsPrices{
		Pairs:        pairs,
		Periods:      periodsChange,
		PeriodsDelta: periodsDelta,

		UpdateChanel: make(chan struct{}),

		MarketsStat: make(map[string]*exModel.MarketsStat),

		ChangePrices:        make(map[string]map[string]*ChangePrices),
		ChangePricesDataset: make(map[string]map[string]*ChangePricesDataset),

		ChangeDelta:        make(map[string]map[string]*ChangeDelta),
		ChangeDeltaDataset: make(map[string]map[string]*ChangeDeltaDataset),

		repo: repo,
	}

	for _, pair := range pairs {
		assetsPrices.MarketsStat[pair] = &exModel.MarketsStat{Pair: pair}

		assetsPrices.ChangePrices[pair] = map[string]*ChangePrices{}
		assetsPrices.ChangePricesDataset[pair] = map[string]*ChangePricesDataset{}
		assetsPrices.ChangeDelta[pair] = map[string]*ChangeDelta{}
		assetsPrices.ChangeDeltaDataset[pair] = map[string]*ChangeDeltaDataset{}

		for period := range periodsChange {
			assetsPrices.ChangePrices[pair][period] = &ChangePrices{}
			assetsPrices.ChangePricesDataset[pair][period] = &ChangePricesDataset{}
		}
		for period := range periodsDelta {
			assetsPrices.ChangeDelta[pair][period] = &ChangeDelta{}
			assetsPrices.ChangeDeltaDataset[pair][period] = &ChangeDeltaDataset{}
		}
	}
	timeRounding := time.Now().Truncate(time.Minute)
	assetsPrices.UpdateTime = timeRounding
	assetsPrices.initChangePrices()
	assetsPrices.initChangeDelta()

	return assetsPrices, nil
}

func (ap *AssetsPrices) OnMarket(ms exModel.MarketsStat) {

	ap.MarketsStatMu.Lock()
	defer ap.MarketsStatMu.Unlock()

	if _, ok := ap.MarketsStat[ms.Pair]; !ok {
		return
	}
	ap.MarketsStat[ms.Pair].Pair = ms.Pair
	ap.MarketsStat[ms.Pair].Price = ms.Price
	ap.MarketsStat[ms.Pair].Time = ms.Time
	ap.MarketsStat[ms.Pair].Ch24 = ms.Ch24
	ap.MarketsStat[ms.Pair].Volume = ms.Volume

	if ms.Time.Sub(ap.UpdateTime) >= time.Duration(time.Minute) {
		ap.UpdateTime = ms.Time.Truncate(time.Minute)

		go func() {
			// За это время ждем пока остальные пары обновят цену, не точное решение...
			time.Sleep(1 * time.Second)
			ap.updateChangePrices()
			select {
			case ap.UpdateChanel <- struct{}{}:
			default:
			}

		}()

		go func() {
			// Ожидание пока данные запишутся в базу данных,данные пишутся в базу данных с feederApp, потом мы считаем новые значения
			// В принципе есть время на это учитывая что запись производим каждую минуту
			time.Sleep(10 * time.Second)
			if err := ap.updateChangeDelta(); err != nil {
				pricesLogger.Errorf("error in updateDelta: %v", err)
			}
		}()
	}
}

func (ap *AssetsPrices) initChangePrices() {

	// Максимальный заданный период для запроса в бд
	var max time.Duration
	for _, dur := range ap.Periods {
		if dur > max {
			max = dur
		}
	}

	// Интервал времени текущее время - время макс. периода
	timeRoundingMax := ap.UpdateTime.Add(-max)

	candles, err := ap.repo.SelectMarketStateTimev2(timeRoundingMax)
	if err != nil {
		pricesLogger.Errorf("error SelectMarketStateTimev2: %v", err)
		return
	}

	if len(candles) == 0 {
		pricesLogger.Info("Нет свечей")
		return
	}

	// candles[0] -самый актуальный candle
	// сравнение времени candle с текущим временем
	if candles[0].Time.Sub(ap.UpdateTime) > 10*time.Minute {
		pricesLogger.Info("В базе данных отсутствуют данные за период, горячий перезапуск не удался")
		return
	}

	for _, candle := range candles {

		if !slices.Contains(ap.Pairs, candle.Pair) {
			continue
		}

		for period, periodValue := range ap.Periods {

			data := ap.ChangePricesDataset[candle.Pair][period]

			if !data.Fill {

				item := DatasetChangePrices{
					Price: candle.Close,
					Time:  candle.Time,
				}

				if len(data.dataset) == int(periodValue.Minutes()-1) {

					data.dataset = append(data.dataset, item)
					ap.ChangePrices[candle.Pair][period].LastPrice = data.dataset[len(data.dataset)-1].Price
					ap.ChangePricesDataset[candle.Pair][period].Fill = true
					ap.ChangePricesDataset[candle.Pair][period].dataset = data.dataset

				} else {

					// Большая погрешность дальше не заполняем  DatasetChangePrices
					if len(data.dataset) > 0 && data.dataset[len(data.dataset)-1].Time.Sub(candle.Time) > 10*time.Minute {
						continue
					}
					ap.ChangePricesDataset[candle.Pair][period].dataset = append(data.dataset, item)
				}
			}
		}
	}
}

func (ap *AssetsPrices) initChangeDelta() {

	// Максимальный заданный период для запроса в бд
	var max time.Duration
	for _, dur := range ap.Periods {
		if dur > max {
			max = dur
		}
	}
	// умножаем на два для сравнения двух периодов
	timeRoundingMax := ap.UpdateTime.Add(-max * 2)
	candles, err := ap.repo.SelectMarketStateTimev2(timeRoundingMax)
	if err != nil {
		pricesLogger.Errorf("error SelectMarketStateTimev2: %v", err)
		return
	}

	if len(candles) == 0 {
		pricesLogger.Info("Нет свечей")
		return
	}

	// candles[0] -самый актуальный candle
	// сравнение времени candle с текущим временем
	if candles[0].Time.Sub(ap.UpdateTime) > 10*time.Minute {
		pricesLogger.Info("В базе данных отсутствуют данные за период, горячий перезапуск не удался")
		return
	}

	for _, candle := range candles {

		if !slices.Contains(ap.Pairs, candle.Pair) {
			continue
		}
		for period, periodValue := range ap.Periods {

			data := ap.ChangeDeltaDataset[candle.Pair][period]

			if !data.fill {

				// Большая погрешность дальше не заполняем  DatasetChangeDelta
				if len(data.dataset) > 0 && data.dataset[len(data.dataset)-1].Time.Sub(candle.Time) > 10*time.Minute {
					continue
				}

				item := ChangeDelta{
					Time:      candle.Time,
					Volume:    candle.Volume,
					VolumeBuy: candle.ActiveBuyVolume,
					VolumeAsk: candle.ActiveAskVolume,
					Trades:    float64(candle.AmountTrade),
					TradesBuy: float64(candle.AmountTradeBuy),
					TradesAsk: float64(candle.AmountTradeAsk),
				}

				data.dataset = append(data.dataset, item)
				ap.ChangeDeltaDataset[candle.Pair][period].dataset = data.dataset
				ap.ChangeDeltaDataset[candle.Pair][period].fill = len(data.dataset) == int(periodValue.Minutes()*2)
			}
		}
	}
}

func (ap *AssetsPrices) updateChangePrices() {
	ap.MarketsStatMu.RLock()
	defer ap.MarketsStatMu.RUnlock()
	ap.ChangePricesMu.Lock()
	defer ap.ChangePricesMu.Unlock()

	timeStart := time.Now()

	for _, pair := range ap.Pairs {
		for period, periodValue := range ap.Periods {

			data := ap.ChangePricesDataset[pair][period]

			item := DatasetChangePrices{
				Price: ap.MarketsStat[pair].Price,
				Time:  ap.MarketsStat[pair].Time,
			}

			if !data.Fill {
				if len(data.dataset) == int(periodValue.Minutes()-1) {
					data.dataset = append(data.dataset, item)
					ap.ChangePrices[pair][period].LastPrice = data.dataset[len(data.dataset)-1].Price
					ap.ChangePricesDataset[pair][period].Fill = true
					ap.ChangePricesDataset[pair][period].dataset = data.dataset
				} else {
					ap.ChangePricesDataset[pair][period].dataset = append(data.dataset, item)
				}
			} else {

				ap.ChangePrices[pair][period].ChangePercent = checkValuesDividing(ap.MarketsStat[pair].Price, ap.ChangePrices[pair][period].LastPrice)

				// Помещаем dataset в самое начало
				data.dataset = append([]DatasetChangePrices{item}, data.dataset...)

				// Удаляем последний элемент
				data.dataset = data.dataset[:len(data.dataset)-1]
				ap.ChangePrices[pair][period].LastPrice = data.dataset[len(data.dataset)-1].Price
				ap.ChangePricesDataset[pair][period].dataset = data.dataset
			}
		}
	}
	duration := time.Since(timeStart)

	pricesLogger.Debugf("Время выполнения UpdateChanges: %v ", duration)
}

func (ap *AssetsPrices) updateChangeDelta() error {

	timeStart := time.Now()

	candles, err := ap.repo.SelectMarketStateTimev2(ap.UpdateTime.Add(-1 * time.Minute))
	if err != nil {
		pricesLogger.Errorf("error SelectMarketStateTimev2: %v", err)
		return err
	}

	ap.ChangeDeltaMu.Lock()
	defer ap.ChangeDeltaMu.Unlock()

	for _, candle := range candles {

		if !slices.Contains(ap.Pairs, candle.Pair) {
			continue
		}

		for period, periodValue := range ap.PeriodsDelta {

			data := ap.ChangeDeltaDataset[candle.Pair][period]

			item := ChangeDelta{
				Time:      candle.Time,
				Volume:    candle.Volume,
				VolumeBuy: candle.ActiveBuyVolume,
				VolumeAsk: candle.ActiveAskVolume,
				Trades:    float64(candle.AmountTrade),
				TradesBuy: float64(candle.AmountTradeBuy),
				TradesAsk: float64(candle.AmountTradeAsk),
			}

			if !data.fill {
				data.dataset = append(data.dataset, item)
				ap.ChangeDeltaDataset[candle.Pair][period].dataset = data.dataset
				ap.ChangeDeltaDataset[candle.Pair][period].fill = len(data.dataset) == int(periodValue.Minutes()*2)
			} else {
				data.dataset = append([]ChangeDelta{item}, data.dataset...)
				data.dataset = data.dataset[:len(data.dataset)-1]

				ap.ChangeDeltaDataset[candle.Pair][period].dataset = data.dataset

				itemFirst := ChangeDelta{}
				itemLast := ChangeDelta{}

				for index, item := range data.dataset {

					if index < len(data.dataset)/2 {
						itemFirst.Volume += item.Volume
						itemFirst.VolumeBuy += item.VolumeBuy
						itemFirst.VolumeAsk += item.VolumeAsk
						itemFirst.Trades += item.Trades
						itemFirst.TradesBuy += item.TradesBuy
						itemFirst.TradesAsk += item.TradesAsk
					}

					if index >= len(data.dataset)/2 {
						itemLast.Volume += item.Volume
						itemLast.VolumeBuy += item.VolumeBuy
						itemLast.VolumeAsk += item.VolumeAsk
						itemLast.Trades += item.Trades
						itemLast.TradesBuy += item.TradesBuy
						itemLast.TradesAsk += item.TradesAsk
					}
				}

				ap.ChangeDelta[candle.Pair][period].Volume = checkValuesDividing(itemFirst.Volume, itemLast.Volume)
				ap.ChangeDelta[candle.Pair][period].VolumeBuy = checkValuesDividing(itemFirst.VolumeBuy, itemLast.VolumeBuy)
				ap.ChangeDelta[candle.Pair][period].VolumeAsk = checkValuesDividing(itemFirst.VolumeAsk, itemLast.VolumeAsk)

				ap.ChangeDelta[candle.Pair][period].Trades = checkValuesDividing(float64(itemFirst.Trades), float64(itemLast.Trades))
				ap.ChangeDelta[candle.Pair][period].TradesBuy = checkValuesDividing(float64(itemFirst.TradesBuy), float64(itemLast.TradesBuy))
				ap.ChangeDelta[candle.Pair][period].TradesAsk = checkValuesDividing(float64(itemFirst.TradesAsk), float64(itemLast.TradesAsk))

			}
		}
	}

	duration := time.Since(timeStart)
	pricesLogger.Debugf("Время выполнения updateChangeDelta: %v ", duration)

	return nil
}

func (ap *AssetsPrices) GetAllChPrice() map[string]map[string]ChangePrices {
	ap.ChangePricesMu.RLock()
	defer ap.ChangePricesMu.RUnlock()

	result := make(map[string]map[string]ChangePrices, len(ap.ChangePrices))

	for k1, innerMap := range ap.ChangePrices {
		innerCopy := make(map[string]ChangePrices, len(innerMap))
		for k2, v := range innerMap {
			if v != nil {
				innerCopy[k2] = *v
			} else {
				innerCopy[k2] = ChangePrices{}
			}
		}
		result[k1] = innerCopy
	}

	return result
}

func (ap *AssetsPrices) GetAllChDelta() map[string]map[string]ChangeDelta {
	ap.ChangeDeltaMu.RLock()
	defer ap.ChangeDeltaMu.RUnlock()

	result := make(map[string]map[string]ChangeDelta, len(ap.ChangeDelta))

	for outerKey, innerMap := range ap.ChangeDelta {
		innerCopy := make(map[string]ChangeDelta, len(innerMap))

		for innerKey, deltaPtr := range innerMap {
			if deltaPtr != nil {
				innerCopy[innerKey] = *deltaPtr
			} else {
				innerCopy[innerKey] = ChangeDelta{}
			}
		}

		result[outerKey] = innerCopy
	}

	return result
}

func (ap *AssetsPrices) GetAllMarketsStat() map[string]exModel.MarketsStat {
	ap.MarketsStatMu.RLock()
	defer ap.MarketsStatMu.RUnlock()

	result := make(map[string]exModel.MarketsStat, len(ap.MarketsStat))

	for key, statPtr := range ap.MarketsStat {
		if statPtr != nil {
			result[key] = *statPtr
		} else {
			result[key] = exModel.MarketsStat{}
		}
	}

	return result
}

func (ap *AssetsPrices) GetMarketsStatForPair(pair string) (exModel.MarketsStat, error) {
	ap.MarketsStatMu.RLock()
	defer ap.MarketsStatMu.RUnlock()

	if statPtr := ap.MarketsStat[pair]; statPtr == nil {
		return exModel.MarketsStat{}, fmt.Errorf("market stat for pair %s not found", pair)
	} else {
		return *statPtr, nil
	}
}

func (ap *AssetsPrices) GetChPriceForPair(pair string) (map[string]ChangePrices, error) {
	ap.ChangePricesMu.RLock()
	defer ap.ChangePricesMu.RUnlock()

	if innerMap, exists := ap.ChangePrices[pair]; exists {
		result := make(map[string]ChangePrices, len(innerMap))
		for k, v := range innerMap {
			if v != nil {
				result[k] = *v
			} else {
				result[k] = ChangePrices{}
			}
		}
		return result, nil
	}

	return nil, fmt.Errorf("pair %s not found", pair)
}

func (ap *AssetsPrices) GetChangeDeltaForPair(pair string) (map[string]ChangeDelta, error) {
	ap.ChangeDeltaMu.RLock()
	defer ap.ChangeDeltaMu.RUnlock()

	if innerMap, exists := ap.ChangeDelta[pair]; exists {
		result := make(map[string]ChangeDelta, len(innerMap))
		for k, v := range innerMap {
			if v != nil {
				result[k] = *v
			} else {
				result[k] = ChangeDelta{}
			}
		}
		return result, nil
	}

	return nil, fmt.Errorf("pair %s not found in ChangeDelta data", pair)
}

func (ap *AssetsPrices) GetDeltaPeriod(pair, period string) ([]model.ChangeDeltaForCandle, error) {

	timeStart := time.Now()

	changeDelta, err := ap.repo.SelectDeltaPeriod(pair, period)
	if err != nil {
		return nil, err
	}

	clearChangeDelta := []model.ChangeDeltaForCandle{}

	if len(changeDelta) > 0 {
		clearChangeDelta = append(clearChangeDelta, changeDelta[0])

		// Если есть пропуски по времени , то заполняем их
		for i := 1; i < len(changeDelta); i++ {
			prevTime := changeDelta[i-1].Time
			currTime := changeDelta[i].Time

			for currTime.Sub(prevTime) > ap.PeriodsDelta[period] {
				buffer := clearChangeDelta[len(clearChangeDelta)-1]
				prevTime = prevTime.Add(ap.PeriodsDelta[period])
				buffer.Time = buffer.Time.Add(ap.PeriodsDelta[period])
				clearChangeDelta = append(clearChangeDelta, buffer)
			}
			clearChangeDelta = append(clearChangeDelta, changeDelta[i])
		}
		duration := time.Since(timeStart)
		pricesLogger.Debugf("Время выполнения GetDeltaPeriod: %v ", duration)
	}

	return clearChangeDelta, nil
}

// +inf/-inf/nan
func checkValuesDividing(numerator, denominator float64) float64 {
	if numerator == 0.0 || denominator == 0.0 {
		return 0
	}
	return float64(numerator/denominator)*100 - 100
}
