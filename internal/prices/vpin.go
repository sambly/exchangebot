package prices

import (
	"sync"
	"time"

	"github.com/sambly/exchangebot/internal/stat"
)

// Параметры VPIN-подобного трекера (Volume-Synchronized Probability of
// Informed Trading, Easley/López de Prado/O'Hara).
//
// Классический VPIN оценивает долю покупок в баре через Bulk Volume
// Classification (нормальное распределение изменения цены внутри бара) -
// потому что у авторов не было настоящей классификации сделок по стороне.
// У нас она ЕСТЬ: candle.ActiveBuyVolume - реальный объём тейкер-покупок с
// биржи, а не оценка. BVC здесь не нужен - берём готовую классификацию, но
// заимствуем главную идею VPIN: копить её не по часам (как activity.go), а
// по НАБРАННОМУ ОБЪЁМУ. У временнОго окна на активном рынке и на затишье
// один и тот же ритм сэмплирования; у объёмного - бакет закрывается быстро
// во время реального всплеска (сигнал приходит вовремя) и медленно в
// затишье (не плодит шумные сэмплы из почти нулевой торговли).
const (
	// vpinBucketMultiplier - целевой объём бакета = столько "типичных"
	// минутных объёмов пары (см. vpinState.avgCandleVolume). 20 - "бакет
	// примерно на 20 обычных минут торговли", подстраивается по факту:
	// в 5 раз активнее обычного - закроется за ~4 минуты, в 5 раз тише -
	// копится ~100 минут.
	vpinBucketMultiplier = 20.0

	// vpinEWMAAlpha - вес нового наблюдения в скользящей оценке "типичного"
	// объёма минутной свечи пары.
	vpinEWMAAlpha = 0.05

	// vpinWindowSize/vpinMinSamples - см. imbalanceWindowSize/imbalanceMinSamples
	// в internal/depth - тот же принцип, для истории бакетов VPIN.
	vpinWindowSize = 90
	vpinMinSamples = 20

	// vpinScaleFloor - сигнал лежит в [-1,1] (2*buyShare-1, как имбаланс
	// стакана), поэтому и пол разброса такой же маленький.
	vpinScaleFloor = 0.02

	vpinConfirmZThreshold = 2.0
	vpinConfirmMinStreak  = 3
)

// vpinState - один объёмно-синхронизированный бакет на пару. Копится на
// КАЖДОЙ новой минутной свече (recordVPIN), а не на throttled-тике: сама суть
// в том, что момент коммита определяется набранным объёмом, а не часами.
type vpinState struct {
	mu sync.Mutex

	avgCandleVolume float64
	// primed - была ли хоть одна свеча вообще (для EWMA-затравки avgCandleVolume).
	// НЕ означает "бакет закоммичен" - это отдельно, см. committed: свеча №1
	// праймит среднее немедленно, а до первого реального коммита ещё далеко.
	primed         bool
	lastCandleTime time.Time

	bucketBuyVolume   float64
	bucketTotalVolume float64

	history *stat.MetricRecord
	confirm stat.SideConfirm

	// committed - закоммитился ли хоть один бакет. GetVPINZScore проверяет
	// именно это, а не primed - иначе он отдавал бы ready=true с первой же
	// свечи, с бессмысленными нулевыми lastValue/lastZScore.
	committed bool

	// lastValue/lastZScore - последний ЗАКОММИЧЕННЫЙ бакет. У VPIN, как и у
	// OFI в internal/depth, нет "текущего снимка" - только "уже накопленное
	// окно"; между коммитами читатели получают одно и то же значение.
	lastValue  float64
	lastZScore float64
}

func newVPINState() *vpinState {
	return &vpinState{
		history: stat.NewMetricRecord(vpinWindowSize, vpinMinSamples, vpinScaleFloor),
	}
}

func (ap *AssetsPrices) initVPIN() {
	ap.vpin = make(map[string]*vpinState, len(ap.Pairs))
	for _, pair := range ap.Pairs {
		ap.vpin[pair] = newVPINState()
	}
}

// recordVPIN - см. package doc у vpinState. Вызывается РОВНО РАЗ на каждую
// новую минутную свечу пары: дедупликация по lastCandleTime, тот же принцип,
// что у window.pushNewest (см. window.go), но не привязан к конкретному
// периоду - этот трекер один на пару, а не один на пару+период.
func (ap *AssetsPrices) recordVPIN(pair string, candleTime time.Time, buyVolume, totalVolume float64) {
	state, ok := ap.vpin[pair]
	if !ok || totalVolume <= 0 {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if !candleTime.After(state.lastCandleTime) {
		return
	}
	state.lastCandleTime = candleTime

	if !state.primed {
		state.avgCandleVolume = totalVolume
		state.primed = true
	} else {
		state.avgCandleVolume = state.avgCandleVolume*(1-vpinEWMAAlpha) + totalVolume*vpinEWMAAlpha
	}

	state.bucketBuyVolume += buyVolume
	state.bucketTotalVolume += totalVolume

	target := state.avgCandleVolume * vpinBucketMultiplier
	if target <= 0 || state.bucketTotalVolume < target {
		return
	}

	// value - доля покупок бакета, приведённая к [-1,1]: 0.5 (ровно половина
	// объёма - покупки) даёт 0, весь объём покупками - +1, весь продажами - -1.
	value := 2*state.bucketBuyVolume/state.bucketTotalVolume - 1

	z := state.history.ZScore(value)
	state.history.Add(value)
	state.confirm.Update(vpinSideFromZScore(z))
	state.lastValue = value
	state.lastZScore = z
	state.committed = true

	state.bucketBuyVolume = 0
	state.bucketTotalVolume = 0
}

// vpinSideFromZScore - см. depth.sideFromZScore, та же идея, локальная копия:
// маленькая чистая функция, не стоящая отдельной зависимости между пакетами.
// Возвращает "BUY"/"SELL"/"" строкой, а не order.SideType - internal/order
// уже зависит от internal/prices (OrderService держит *AssetsPrices), обратный
// импорт создал бы цикл. Строка "BUY"/"SELL" и так совпадает по значению с
// order.SideType - вызывающий код (например, entrysetup, который уже зависит
// от order) сам приводит её к нужному типу.
func vpinSideFromZScore(z float64) string {
	if z >= vpinConfirmZThreshold {
		return "BUY"
	}
	if z <= -vpinConfirmZThreshold {
		return "SELL"
	}
	return ""
}

// GetVPINZScore - сигнал последнего закоммиченного объёмного бакета (см.
// vpinState) и его z-score относительно истории пары. value в [-1,1]:
// положительное - в бакете преобладали покупки, отрицательное - продажи.
// ready=false, пока история не набрала vpinMinSamples закоммиченных бакетов -
// до этого ZScore тихо возвращает 0 (см. stat.MetricRecord.ZScore), и без
// этой проверки читатель получил бы ready=true с бессмысленным z=0 на все
// первые бакеты (для тонких пар это может быть половина суток - бакет
// коммитится раз в 20-55 минут). Тот же баг уже был найден и исправлен для
// RelativeStrength, см. GetRelativeStrengthZScore.
func (ap *AssetsPrices) GetVPINZScore(pair string) (value, zScore float64, ready bool) {
	state, ok := ap.vpin[pair]
	if !ok {
		return 0, 0, false
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.committed || state.history.Len() < vpinMinSamples {
		return 0, 0, false
	}
	return state.lastValue, state.lastZScore, true
}

// GetVPINConfirmedSide - сторона VPIN ("BUY"/"SELL"), если |z-score| держится
// выше vpinConfirmZThreshold минимум vpinConfirmMinStreak закоммиченных
// бакетов подряд. Тот же смысл, что у depth.GetOFIConfirmedSide - "держится
// уже какое-то время", а не "что показал последний бакет". Строка, а не
// order.SideType - см. комментарий у vpinSideFromZScore.
func (ap *AssetsPrices) GetVPINConfirmedSide(pair string) (side string, confirmed bool) {
	state, ok := ap.vpin[pair]
	if !ok {
		return "", false
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	return state.confirm.Confirmed(vpinConfirmMinStreak)
}
