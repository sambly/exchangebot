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
	SelectMarketStateTimev2(timeRounding time.Time) ([]exModel.Candle, error)
	SelectDeltaPeriod(pair string, period string) ([]model.ChangeDeltaForCandle, error)
	SelectCandlesFromPeriod(period string, from time.Time) ([]exModel.Candle, error)
}

type ChangePrices struct {
	LastPrice     float64
	ChangePercent float64
}

// ChangePricesDataset - окно цен за период (по одному значению в минуту)
type ChangePricesDataset struct {
	window[DatasetChangePrices]
	Fill bool
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

// ChangeDeltaDataset - окно объёмов и трейдов за 2 x период: свежая половина
// сравнивается со старой.
type ChangeDeltaDataset struct {
	window[ChangeDelta]
	fill bool
}

type AssetsPrices struct {
	repo Repository

	Pairs        []string
	Periods      map[string]time.Duration
	PeriodsDelta map[string]time.Duration

	UpdateTime time.Time

	// Каждый потребитель обновлений получает собственный канал через Subscribe().
	// Общий канал на всех не подходит: тик достался бы только одной стратегии.
	subscribersMu sync.Mutex
	subscribers   []chan struct{}

	// Актуальные данные для каждой пары. Price, 24ch, Volume
	MarketsStatMu sync.RWMutex
	MarketsStat   map[string]*exModel.MarketsStat

	ChangePricesMu      sync.RWMutex
	ChangePrices        map[string]map[string]*ChangePrices
	ChangePricesDataset map[string]map[string]*ChangePricesDataset

	ChangeDeltaMu      sync.RWMutex
	ChangeDelta        map[string]map[string]*ChangeDelta
	ChangeDeltaDataset map[string]map[string]*ChangeDeltaDataset

	// volatility - грубая история изменения цены для GetVolatility, отдельно
	// от ChangePrices: та же математика (медиана+MAD), но своя, не завязанная
	// на детектор аномалий история (см. volatility.go).
	volatility map[string]map[string]*volatilityState

	// activity - история buy-side активности для GetBuyActivityZScore, тем же
	// принципом изоляции, что и volatility (см. activity.go).
	activity map[string]map[string]*activityState
}

var pricesLogger = logger.AddFieldsEmpty()

func NewAssetsPrices(pairs []string, periodsChange, periodsDelta map[string]time.Duration, repo Repository) (*AssetsPrices, error) {
	assetsPrices := &AssetsPrices{
		Pairs:        pairs,
		Periods:      periodsChange,
		PeriodsDelta: periodsDelta,

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

		// Размер окна цен - одно значение в минуту за период.
		// Окно дельт вдвое больше: свежая половина сравнивается со старой.
		for period, duration := range periodsChange {
			assetsPrices.ChangePrices[pair][period] = &ChangePrices{}
			assetsPrices.ChangePricesDataset[pair][period] = &ChangePricesDataset{
				window: newWindow[DatasetChangePrices](int(duration.Minutes())),
			}
		}
		for period, duration := range periodsDelta {
			assetsPrices.ChangeDelta[pair][period] = &ChangeDelta{}
			assetsPrices.ChangeDeltaDataset[pair][period] = &ChangeDeltaDataset{
				window: newWindow[ChangeDelta](int(duration.Minutes()) * 2),
			}
		}
	}
	timeRounding := time.Now().Truncate(time.Minute)
	assetsPrices.UpdateTime = timeRounding
	assetsPrices.initChangePrices()
	assetsPrices.initChangeDelta()
	assetsPrices.initVolatility()
	assetsPrices.initActivity()

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
			ap.broadcastUpdate()
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

// Subscribe возвращает отдельный канал обновлений для одного потребителя.
// Канал буферизован на 1: если потребитель ещё обрабатывает предыдущий тик,
// новый тик не теряется, но и не копится.
func (ap *AssetsPrices) Subscribe() <-chan struct{} {
	ap.subscribersMu.Lock()
	defer ap.subscribersMu.Unlock()

	ch := make(chan struct{}, 1)
	ap.subscribers = append(ap.subscribers, ch)
	return ch
}

// broadcastUpdate рассылает тик всем подписчикам. Отправка неблокирующая:
// медленный потребитель не должен тормозить остальных.
func (ap *AssetsPrices) broadcastUpdate() {
	ap.subscribersMu.Lock()
	defer ap.subscribersMu.Unlock()

	for _, ch := range ap.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// GetPeriodCandles отдаёт агрегированные свечи периода из БД начиная с from.
// Одна такая свеча = одна выборка истории для стратегий: их метрики (change
// цены, дельта объёма/трейдов) считаются как отношение соседних свечей периода.
func (ap *AssetsPrices) GetPeriodCandles(period string, from time.Time) ([]exModel.Candle, error) {
	return ap.repo.SelectCandlesFromPeriod(period, from)
}

// GetChangePrices возвращает копию ChangePrices для пары+периода и признак
// того, что датасет уже заполнен (иначе ChangePercent не имеет смысла).
func (ap *AssetsPrices) GetChangePrices(pair, period string) (ChangePrices, bool) {
	ap.ChangePricesMu.RLock()
	defer ap.ChangePricesMu.RUnlock()

	ds, ok := ap.ChangePricesDataset[pair][period]
	if !ok || ds == nil || !ds.Fill {
		return ChangePrices{}, false
	}
	cp, ok := ap.ChangePrices[pair][period]
	if !ok || cp == nil {
		return ChangePrices{}, false
	}
	return *cp, true
}

// GetChangeDelta возвращает копию ChangeDelta для пары+периода и признак
// того, что дельта-датасет уже заполнен.
func (ap *AssetsPrices) GetChangeDelta(pair, period string) (ChangeDelta, bool) {
	ap.ChangeDeltaMu.RLock()
	defer ap.ChangeDeltaMu.RUnlock()

	ds, ok := ap.ChangeDeltaDataset[pair][period]
	if !ok || ds == nil || !ds.fill {
		return ChangeDelta{}, false
	}
	cd, ok := ap.ChangeDelta[pair][period]
	if !ok || cd == nil {
		return ChangeDelta{}, false
	}
	return *cd, true
}

// maxGap - разрыв между соседними свечами, после которого прогрев из БД
// прекращается: собирать окно из кусков, между которыми дыра, бессмысленно.
const maxGap = 10 * time.Minute

// warmupCandles читает свечи из БД для прогрева окон при старте.
// depth - на сколько назад брать историю (для дельт нужно вдвое больше).
func (ap *AssetsPrices) warmupCandles(depth time.Duration) []exModel.Candle {

	// Максимальный заданный период для запроса в бд
	var max time.Duration
	for _, dur := range ap.Periods {
		if dur > max {
			max = dur
		}
	}

	candles, err := ap.repo.SelectMarketStateTimev2(ap.UpdateTime.Add(-max * depth))
	if err != nil {
		pricesLogger.Errorf("error SelectMarketStateTimev2: %v", err)
		return nil
	}

	if len(candles) == 0 {
		pricesLogger.Info("Нет свечей")
		return nil
	}

	// candles[0] - самая свежая свеча. Если она сильно старше текущего времени,
	// значит в БД нет данных за период и горячий перезапуск не удался.
	if candles[0].Time.Sub(ap.UpdateTime) > maxGap {
		pricesLogger.Info("В базе данных отсутствуют данные за период, горячий перезапуск не удался")
		return nil
	}

	return candles
}

// initChangePrices прогревает окна цен из БД. Свечи приходят от новых к старым -
// ровно в том порядке, в котором окно их и хранит, поэтому каждая следующая
// дописывается в хвост (pushOlder).
func (ap *AssetsPrices) initChangePrices() {

	for _, candle := range ap.warmupCandles(1) {

		if !slices.Contains(ap.Pairs, candle.Pair) {
			continue
		}

		for period := range ap.Periods {

			data := ap.ChangePricesDataset[candle.Pair][period]
			if data.Fill {
				continue
			}

			// Дыра в данных - дальше окно не набираем
			if tail, ok := data.oldest(); ok && tail.Time.Sub(candle.Time) > maxGap {
				continue
			}

			data.pushOlder(DatasetChangePrices{Price: candle.Close, Time: candle.Time})

			if data.filled() {
				data.Fill = true
				if oldest, ok := data.oldest(); ok {
					ap.ChangePrices[candle.Pair][period].LastPrice = oldest.Price
				}
			}
		}
	}
}

// initChangeDelta прогревает окна дельт. Глубина вдвое больше: окно сравнивает
// два соседних периода.
func (ap *AssetsPrices) initChangeDelta() {

	for _, candle := range ap.warmupCandles(2) {

		if !slices.Contains(ap.Pairs, candle.Pair) {
			continue
		}

		for period := range ap.PeriodsDelta {

			data := ap.ChangeDeltaDataset[candle.Pair][period]
			if data.fill {
				continue
			}

			// Дыра в данных - дальше окно не набираем
			if tail, ok := data.oldest(); ok && tail.Time.Sub(candle.Time) > maxGap {
				continue
			}

			data.pushOlder(deltaFromCandle(candle))
			data.fill = data.filled()
		}
	}
}

func deltaFromCandle(candle exModel.Candle) ChangeDelta {
	return ChangeDelta{
		Time:      candle.Time,
		Volume:    candle.Volume,
		VolumeBuy: candle.ActiveBuyVolume,
		VolumeAsk: candle.ActiveAskVolume,
		Trades:    float64(candle.AmountTrade),
		TradesBuy: float64(candle.AmountTradeBuy),
		TradesAsk: float64(candle.AmountTradeAsk),
	}
}

func (ap *AssetsPrices) updateChangePrices() {
	ap.MarketsStatMu.RLock()
	defer ap.MarketsStatMu.RUnlock()
	ap.ChangePricesMu.Lock()
	defer ap.ChangePricesMu.Unlock()

	timeStart := time.Now()

	for _, pair := range ap.Pairs {
		stat := ap.MarketsStat[pair]

		for period := range ap.Periods {

			data := ap.ChangePricesDataset[pair][period]
			changePrices := ap.ChangePrices[pair][period]

			// Изменение считаем ДО сдвига окна: LastPrice сейчас - это цена в
			// начале окна, то есть period минут назад.
			if data.Fill {
				changePrices.ChangePercent = checkValuesDividing(stat.Price, changePrices.LastPrice)
				ap.recordVolatility(pair, period, changePrices.ChangePercent, timeStart)
			}

			data.pushNewest(DatasetChangePrices{Price: stat.Price, Time: stat.Time})

			if data.filled() {
				data.Fill = true
				if oldest, ok := data.oldest(); ok {
					changePrices.LastPrice = oldest.Price
				}
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

	// Свечи приходят от новых к старым, а вставлять их нужно в порядке
	// возрастания времени: если по паре пришли сразу две новые (feederapp
	// дописал предыдущую с опозданием), обе должны встать в датасет по порядку.
	for i := len(candles) - 1; i >= 0; i-- {
		candle := candles[i]

		if !slices.Contains(ap.Pairs, candle.Pair) {
			continue
		}

		for period := range ap.PeriodsDelta {

			data := ap.ChangeDeltaDataset[candle.Pair][period]

			// Пока окно не заполнено, считать нечего: сравнивались бы половины
			// разной длины. Дедупликацию берёт на себя pushNewest - запрос к БД
			// идёт с запасом в минуту, и одна и та же свеча приходит дважды.
			wasFilled := data.fill

			if !data.pushNewest(deltaFromCandle(candle)) {
				continue
			}

			if !wasFilled {
				data.fill = data.filled()
				continue
			}

			ap.recalcDelta(candle.Pair, period, data)
			ap.recordActivity(candle.Pair, period, *ap.ChangeDelta[candle.Pair][period], timeStart)
		}
	}

	duration := time.Since(timeStart)
	pricesLogger.Debugf("Время выполнения updateChangeDelta: %v ", duration)

	return nil
}

// recalcDelta сравнивает свежую половину окна со старой: окно хранится от новых
// к старым, поэтому первая половина по индексу - это последние period минут,
// вторая - предыдущие period минут.
func (ap *AssetsPrices) recalcDelta(pair, period string, data *ChangeDeltaDataset) {
	items := data.values()
	half := len(items) / 2

	recent := ChangeDelta{}
	previous := ChangeDelta{}

	for index, item := range items {
		target := &previous
		if index < half {
			target = &recent
		}

		target.Volume += item.Volume
		target.VolumeBuy += item.VolumeBuy
		target.VolumeAsk += item.VolumeAsk
		target.Trades += item.Trades
		target.TradesBuy += item.TradesBuy
		target.TradesAsk += item.TradesAsk
	}

	delta := ap.ChangeDelta[pair][period]
	delta.Volume = checkValuesDividing(recent.Volume, previous.Volume)
	delta.VolumeBuy = checkValuesDividing(recent.VolumeBuy, previous.VolumeBuy)
	delta.VolumeAsk = checkValuesDividing(recent.VolumeAsk, previous.VolumeAsk)
	delta.Trades = checkValuesDividing(recent.Trades, previous.Trades)
	delta.TradesBuy = checkValuesDividing(recent.TradesBuy, previous.TradesBuy)
	delta.TradesAsk = checkValuesDividing(recent.TradesAsk, previous.TradesAsk)
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
