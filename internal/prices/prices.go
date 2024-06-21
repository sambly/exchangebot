package prices

import (
	"database/sql"
	"fmt"
	"main/internal/database"
	"main/internal/logging"
	"main/internal/model"
	"main/internal/notification"
	"slices"
	"sync"
	"time"
)

type ChangePrices struct {
	LastPrice           float64
	СhangePercent       float64
	DatasetChangePrices []DatasetChangePrices
	DatasetFil          bool
}
type DatasetChangePrices struct {
	Price float64
	Time  time.Time
}

type AsetsPrices struct {
	database     *sql.DB
	Notification *notification.Notification

	Pairs          []string
	Periods        map[string]time.Duration
	PeriodsDelta   map[string]time.Duration
	WeightProcents map[string]float64

	UpdateTime time.Time

	// Актуальные данные для каждой пары. Price, 24ch, Volume
	MarketsStatMu sync.RWMutex
	MarketsStat   map[string]*model.MarketsStat

	ChangePricesMu sync.RWMutex
	ChangePrices   map[string]map[string]*ChangePrices

	FormingChangeDelta map[string]map[string]*ChangeDeltaForming
	DeltaFastMu        sync.RWMutex
	DeltaFast          map[string]map[string]*model.DeltaFast
}

type ChangeDeltaForming struct {
	ChangeDeltaMinute []model.ChangeDelta
	Fill              bool
}

func NewAssetsPrices(pairs []string, periodsChange, periodsDelta map[string]time.Duration, weightProcents map[string]float64, db *sql.DB, notification *notification.Notification) *AsetsPrices {
	asetsPrices := &AsetsPrices{
		Pairs:              pairs,
		Periods:            periodsChange,
		PeriodsDelta:       periodsDelta,
		WeightProcents:     weightProcents,
		MarketsStat:        make(map[string]*model.MarketsStat),
		ChangePrices:       make(map[string]map[string]*ChangePrices),
		FormingChangeDelta: make(map[string]map[string]*ChangeDeltaForming),
		DeltaFast:          make(map[string]map[string]*model.DeltaFast),
		database:           db,
		Notification:       notification,
	}

	for _, pair := range pairs {
		asetsPrices.MarketsStat[pair] = &model.MarketsStat{Pair: pair}

		asetsPrices.ChangePrices[pair] = map[string]*ChangePrices{}

		asetsPrices.FormingChangeDelta[pair] = map[string]*ChangeDeltaForming{}
		asetsPrices.DeltaFast[pair] = map[string]*model.DeltaFast{}

		for period := range periodsChange {
			asetsPrices.ChangePrices[pair][period] = &ChangePrices{}
		}
		for period := range periodsDelta {
			asetsPrices.FormingChangeDelta[pair][period] = &ChangeDeltaForming{}
			asetsPrices.DeltaFast[pair][period] = &model.DeltaFast{}
		}
	}
	return asetsPrices
}

func (ap *AsetsPrices) OnMarket(ms model.MarketsStat) {

	ap.MarketsStatMu.Lock()
	defer ap.MarketsStatMu.Unlock()

	if _, ok := ap.MarketsStat[ms.Pair]; !ok {
		ap.MarketsStat[ms.Pair] = &model.MarketsStat{}
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
			ap.UpdateChanges()
		}()

		go func() {
			// Ожидание пока данные запишутся в базу данных, потом мы считаем новые значения
			time.Sleep(10 * time.Second)
			if err := ap.UpdateDelta(); err != nil {
				logging.MyLogger.ErrorOut(fmt.Errorf("error in updateDelta: %v", err))
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
		logging.MyLogger.ErrorOut(fmt.Errorf("error SelectMarketStateTimev2: %v", err))
		return
	}

	if len(candles) == 0 {
		logging.MyLogger.InfoLog.Println("Нет candles")
		return
	}

	// candles[0] -самый актуальный candle
	// сравнение времени candle с текущим временем
	if candles[0].Time.Sub(ap.UpdateTime) > 10*time.Minute {
		logging.MyLogger.InfoLog.Println("В базе данных отсутствуют данные за период, горячий перезапуск не удался")
		return
	}

	for _, candle := range candles {

		if !slices.Contains(ap.Pairs, candle.Pair) {
			continue
		}

		for period, periodValue := range ap.Periods {

			data := ap.ChangePrices[candle.Pair][period]
			if !data.DatasetFil {

				dataset := DatasetChangePrices{
					Price: candle.Close,
					Time:  candle.Time,
				}

				if len(data.DatasetChangePrices) == int(periodValue.Minutes()-1) {

					data.DatasetChangePrices = append(data.DatasetChangePrices, dataset)
					data.LastPrice = data.DatasetChangePrices[len(data.DatasetChangePrices)-1].Price
					data.DatasetFil = true
				} else {

					// Большая погрешность дальше не заполняем  forming.DatasetCandle
					if len(data.DatasetChangePrices) > 0 && data.DatasetChangePrices[len(data.DatasetChangePrices)-1].Time.Sub(candle.Time) > 10*time.Minute {
						continue
					}
					data.DatasetChangePrices = append(data.DatasetChangePrices, dataset)
				}
			}
		}
	}
}

func (ap *AsetsPrices) InitDelta() {

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
		logging.MyLogger.ErrorOut(fmt.Errorf("error SelectMarketStateTimev2: %v", err))
		return
	}

	if len(candles) == 0 {
		logging.MyLogger.InfoLog.Println("Нет candles")
		return
	}

	// candles[0] -самый актуальный candle
	// сравнение времени candle с текущим временем
	if candles[0].Time.Sub(ap.UpdateTime) > 10*time.Minute {
		logging.MyLogger.InfoLog.Println("В базе данных отсутствуют данные за период, горячий перезапуск не удался")
		return
	}

	for _, candle := range candles {

		if !slices.Contains(ap.Pairs, candle.Pair) {
			continue
		}
		for period, periodValue := range ap.Periods {

			candleMinute := ap.FormingChangeDelta[candle.Pair][period].ChangeDeltaMinute
			fill := ap.FormingChangeDelta[candle.Pair][period].Fill

			if !fill {

				if len(candleMinute) > 0 {
					// Большая погрешность дальше не заполняем  forming.DatasetCandle
					if candleMinute[len(candleMinute)-1].Time.Sub(candle.Time) > 10*time.Minute {
						//logging.MyLogger.InfoLog.Println("Большая погрешность, при формировании candles (continue)")
						continue
					}
				}

				chDelta := model.ChangeDelta{
					Time:      candle.Time,
					Volume:    candle.Volume,
					VolumeBuy: candle.ActiveBuyVolume,
					VolumeAsk: candle.ActiveAskVolume,
					Trades:    candle.AmountTrade,
					TradesBuy: candle.AmountTradeBuy,
					TradesAsk: candle.AmountTradeAsk,
				}

				candleMinute = append(candleMinute, chDelta)
				ap.FormingChangeDelta[candle.Pair][period].ChangeDeltaMinute = candleMinute
				ap.FormingChangeDelta[candle.Pair][period].Fill = len(candleMinute) == int(periodValue.Minutes()*2)
			}
		}
	}
}

func (ap *AsetsPrices) UpdateChanges() {
	ap.MarketsStatMu.RLock()
	defer ap.MarketsStatMu.RUnlock()
	ap.ChangePricesMu.Lock()
	defer ap.ChangePricesMu.Unlock()

	timeStart := time.Now()

	for _, pair := range ap.Pairs {
		for period, periodValue := range ap.Periods {

			data := ap.ChangePrices[pair][period]

			dataset := DatasetChangePrices{
				Price: ap.MarketsStat[pair].Price,
				Time:  ap.MarketsStat[pair].Time,
			}

			if !data.DatasetFil {
				if len(data.DatasetChangePrices) == int(periodValue.Minutes()-1) {

					data.DatasetChangePrices = append(data.DatasetChangePrices, dataset)
					data.LastPrice = data.DatasetChangePrices[len(data.DatasetChangePrices)-1].Price
					data.DatasetFil = true
				} else {
					data.DatasetChangePrices = append(data.DatasetChangePrices, dataset)
				}
			} else {

				data.СhangePercent = check_values_dividing(ap.MarketsStat[pair].Price, data.LastPrice)

				// Отправка сообщения об изменении цены
				if data.СhangePercent >= ap.WeightProcents[period] {
					ap.Notification.NotificationWeightPercent(pair, period, data.СhangePercent)
				}

				// Помещаем dataset в самое начало
				data.DatasetChangePrices = append([]DatasetChangePrices{dataset}, data.DatasetChangePrices...)

				// Удаляем последний элемент
				data.DatasetChangePrices = data.DatasetChangePrices[:len(data.DatasetChangePrices)-1]
				data.LastPrice = data.DatasetChangePrices[len(data.DatasetChangePrices)-1].Price
			}
		}
	}
	duration := time.Since(timeStart)
	logging.MyLogger.InfoLog.Println("Время выполнения UpdateChanges: ", duration)
}

func (ap *AsetsPrices) UpdateDelta() error {

	timeStart := time.Now()

	// TODO Надо сделать проверку на то , что дейсвтительно мы получили
	// более новый candle с базы данных

	candles, err := database.SelectMarketStateTimev2(ap.database, ap.UpdateTime.Add(-1*time.Minute))
	if err != nil {
		logging.MyLogger.ErrorOut(fmt.Errorf("error SelectMarketStateTimev2: %v", err))
		return err
	}

	ap.DeltaFastMu.Lock()
	defer ap.DeltaFastMu.Unlock()

	for _, candle := range candles {

		if !slices.Contains(ap.Pairs, candle.Pair) {
			continue
		}

		for period, periodValue := range ap.PeriodsDelta {

			candleMinute := ap.FormingChangeDelta[candle.Pair][period].ChangeDeltaMinute
			deltaFast := ap.DeltaFast[candle.Pair][period]
			fill := ap.FormingChangeDelta[candle.Pair][period].Fill

			chDelta := model.ChangeDelta{
				Time:      candle.Time,
				Volume:    candle.Volume,
				VolumeBuy: candle.ActiveBuyVolume,
				VolumeAsk: candle.ActiveAskVolume,
				Trades:    candle.AmountTrade,
				TradesBuy: candle.AmountTradeBuy,
				TradesAsk: candle.AmountTradeAsk,
			}

			if !fill {
				candleMinute = append(candleMinute, chDelta)
				ap.FormingChangeDelta[candle.Pair][period].ChangeDeltaMinute = candleMinute
				ap.FormingChangeDelta[candle.Pair][period].Fill = len(candleMinute) == int(periodValue.Minutes()*2)
			} else {
				candleMinute = append([]model.ChangeDelta{chDelta}, candleMinute...)
				candleMinute = candleMinute[:len(candleMinute)-1]

				ap.FormingChangeDelta[candle.Pair][period].ChangeDeltaMinute = candleMinute

				chDeltaFirst := model.ChangeDelta{}
				chDeltaLast := model.ChangeDelta{}

				for index, item := range candleMinute {

					if index < len(candleMinute)/2 {
						chDeltaFirst.Time = item.Time
						chDeltaFirst.Volume += item.Volume
						chDeltaFirst.VolumeBuy += item.VolumeBuy
						chDeltaFirst.VolumeAsk += item.VolumeAsk
						chDeltaFirst.Trades += item.Trades
						chDeltaFirst.TradesBuy += item.TradesBuy
						chDeltaFirst.TradesAsk += item.TradesAsk
					}

					if index >= len(candleMinute)/2 {
						chDeltaLast.Time = item.Time
						chDeltaLast.Volume += item.Volume
						chDeltaLast.VolumeBuy += item.VolumeBuy
						chDeltaLast.VolumeAsk += item.VolumeAsk
						chDeltaLast.Trades += item.Trades
						chDeltaLast.TradesBuy += item.TradesBuy
						chDeltaLast.TradesAsk += item.TradesAsk
					}
				}

				deltaFast.Volume = check_values_dividing(chDeltaFirst.Volume, chDeltaLast.Volume)
				deltaFast.VolumeBuy = check_values_dividing(chDeltaFirst.VolumeBuy, chDeltaLast.VolumeBuy)
				deltaFast.VolumeAsk = check_values_dividing(chDeltaFirst.VolumeAsk, chDeltaLast.VolumeAsk)

				deltaFast.Trades = check_values_dividing(float64(chDeltaFirst.Trades), float64(chDeltaLast.Trades))
				deltaFast.TradesBuy = check_values_dividing(float64(chDeltaFirst.TradesBuy), float64(chDeltaLast.TradesBuy))
				deltaFast.TradesAsk = check_values_dividing(float64(chDeltaFirst.TradesAsk), float64(chDeltaLast.TradesAsk))

			}
		}
	}

	duration := time.Since(timeStart)
	logging.MyLogger.InfoLog.Println("Время выполнения UpdateDelta: ", duration)
	return nil
}

func (ap *AsetsPrices) GetDeltaPeriod(pair, period string) ([]model.ChangeDelta, error) {

	timeStart := time.Now()

	changeDelta, err := database.SelectDeltaPeriod(ap.database, pair, period)
	if err != nil {
		return nil, err
	}

	clearChangeDelta := []model.ChangeDelta{}

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
		logging.MyLogger.InfoLog.Println("Время выполнения GetDeltaPeriod: ", duration)
	}

	return clearChangeDelta, nil
}

// +inf/-inf/nan
func check_values_dividing(numerator, denominator float64) float64 {
	if numerator == 0.0 || denominator == 0.0 {
		return 0
	}
	return float64(numerator/denominator)*100 - 100
}
