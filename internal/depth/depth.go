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
)

var depthLogger = logger.AddFieldsEmpty()

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
}

func newBook() *book {
	return &book{
		bids: make(map[float64]float64),
		asks: make(map[float64]float64),
	}
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

// IsReady - пришёл ли по паре хотя бы один снапшот/апдейт стакана.
func (ad *AssetsDepth) IsReady(pair string) bool {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	b, ok := ad.books[pair]
	return ok && b.ready
}
