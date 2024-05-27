package prices

import (
	"database/sql"
	"fmt"
	"log"
	"main/internal/database"
	"main/internal/model"
	"main/internal/notification"
	"slices"
	"sync"
	"time"
)

type AsetsPrices struct {
	database     *sql.DB
	Notification *notification.Notification

	Pairs          []string
	Periods        map[string]time.Duration
	PeriodsDelta   map[string]time.Duration
	WeightProcents map[string]float64

	UpdateTime time.Time

	MarketsStatMu sync.RWMutex
	MarketsStat   map[string]*model.MarketsStat

	FormingChangePrices map[string]map[string]*ChangeDataForming
	ChangePricesMu      sync.RWMutex
	ChangePrices        map[string]map[string]*model.ChangeData

	FormingChangeDelta map[string]map[string]*ChangeDeltaForming
	DeltaFastMu        sync.RWMutex
	DeltaFast          map[string]map[string]*model.DeltaFast
}

type ChangeDeltaForming struct {
	ChangeDeltaMinute []model.ChangeDelta
	Fill              bool
}

type ChangeDataForming struct {
	DatasetCandle []model.DatasetCandle
	Fill          bool
}

func NewAssetsPrices(pairs []string, periodsChange, periodsDelta map[string]time.Duration, weightProcents map[string]float64, db *sql.DB, notification *notification.Notification) *AsetsPrices {
	asetsPrices := &AsetsPrices{
		Pairs:               pairs,
		Periods:             periodsChange,
		PeriodsDelta:        periodsDelta,
		WeightProcents:      weightProcents,
		MarketsStat:         make(map[string]*model.MarketsStat),
		FormingChangePrices: make(map[string]map[string]*ChangeDataForming),
		ChangePrices:        make(map[string]map[string]*model.ChangeData),
		FormingChangeDelta:  make(map[string]map[string]*ChangeDeltaForming),
		DeltaFast:           make(map[string]map[string]*model.DeltaFast),
		database:            db,
		Notification:        notification,
	}
	for _, pair := range pairs {
		asetsPrices.MarketsStat[pair] = &model.MarketsStat{Pair: pair}
	}
	for _, pair := range pairs {
		if _, ok := asetsPrices.ChangePrices[pair]; !ok {

			asetsPrices.FormingChangePrices[pair] = map[string]*ChangeDataForming{}
			asetsPrices.ChangePrices[pair] = map[string]*model.ChangeData{}

			asetsPrices.FormingChangeDelta[pair] = map[string]*ChangeDeltaForming{}
			asetsPrices.DeltaFast[pair] = map[string]*model.DeltaFast{}
		}
		for period, _ := range periodsChange {
			asetsPrices.FormingChangePrices[pair][period] = &ChangeDataForming{}
			asetsPrices.ChangePrices[pair][period] = &model.ChangeData{}
		}
		for period, _ := range periodsDelta {
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
				log.Println("Error in UpdateDelta:", err)
			}
		}()
	}
}

func (ap *AsetsPrices) InitChangePrices() {

	// Определить масимальное время из периода для запроса в бд
	var max time.Duration
	for _, dur := range ap.Periods {
		if dur > max {
			max = dur
		}
	}

	timeRoundingMax := ap.UpdateTime.Add(-max)
	candles, err := database.SelectMarketStateTimev2(ap.database, timeRoundingMax)
	if err != nil {
		// TODO
		fmt.Println("ERROR DBBBBBB InitChangePrices")
	}

	if len(candles) == 0 {
		fmt.Println("Нет candles")
		return
	}

	// Сделаем небольшую погрешность , для возможности горячего перезапуска приложения
	if candles[0].Time.Sub(ap.UpdateTime) > 10*time.Minute {
		fmt.Println("Большая погрешность №1")
		return
	}

	for _, candle := range candles {
		if idx := slices.Index(ap.Pairs, candle.Pair); idx >= 0 {
			for period, periodValue := range ap.Periods {

				forming := ap.FormingChangePrices[candle.Pair][period]
				change := ap.ChangePrices[candle.Pair][period]
				if !forming.Fill {
					if len(forming.DatasetCandle) == int(periodValue.Minutes()-1) {
						forming.DatasetCandle = append(forming.DatasetCandle, model.DatasetCandle{Price: candle.Close, Volume: candle.Volume, Time: candle.Time})
						change.LastPrice = forming.DatasetCandle[len(forming.DatasetCandle)-1].Price
						forming.Fill = true
					} else {

						if len(forming.DatasetCandle) > 0 {
							// Большая погрешность дальше не заполняем  forming.DatasetCandle
							if forming.DatasetCandle[len(forming.DatasetCandle)-1].Time.Sub(candle.Time) > 10*time.Minute {
								continue
							}
						}
						forming.DatasetCandle = append(forming.DatasetCandle, model.DatasetCandle{Price: candle.Close, Volume: candle.Volume, Time: candle.Time})
					}
				}
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

			forming := ap.FormingChangePrices[pair][period]
			changeData := ap.ChangePrices[pair][period]

			if !forming.Fill {
				if len(forming.DatasetCandle) == int(periodValue.Minutes()-1) {
					forming.DatasetCandle = append(forming.DatasetCandle, model.DatasetCandle{Price: ap.MarketsStat[pair].Price, Volume: 0, Time: ap.MarketsStat[pair].Time})
					changeData.LastPrice = forming.DatasetCandle[len(forming.DatasetCandle)-1].Price
					forming.Fill = true
				} else {
					forming.DatasetCandle = append(forming.DatasetCandle, model.DatasetCandle{Price: ap.MarketsStat[pair].Price, Volume: 0, Time: ap.MarketsStat[pair].Time})
				}
			} else {

				changeData.СhangePercent = check_values_dividing(ap.MarketsStat[pair].Price, changeData.LastPrice)

				if changeData.СhangePercent >= ap.WeightProcents[period] {
					ap.Notification.NotificationWeightPercent(pair, period, changeData.СhangePercent)
				}
				forming.DatasetCandle = append([]model.DatasetCandle{{Price: ap.MarketsStat[pair].Price, Volume: 0, Time: ap.MarketsStat[pair].Time}}, forming.DatasetCandle...)
				// Удаляем последний элемент
				forming.DatasetCandle = forming.DatasetCandle[:len(forming.DatasetCandle)-1]
				changeData.LastPrice = forming.DatasetCandle[len(forming.DatasetCandle)-1].Price

			}

		}

	}
	duration := time.Since(timeStart)

	log.Println("Время выполнения UpdateChanges: ", duration)
}

func (ap *AsetsPrices) InitDelta() {
	// Определить масимальное время из периода для запроса в бд
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
		// TODO
		fmt.Println("ERROR DBBBBBB InitDelta")
	}

	if len(candles) == 0 {
		fmt.Println("Нет candles")
		return
	}

	// Сделаем небольшую погрешность , для возможности горячего перезапуска приложения
	if candles[0].Time.Sub(ap.UpdateTime) > 10*time.Minute {
		fmt.Println("Большая погрешность №1")
		return
	}

	for _, candle := range candles {

		if idx := slices.Index(ap.Pairs, candle.Pair); idx >= 0 {

			for period, periodValue := range ap.Periods {

				candleMinute := ap.FormingChangeDelta[candle.Pair][period].ChangeDeltaMinute
				fill := ap.FormingChangeDelta[candle.Pair][period].Fill

				if !fill {

					if len(candleMinute) > 0 {
						// Большая погрешность дальше не заполняем  forming.DatasetCandle
						if candleMinute[len(candleMinute)-1].Time.Sub(candle.Time) > 10*time.Minute {
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
}

func (ap *AsetsPrices) UpdateDelta() error {

	timeStart := time.Now()

	// TODO Надо сделать проверку на то , что дейсвтительно мы получили
	// более новый candle с базы данных

	candles, err := database.SelectMarketStateTimev2(ap.database, ap.UpdateTime.Add(-1*time.Minute))
	fmt.Println(len(candles))
	if err != nil {
		fmt.Println("Err")
		return err
	}

	ap.DeltaFastMu.Lock()
	defer ap.DeltaFastMu.Unlock()

	for _, candle := range candles {

		if idx := slices.Index(ap.Pairs, candle.Pair); idx >= 0 {

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
	}

	duration := time.Since(timeStart)

	log.Println("Время выполнения UpdateDelta: ", duration)

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

		log.Println("Время выполнения GetDeltaPeriod: ", duration)
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

// Проверка на кратность времени
func isTimeMultipleOfInterval(t time.Time, interval time.Duration) bool {
	startTime := time.Unix(0, 0) // Начальное время (начало Unix эпохи)
	return t.Sub(startTime)%interval == 0
}
