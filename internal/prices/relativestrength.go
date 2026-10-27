package prices

import (
	"sync"

	"github.com/sambly/exchangebot/internal/stat"
)

// Кросс-секционная относительная сила пары: на сколько её ChangePercent за
// period отличается от МЕДИАННОГО ChangePercent по ВСЕМ отслеживаемым парам
// за тот же period в тот же момент - то есть не "выросла ли пара", а
// "выросла ли она БОЛЬШЕ/МЕНЬШЕ рынка в целом прямо сейчас".
//
// Ни одна из существующих метрик (OFI, VPIN, PriceLevels, VolatilityRegime)
// этого не различает - все они смотрят на пару САМУ ПО СЕБЕ во времени.
// Рыночное движение (весь рынок падает вслед за BTC) выглядит для них так
// же, как идиосинкратическая аномалия конкретной пары - хотя по смыслу это
// совершенно разные вещи: "все падают" не значит "с этой парой что-то
// происходит".
//
// Медиана рынка, а не среднее - тот же принцип робастности, что и везде в
// этом пакете: одна-две пары с делистингом/разовым пампом не утягивают
// "рынок" за собой.
//
// z-score считается ОТ ИСТОРИИ ОТНОСИТЕЛЬНОЙ СИЛЫ КОНКРЕТНОЙ ПАРЫ (разницы
// "пара минус рынок"), а не от сырого ChangePercent - вопрос "необычно ли
// для ЭТОЙ пары обгонять/отставать от рынка настолько", а не "необычно ли
// само движение цены" (это уже задача internal/strategy/anomaly).
const (
	relativeStrengthWindowSize = 30
	relativeStrengthMinSamples = 20
	// relativeStrengthScaleFloor - в процентных пунктах (разница ChangePercent
	// пары и медианы рынка), не в доле - 0.05% пола достаточно, чтобы не дать
	// вырожденной истории (пара двигается ровно как рынок несколько тиков
	// подряд) взорвать z-score на следующем небольшом расхождении.
	relativeStrengthScaleFloor = 0.05

	relativeStrengthConfirmZThreshold = 2.0
	relativeStrengthConfirmMinStreak  = 3
)

// relativeStrengthState - история относительной силы одной пары+периода.
//
// Готовность НЕ хранится отдельным флагом - см. GetRelativeStrengthZScore:
// такой флаг легко подделать (был именно этот баг в первой версии - "ready"
// выставлялся при первом же тике, а ZScore ещё возвращал 0, потому что в
// history меньше relativeStrengthMinSamples значений; тот же класс ошибки,
// что уже правился у VPIN - см. vpinState.committed). Здесь источник истины
// один - history.Len(), отдельного флага не завести рассинхронизированным.
type relativeStrengthState struct {
	mu sync.Mutex

	history *stat.MetricRecord
	confirm stat.SideConfirm

	lastValue  float64
	lastZScore float64
}

func newRelativeStrengthState() *relativeStrengthState {
	return &relativeStrengthState{
		history: stat.NewMetricRecord(relativeStrengthWindowSize, relativeStrengthMinSamples, relativeStrengthScaleFloor),
	}
}

func (ap *AssetsPrices) initRelativeStrength() {
	ap.relativeStrength = make(map[string]map[string]*relativeStrengthState, len(ap.Pairs))
	for _, pair := range ap.Pairs {
		ap.relativeStrength[pair] = make(map[string]*relativeStrengthState, len(ap.Periods))
		for period := range ap.Periods {
			ap.relativeStrength[pair][period] = newRelativeStrengthState()
		}
	}
}

// recordRelativeStrength - см. package doc. Вызывается ИЗ updateChangePrices
// ПОСЛЕ того, как ChangePercent посчитан для ВСЕХ пар за period в этом же
// тике - иначе рыночную медиану считать не по чему. changes - ChangePercent
// пары за period, только для пар, у которых окно уже заполнено (см. вызов в
// prices.go: заполняется, только если data.Fill==true).
func (ap *AssetsPrices) recordRelativeStrength(period string, changes map[string]float64) {
	// Меньше 2 пар - "рынок" не определить (медиана по одному значению - это
	// просто само значение, любая пара оказалась бы "точно как рынок").
	if len(changes) < 2 {
		return
	}

	values := make([]float64, 0, len(changes))
	for _, v := range changes {
		values = append(values, v)
	}
	marketMedian, _ := stat.MedianAndScale(values, 0)

	for pair, changePercent := range changes {
		byPeriod, ok := ap.relativeStrength[pair]
		if !ok {
			continue
		}
		state, ok := byPeriod[period]
		if !ok {
			continue
		}

		value := changePercent - marketMedian

		state.mu.Lock()
		z := state.history.ZScore(value)
		state.history.Add(value)
		state.confirm.Update(relativeStrengthSideFromZScore(z))
		state.lastValue = value
		state.lastZScore = z
		state.mu.Unlock()
	}
}

// relativeStrengthSideFromZScore - см. depth.sideFromZScore/vpinSideFromZScore,
// та же идея, локальная копия по тем же причинам (маленькая чистая функция,
// не стоящая отдельной межпакетной зависимости).
func relativeStrengthSideFromZScore(z float64) string {
	if z >= relativeStrengthConfirmZThreshold {
		return "BUY"
	}
	if z <= -relativeStrengthConfirmZThreshold {
		return "SELL"
	}
	return ""
}

// GetRelativeStrengthZScore - относительная сила пары к рынку (ChangePercent
// пары минус медиана ChangePercent по рынку за тот же period) и её z-score
// относительно ИСТОРИИ ЭТОЙ ЖЕ РАЗНИЦЫ у пары. value>0 - пара обгоняет рынок,
// value<0 - отстаёт. ready=false, пока не накопилось достаточно тиков с
// определённым рынком (см. relativeStrengthMinSamples).
func (ap *AssetsPrices) GetRelativeStrengthZScore(pair, period string) (value, zScore float64, ready bool) {
	byPeriod, ok := ap.relativeStrength[pair]
	if !ok {
		return 0, 0, false
	}
	state, ok := byPeriod[period]
	if !ok {
		return 0, 0, false
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.history.Len() < relativeStrengthMinSamples {
		return 0, 0, false
	}
	return state.lastValue, state.lastZScore, true
}

// GetRelativeStrengthConfirmedSide - сторона относительной силы ("BUY" -
// обгоняет рынок, "SELL" - отстаёт), если она держится минимум
// relativeStrengthConfirmMinStreak тиков подряд. Строка, а не order.SideType -
// см. комментарий у vpinSideFromZScore (internal/order уже зависит от
// internal/prices, обратный импорт дал бы цикл).
func (ap *AssetsPrices) GetRelativeStrengthConfirmedSide(pair, period string) (side string, confirmed bool) {
	byPeriod, ok := ap.relativeStrength[pair]
	if !ok {
		return "", false
	}
	state, ok := byPeriod[period]
	if !ok {
		return "", false
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	return state.confirm.Confirmed(relativeStrengthConfirmMinStreak)
}
