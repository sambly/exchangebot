package prices

import (
	"slices"
	"sync"
	"time"

	"github.com/sambly/exchangebot/internal/database"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"gorm.io/gorm"

	exModel "github.com/sambly/exchangeService/pkg/model"
)

type ChangePrices struct {
	LastPrice     float64
	СhangePercent float64
}

type ChangePricesDataset struct {
	dataset []DatasetChangePrices
	fill    bool
}

type DatasetChangePrices struct {
	Price float64
	Time  time.Time
}
type ChangeDelta struct {
	Time      time.Time
	Volume    float64
	VolumeBuy float64
	VolumeAsk float64
	Trades    float64
	TradesBuy float64
	TradesAsk float64
}

type ChangeDeltaDataset struct {
	dataset []ChangeDelta
	fill    bool
}

type AsetsPrices struct {
	database     *gorm.DB
	Notification *notification.Notification

	Pairs          []string
	Periods        map[string]time.Duration
	PeriodsDelta   map[string]time.Duration
	WeightProcents map[string]float64

	UpdateTime time.Time

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

func NewAssetsPrices(pairs []string, periodsChange, periodsDelta map[string]time.Duration, weightProcents map[string]float64, db *gorm.DB, notification *notification.Notification) *AsetsPrices {
	asetsPrices := &AsetsPrices{
		Pairs:          pairs,
		Periods:        periodsChange,
		PeriodsDelta:   periodsDelta,
		WeightProcents: weightProcents,

		MarketsStat: make(map[string]*exModel.MarketsStat),

		ChangePrices:        make(map[string]map[string]*ChangePrices),
		ChangePricesDataset: make(map[string]map[string]*ChangePricesDataset),

		ChangeDelta:        make(map[string]map[string]*ChangeDelta),
		ChangeDeltaDataset: make(map[string]map[string]*ChangeDeltaDataset),

		database:     db,
		Notification: notification,
	}

	for _, pair := range pairs {
		asetsPrices.MarketsStat[pair] = &exModel.MarketsStat{Pair: pair}

		asetsPrices.ChangePrices[pair] = map[string]*ChangePrices{}
		asetsPrices.ChangePricesDataset[pair] = map[string]*ChangePricesDataset{}
		asetsPrices.ChangeDelta[pair] = map[string]*ChangeDelta{}
		asetsPrices.ChangeDeltaDataset[pair] = map[string]*ChangeDeltaDataset{}

		for period := range periodsChange {
			asetsPrices.ChangePrices[pair][period] = &ChangePrices{}
			asetsPrices.ChangePricesDataset[pair][period] = &ChangePricesDataset{}
		}
		for period := range periodsDelta {
			asetsPrices.ChangeDelta[pair][period] = &ChangeDelta{}
			asetsPrices.ChangeDeltaDataset[pair][period] = &ChangeDeltaDataset{}
		}
	}
	return asetsPrices
}

func (ap *AsetsPrices) OnMarket(ms exModel.MarketsStat) {

	ap.MarketsStatMu.Lock()
	defer ap.MarketsStatMu.Unlock()

	if _, ok := ap.MarketsStat[ms.Pair]; !ok {
		ap.MarketsStat[ms.Pair] = &exModel.MarketsStat{}
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
			ap.UpdateChangePrices()
		}()

		go func() {
			// Ожидание пока данные запишутся в базу данных, потом мы считаем новые значения
			time.Sleep(10 * time.Second)
			if err := ap.UpdateChangeDelta(); err != nil {
				pricesLogger.Errorf("error in updateDelta: %v", err)
			}
		}()
	}
}

func (ap *AsetsPrices) InitChangePrices() {

	// Максимальный заданный период для запроса в бд
	var max time.Duration
	for _, dur := range ap.Periods {
		if dur > max {
			max = dur
		}
	}

	// Интервал времени текущее время - время макс. периода
	timeRoundingMax := ap.UpdateTime.Add(-max)

	candles, err := database.SelectMarketStateTimev2(ap.database, timeRoundingMax)
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

			if !data.fill {

				item := DatasetChangePrices{
					Price: candle.Close,
					Time:  candle.Time,
				}

				if len(data.dataset) == int(periodValue.Minutes()-1) {

					data.dataset = append(data.dataset, item)
					ap.ChangePrices[candle.Pair][period].LastPrice = data.dataset[len(data.dataset)-1].Price
					ap.ChangePricesDataset[candle.Pair][period].fill = true
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

func (ap *AsetsPrices) InitChangeDelta() {

	// Максимальный заданный период для запроса в бд
	var max time.Duration
	for _, dur := range ap.Periods {
		if dur > max {
			max = dur
		}
	}
	// умножаем на два для сравнения двух периодов
	timeRoundingMax := ap.UpdateTime.Add(-max * 2)
	candles, err := database.SelectMarketStateTimev2(ap.database, timeRoundingMax)
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

func (ap *AsetsPrices) UpdateChangePrices() {
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

			if !data.fill {
				if len(data.dataset) == int(periodValue.Minutes()-1) {
					data.dataset = append(data.dataset, item)
					ap.ChangePrices[pair][period].LastPrice = data.dataset[len(data.dataset)-1].Price
					ap.ChangePricesDataset[pair][period].fill = true
					ap.ChangePricesDataset[pair][period].dataset = data.dataset
				} else {
					ap.ChangePricesDataset[pair][period].dataset = append(data.dataset, item)
				}
			} else {

				ap.ChangePrices[pair][period].СhangePercent = checkValuesDividing(ap.MarketsStat[pair].Price, ap.ChangePrices[pair][period].LastPrice)

				// Отправка сообщения об изменении цены
				if ap.ChangePrices[pair][period].СhangePercent >= ap.WeightProcents[period] {
					ap.Notification.NotificationWeightPercent(pair, period, ap.ChangePrices[pair][period].СhangePercent)
				}

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

	pricesLogger.Infof("Время выполнения UpdateChanges: %v ", duration)
}

func (ap *AsetsPrices) UpdateChangeDelta() error {

	timeStart := time.Now()

	candles, err := database.SelectMarketStateTimev2(ap.database, ap.UpdateTime.Add(-1*time.Minute))
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
						itemFirst.Time = item.Time
						itemFirst.Volume += item.Volume
						itemFirst.VolumeBuy += item.VolumeBuy
						itemFirst.VolumeAsk += item.VolumeAsk
						itemFirst.Trades += item.Trades
						itemFirst.TradesBuy += item.TradesBuy
						itemFirst.TradesAsk += item.TradesAsk
					}

					if index >= len(data.dataset)/2 {
						itemLast.Time = item.Time
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
	pricesLogger.Infof("Время выполнения UpdateChangeDelta: %v ", duration)

	return nil
}

func (ap *AsetsPrices) GetAllChPrice() map[string]map[string]*ChangePrices {
	ap.ChangePricesMu.RLock()
	defer ap.ChangePricesMu.RUnlock()

	return ap.ChangePrices
}

func (ap *AsetsPrices) GetAllChDelta() map[string]map[string]*ChangeDelta {
	ap.ChangeDeltaMu.RLock()
	defer ap.ChangeDeltaMu.RUnlock()

	return ap.ChangeDelta
}

func (ap *AsetsPrices) GetAllMarketsStat() map[string]*exModel.MarketsStat {
	ap.MarketsStatMu.RLock()
	defer ap.MarketsStatMu.RUnlock()

	return ap.MarketsStat
}

func (ap *AsetsPrices) GetMarketsStatForPair(pair string) *exModel.MarketsStat {
	ap.MarketsStatMu.RLock()
	defer ap.MarketsStatMu.RUnlock()

	return ap.MarketsStat[pair]
}

func (ap *AsetsPrices) GetDeltaPeriod(pair, period string) ([]model.ChangeDeltaForCandle, error) {

	timeStart := time.Now()

	changeDelta, err := database.SelectDeltaPeriod(ap.database, pair, period)
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
		pricesLogger.Infof("Время выполнения GetDeltaPeriod: %v ", duration)
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
