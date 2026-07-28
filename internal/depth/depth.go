// Package depth хранит L2-стакан (глубину рынка) по каждой отслеживаемой
// паре. По структуре и назначению - аналог internal/prices.AssetsPrices, но
// вместо цены/объёма/свечей здесь bid/ask уровни biржи.
package depth

import (
	"sort"
	"sync"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/stat"
)

var depthLogger = logger.AddFieldsEmpty()

// Параметры z-score имбаланса стакана (см. book.imbalanceHistory).
//
// Сырой имбаланс сам по себе плохо интерпретируется: у низколиквидной пары
// книга может быть почти всегда перекошена в одну сторону, и 0.6 для неё -
// норма, а не сигнал. Z-score (та же робастная медиана+MAD, что использует
// internal/strategy/anomaly - см. internal/stat) отвечает на вопрос "необычно
// ли ЭТО значение ДЛЯ ЭТОЙ ПАРЫ", а не "перевешивают ли биды прямо сейчас".
const (
	// imbalanceLevels - сколько лучших уровней с каждой стороны участвует в
	// расчёте имбаланса. Слишком мало - шумно от одной случайной заявки;
	// слишком много - учитывает уровни, до которых реальная сделка никогда
	// не доберётся.
	imbalanceLevels = 20

	// imbalanceSampleInterval - как часто новое значение имбаланса попадает в
	// историю для z-score. Апдейты стакана летят много раз в секунду - копить
	// историю с этой частотой означало бы, что все точки почти идентичны
	// (окно не успевает измениться), и разброс (MAD) занижается - та же
	// проблема автокорреляции, что и в anomaly (см. PeriodState.nextSampleAt
	// там).
	imbalanceSampleInterval = 10 * time.Second

	// imbalanceWindowSize samples * imbalanceSampleInterval = ~15 минут истории.
	imbalanceWindowSize = 90
	// imbalanceMinSamples - до накопления этого числа выборок (~3.3 минуты)
	// z-score не считается вообще, см. stat.MetricRecord.
	imbalanceMinSamples = 20
	// imbalanceScaleFloor - имбаланс лежит в [-1,1]; ниже этого разброс не
	// считается значимым (гасит шум у пар, где книга почти не шевелится).
	imbalanceScaleFloor = 0.02

	// imbalanceConfirmZThreshold - порог |z-score|, начиная с которого сэмпл
	// считается "имбаланс явно перевешивает в одну сторону" для счётчика
	// устойчивости (см. book.imbalanceSideConfirm). Тот же порог, что уже
	// используется на фронте для подсветки (Z_THRESHOLD в DataImbalance.vue/
	// DepthChart.vue) - ниже него сэмпл и так не считается заметным нигде
	// больше, незачем заводить для устойчивости отдельную планку.
	imbalanceConfirmZThreshold = 2.0

	// imbalanceConfirmMinStreak - сколько сэмплов подряд (см.
	// imbalanceSampleInterval, т.е. ~imbalanceConfirmMinStreak*10 секунд)
	// сторона должна держаться, чтобы считаться подтверждённой, а не
	// секундным шумом/спуфингом. Меньше - подтверждение почти сразу теряет
	// смысл (одна заявка = "подтверждено"); больше - реальный, но короткий
	// перекос никогда не успеет подтвердиться.
	imbalanceConfirmMinStreak = 3
)

// Level - один уровень стакана: цена и суммарное количество на ней.
type Level struct {
	Price    float64
	Quantity float64
}

// Snapshot - копия стакана пары для чтения наружу. Уровни отдаются как есть,
// без сортировки и без ограничения глубины - top-N/сортировка по цене решается
// на стороне потребителя, у разных задач (imbalance, оценка слиппеджа, стены
// ликвидности) разные требования к этому.
type Snapshot struct {
	Bids         map[float64]float64
	Asks         map[float64]float64
	Time         time.Time
	LastUpdateID int64
	Ready        bool
}

// book - состояние стакана одной пары. Формат (map цена->количество) такой
// же, как у exchangeService/pkg/exchange.OrderBook: diff-обновление построчно
// заменяет количество на уровне, а количество 0 означает "уровень пропал".
type book struct {
	bids         map[float64]float64
	asks         map[float64]float64
	time         time.Time
	lastUpdateID int64
	ready        bool

	// imbalanceHistory - скользящее окно сэмплов имбаланса для z-score.
	// Не сбрасывается в clear(): пересинк книги - это разрыв в сырых
	// уровнях, а не повод забыть статистическое поведение пары.
	imbalanceHistory      *stat.MetricRecord
	nextImbalanceSampleAt time.Time

	// imbalanceSideConfirm - счётчик устойчивости стороны имбаланса (см.
	// GetImbalanceConfirmedSide), обновляется на том же тике, что и
	// imbalanceHistory - реже, чем реальные апдейты книги, специально, чтобы
	// счётчик не набивался за доли секунды на одном и том же шумном тике.
	imbalanceSideConfirm stat.SideConfirm
}

func newBook() *book {
	return &book{
		bids:             make(map[float64]float64),
		asks:             make(map[float64]float64),
		imbalanceHistory: stat.NewMetricRecord(imbalanceWindowSize, imbalanceMinSamples, imbalanceScaleFloor),
	}
}

// imbalance - соотношение объёма топ-N уровней бидов и асков:
// (bidVol-askVol)/(bidVol+askVol), диапазон [-1,1]. Положительное - перевес
// покупателей, отрицательное - продавцов. Вызывающий код обязан уже держать
// нужный лок AssetsDepth.mu.
func (b *book) imbalance(levels int) (float64, bool) {
	bidPrices := make([]float64, 0, len(b.bids))
	for price := range b.bids {
		bidPrices = append(bidPrices, price)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(bidPrices)))
	if levels > 0 && len(bidPrices) > levels {
		bidPrices = bidPrices[:levels]
	}
	bidVol := 0.0
	for _, price := range bidPrices {
		bidVol += b.bids[price]
	}

	askPrices := make([]float64, 0, len(b.asks))
	for price := range b.asks {
		askPrices = append(askPrices, price)
	}
	sort.Float64s(askPrices)
	if levels > 0 && len(askPrices) > levels {
		askPrices = askPrices[:levels]
	}
	askVol := 0.0
	for _, price := range askPrices {
		askVol += b.asks[price]
	}

	total := bidVol + askVol
	if total == 0 {
		return 0, false
	}
	return (bidVol - askVol) / total, true
}

func (b *book) apply(update exModel.DepthUpdate) {
	for _, level := range update.Bids {
		if level.Quantity == 0 {
			delete(b.bids, level.Price)
		} else {
			b.bids[level.Price] = level.Quantity
		}
	}
	for _, level := range update.Asks {
		if level.Quantity == 0 {
			delete(b.asks, level.Price)
		} else {
			b.asks[level.Price] = level.Quantity
		}
	}
	b.time = update.Time
	b.lastUpdateID = update.LastUpdateID
}

func (b *book) clear() {
	b.bids = make(map[float64]float64)
	b.asks = make(map[float64]float64)
	b.lastUpdateID = 0
}

// AssetsDepth - L2-стакан по каждой паре. Наполняется подпиской на
// exchange.DataFeed.SubscribeObserverDepth (см. internal/application/app.go),
// потокобезопасен - как и AssetsPrices, один RWMutex на все пары.
type AssetsDepth struct {
	Pairs []string

	mu    sync.RWMutex
	books map[string]*book
}

func NewAssetsDepth(pairs []string) *AssetsDepth {
	ad := &AssetsDepth{
		Pairs: pairs,
		books: make(map[string]*book, len(pairs)),
	}
	for _, pair := range pairs {
		ad.books[pair] = newBook()
	}
	return ad
}

// OnDepth - точка входа из DataFeed.SubscribeObserverDepth. Консьюмер там
// типизирован как func(data any): в один и тот же канал попадают и апдейты
// стакана (exModel.DepthUpdate - это и diff, и синтетический снапшот после
// ре-синка), и состояние готовности пары (exModel.DepthState).
func (ad *AssetsDepth) OnDepth(data any) {
	switch v := data.(type) {
	case exModel.DepthUpdate:
		ad.applyUpdate(v)
	case exModel.DepthState:
		ad.setReady(v.Pair, v.Ready)
	default:
		depthLogger.Warnf("OnDepth: неизвестный тип payload %T", data)
	}
}

func (ad *AssetsDepth) applyUpdate(update exModel.DepthUpdate) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	b, ok := ad.books[update.Pair]
	if !ok {
		return
	}
	b.apply(update)

	// exchangeService/pkg/exchange.DataFeed.StartDepthFeeder (не Combined,
	// именно им сейчас пользуется exchangebot) шлёт DepthState только внутренним
	// потребителям FeedStatus, а в наблюдатели (SubscribeObserverDepth) - нет:
	// Notify(depthState) там не вызывается, только Notify(depthUpdate). Поэтому
	// готовность здесь выставляется по факту первого успешно применённого
	// апдейта, а не по приходу DepthState (setReady остаётся на случай перехода
	// на Combined-подписку в будущем).
	b.ready = true

	now := time.Now()
	if !now.Before(b.nextImbalanceSampleAt) {
		if value, ok := b.imbalance(imbalanceLevels); ok {
			// z-score ДО Add (см. комментарий у MetricRecord.ZScore) - иначе
			// сэмпл занижает свой же z относительно самого себя.
			z := b.imbalanceHistory.ZScore(value)
			b.imbalanceHistory.Add(value)
			b.imbalanceSideConfirm.Update(sideFromZScore(z))
		}
		b.nextImbalanceSampleAt = now.Add(imbalanceSampleInterval)
	}
}

// sideFromZScore - сторона имбаланса для счётчика устойчивости: "" (нет явной
// стороны), пока |z| не превысил imbalanceConfirmZThreshold.
func sideFromZScore(z float64) string {
	if z >= imbalanceConfirmZThreshold {
		return string(order.SideTypeBuy)
	}
	if z <= -imbalanceConfirmZThreshold {
		return string(order.SideTypeSell)
	}
	return ""
}

func (ad *AssetsDepth) setReady(pair string, ready bool) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	b, ok := ad.books[pair]
	if !ok {
		return
	}
	b.ready = ready
	if !ready {
		b.clear()
	}
}

// GetSnapshot возвращает копию сырых уровней стакана пары.
func (ad *AssetsDepth) GetSnapshot(pair string) (Snapshot, bool) {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	b, ok := ad.books[pair]
	if !ok {
		return Snapshot{}, false
	}

	bidsCopy := make(map[float64]float64, len(b.bids))
	for price, qty := range b.bids {
		bidsCopy[price] = qty
	}
	asksCopy := make(map[float64]float64, len(b.asks))
	for price, qty := range b.asks {
		asksCopy[price] = qty
	}

	return Snapshot{
		Bids:         bidsCopy,
		Asks:         asksCopy,
		Time:         b.time,
		LastUpdateID: b.lastUpdateID,
		Ready:        b.ready,
	}, true
}

// GetBestBidAsk - лучшая цена покупки/продажи. Линейный проход по карте без
// полной сортировки стакана - для top-of-book этого достаточно и заметно
// дешевле, чем пересортировывать весь стакан (у ликвидных пар - тысячи
// уровней) на каждый вызов.
func (ad *AssetsDepth) GetBestBidAsk(pair string) (bid, ask Level, ready bool) {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	b, ok := ad.books[pair]
	if !ok || !b.ready {
		return Level{}, Level{}, false
	}

	var bestBid, bestAsk Level
	for price, qty := range b.bids {
		if price > bestBid.Price {
			bestBid = Level{Price: price, Quantity: qty}
		}
	}
	for price, qty := range b.asks {
		if bestAsk.Price == 0 || price < bestAsk.Price {
			bestAsk = Level{Price: price, Quantity: qty}
		}
	}

	if bestBid.Price == 0 || bestAsk.Price == 0 {
		return Level{}, Level{}, false
	}

	return bestBid, bestAsk, true
}

// GetTopLevels отдаёт лучшие n уровней стакана с каждой стороны, уже
// отсортированные к цене (bids по убыванию от лучшего бида, asks по
// возрастанию от лучшего аска) - то, что нужно для cumulative depth chart:
// накопление считается последовательным проходом от спреда наружу. n<=0 -
// без ограничения глубины.
func (ad *AssetsDepth) GetTopLevels(pair string, n int) (bids, asks []Level, ready bool) {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	b, ok := ad.books[pair]
	if !ok || !b.ready {
		return nil, nil, false
	}

	bids = make([]Level, 0, len(b.bids))
	for price, qty := range b.bids {
		bids = append(bids, Level{Price: price, Quantity: qty})
	}
	sort.Slice(bids, func(i, j int) bool { return bids[i].Price > bids[j].Price })

	asks = make([]Level, 0, len(b.asks))
	for price, qty := range b.asks {
		asks = append(asks, Level{Price: price, Quantity: qty})
	}
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })

	if n > 0 {
		if len(bids) > n {
			bids = bids[:n]
		}
		if len(asks) > n {
			asks = asks[:n]
		}
	}

	return bids, asks, true
}

// GetImbalanceZScore - текущий имбаланс топ-N уровней стакана (см.
// imbalanceLevels) и его z-score относительно недавней истории самой этой
// пары. imbalance лежит в [-1,1]: положительный - перевес покупателей,
// отрицательный - продавцов. zScore==0 до накопления imbalanceMinSamples
// выборок (см. stat.MetricRecord) - за это время imbalance уже осмыслен, а
// zScore ещё нет.
func (ad *AssetsDepth) GetImbalanceZScore(pair string) (imbalance, zScore float64, ready bool) {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	b, ok := ad.books[pair]
	if !ok || !b.ready {
		return 0, 0, false
	}

	value, ok := b.imbalance(imbalanceLevels)
	if !ok {
		return 0, 0, false
	}

	return value, b.imbalanceHistory.ZScore(value), true
}

// GetImbalanceConfirmedSide - сторона имбаланса (BUY/SELL), если |z-score|
// держится выше imbalanceConfirmZThreshold минимум imbalanceConfirmMinStreak
// сэмплов подряд (см. book.imbalanceSideConfirm). confirmed=false - либо
// сторона ещё не набрала нужный стрик, либо сейчас имбаланс не выражен
// вовсе. В отличие от GetImbalanceZScore, это не "сколько сейчас", а "держится
// ли это уже какое-то время" - для решений, которым важна не мгновенная
// картина, а устойчивый перекос (см. entrysetup.GetAllStrengthComponents).
func (ad *AssetsDepth) GetImbalanceConfirmedSide(pair string) (side order.SideType, confirmed bool) {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	b, ok := ad.books[pair]
	if !ok {
		return "", false
	}

	s, ok := b.imbalanceSideConfirm.Confirmed(imbalanceConfirmMinStreak)
	if !ok {
		return "", false
	}
	return order.SideType(s), true
}

// ImbalanceStat - имбаланс и его z-score для одной пары, для таблицы
// "по всем парам сразу" (см. GetAllImbalanceZScore). Ready=false - пара не
// готова (нет апдейтов ещё) или книга сейчас пуста с одной из сторон.
type ImbalanceStat struct {
	Imbalance float64
	ZScore    float64
	Ready     bool
}

// GetAllImbalanceZScore - то же самое, что GetImbalanceZScore, но сразу по
// всем отслеживаемым парам одним проходом под одним локом - для таблицы
// имбаланса по рынку целиком (аналог GetAllChPrice/GetAllChDelta в prices).
func (ad *AssetsDepth) GetAllImbalanceZScore() map[string]ImbalanceStat {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	result := make(map[string]ImbalanceStat, len(ad.books))
	for pair, b := range ad.books {
		if !b.ready {
			result[pair] = ImbalanceStat{}
			continue
		}

		value, ok := b.imbalance(imbalanceLevels)
		if !ok {
			result[pair] = ImbalanceStat{}
			continue
		}

		result[pair] = ImbalanceStat{
			Imbalance: value,
			ZScore:    b.imbalanceHistory.ZScore(value),
			Ready:     true,
		}
	}
	return result
}

// IsReady - пришёл ли по паре хотя бы один снапшот/апдейт стакана.
func (ad *AssetsDepth) IsReady(pair string) bool {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	b, ok := ad.books[pair]
	return ok && b.ready
}
