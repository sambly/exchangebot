package anomaly

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/signal"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	"github.com/sambly/exchangebot/internal/toggle"
)

// Константы перевода робастных оценок разброса в шкалу стандартного отклонения
// нормального распределения. Благодаря им пороги z (3/5/8) остаются в привычной
// "сигма"-шкале, хотя разброс считается робастно.
const (
	// 1 / 0.6745: MAD нормального распределения = 0.6745 * sigma
	madToSigma = 1.4826
	// 1 / 0.7979: среднее абсолютное отклонение нормального = 0.7979 * sigma
	meanADToSigma = 1.2533
)

// MetricRecord хранит историю значений для одной метрики
type MetricRecord struct {
	mu     sync.RWMutex
	values []float64 // кольцевой буфер
	maxLen int
	// minSamples - минимум значений в буфере, до которого z-score не считается.
	// Без этого порога на 2-3 значениях разброс оценивается по шуму: два случайно
	// близких значения дают крошечный разброс, и следующее рядовое значение
	// получает z=10. Практически это означало бы шквал ложных 🚨 после каждого
	// рестарта, пока буфер не наберётся.
	minSamples int
	// scaleFloor - нижняя граница разброса. У пар, где метрика почти всегда
	// стоит на месте (стейблкоины по цене, неликвид по объёму), MAD вырождается
	// почти в ноль, и любое шевеление даёт z в десятки. Пол разброса переводит
	// такой z обратно в осмысленный диапазон.
	scaleFloor float64
}

func NewMetricRecord(maxLen, minSamples int, scaleFloor float64) *MetricRecord {
	if minSamples < 2 {
		minSamples = 2
	}
	if minSamples > maxLen {
		minSamples = maxLen
	}
	return &MetricRecord{
		values:     make([]float64, 0, maxLen),
		maxLen:     maxLen,
		minSamples: minSamples,
		scaleFloor: scaleFloor,
	}
}

func (mr *MetricRecord) Add(value float64) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.values = append(mr.values, value)
	if len(mr.values) > mr.maxLen {
		mr.values = mr.values[1:]
	}
}

func (mr *MetricRecord) Len() int {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	return len(mr.values)
}

// median возвращает медиану копии среза (сам срез не переупорядочивается)
func median(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// Scale - робастная оценка типичного разброса метрики (в тех же единицах, что
// и сама метрика: для price это проценты). Это знаменатель z-score.
//
// Наружу нужна политике выхода: «типичное движение пары за период» - честная
// мера того, что для этой пары много, а что шум.
func (mr *MetricRecord) Scale() float64 {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if len(mr.values) < mr.minSamples {
		return 0
	}
	return mr.scaleLocked()
}

// ZScore вычисляет робастный z-score для value ОТНОСИТЕЛЬНО ТЕКУЩЕЙ истории,
// не включая само value. Вызывающий код обязан вызвать ZScore ДО Add, иначе
// выброс попадает в собственную базу и занижает свой же z-score.
//
// Используются медиана и MAD (median absolute deviation), а не среднее и
// стандартное отклонение: у крипты тяжёлые хвосты, и одно прошлое экстремальное
// значение, осевшее в буфере, раздувает stddev настолько, что следующая реальная
// аномалия уже не проходит порог. Медиана и MAD к таким выбросам нечувствительны.
func (mr *MetricRecord) ZScore(value float64) float64 {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if len(mr.values) < mr.minSamples {
		return 0
	}

	scale := mr.scaleLocked()
	if scale == 0 {
		return 0
	}

	return (value - median(mr.values)) / scale
}

// scaleLocked - робастная оценка разброса: MAD, приведённый к шкале сигмы.
func (mr *MetricRecord) scaleLocked() float64 {
	med := median(mr.values)

	deviations := make([]float64, len(mr.values))
	for i, v := range mr.values {
		deviations[i] = math.Abs(v - med)
	}

	scale := median(deviations) * madToSigma

	// MAD вырождается в 0, если больше половины значений одинаковы (например,
	// объём стабильно нулевой). Тогда падаем на среднее абсолютное отклонение,
	// оно устойчивее к вырождению, но всё ещё робастнее stddev.
	if scale == 0 {
		sum := 0.0
		for _, d := range deviations {
			sum += d
		}
		scale = (sum / float64(len(deviations))) * meanADToSigma
	}

	if scale < mr.scaleFloor {
		scale = mr.scaleFloor
	}
	return scale
}

// AnomalyResult результат проверки одной пары за один период
type AnomalyResult struct {
	Pair        string
	Period      string
	Level       int // 1, 2, 3
	Metrics     map[string]MetricAnomaly
	CompositeZ  float64
	IsAnomalous bool

	// HasPrice - была ли посчитана ценовая метрика. Если нет, у результата НЕТ
	// направления: объём и число сделок растут и на выносе вверх, и на сбросе
	// вниз, поэтому вывести из них сторону сделки невозможно.
	HasPrice bool
	// PriceChange - сырое изменение цены за период, %. Именно оно (а не z-score)
	// задаёт направление сигнала: z считается относительно МЕДИАНЫ истории, и у
	// пары в устойчивом росте выросшая цена запросто даёт отрицательный z.
	PriceChange float64
	// PriceZ - z-score цены со знаком
	PriceZ float64

	// HasActivity - была ли посчитана хоть одна метрика объёма/сделок
	HasActivity bool
	// ActivityZ - насколько торговая активность выше обычной (z со знаком)
	ActivityZ float64
	// Divergent - цена сходила, а активность была НИЖЕ обычной: движение по
	// пустому стакану. Чаще всего откатывается; в дайджесте помечается ⚡,
	// чтобы читатель не принял вынос за подтверждённый импульс.
	Divergent bool
}

// Семейства метрик. Внутри семейства метрики почти полностью скоррелированы
// (вырос объём - выросло и число сделок, и объём покупок), поэтому агрегировать
// их надо вместе, а не считать каждую независимым подтверждением.
const (
	familyPrice = "price"
	// familyActivity - "сколько торгуют": объёмы и число сделок. Метрика
	// НЕнаправленная: растёт и когда пару разгоняют, и когда сливают.
	familyActivity = "activity"
)

func metricFamily(metric string) string {
	if metric == "price" {
		return familyPrice
	}
	return familyActivity
}

type MetricAnomaly struct {
	Value     float64
	ZScore    float64
	Threshold float64 // порог того уровня, который реально был достигнут (0, если уровень 0 - тогда это Level1, ближайший непройденный порог)
	IsAnomaly bool
}

// PeriodState хранит состояние cooldown для пары+периода
type PeriodState struct {
	cooldownUntil time.Time
	lastLevel     int // последний отправленный уровень (1, 2, 3); 0 - аномалии сейчас нет

	// nextSampleAt - момент, когда разрешено СЛЕДУЮЩЕЕ добавление значения
	// в историю (MetricRecord). Источники (ChangePrices/ChangeDelta) сами
	// пересчитываются каждую минуту скользящим окном, поэтому если писать
	// в историю каждую минуту, соседние точки буфера будут почти идентичны
	// (окно сдвигается всего на 1 минуту из period/2*period) - это занижает
	// оценку stddev и ломает z-score. Поэтому в историю пишем не чаще
	// одного раза за period, а проверку (z-score) делаем каждую минуту
	// против уже накопленной истории.
	nextSampleAt time.Time

	// Окно подтверждения (см. PeriodConfig.ConfirmChecks): сколько проверок подряд
	// аномалия держится и какой у неё ПИКОВЫЙ уровень за это окно. Смена
	// направления означает, что событие ДРУГОЕ, и счётчик надо начинать заново,
	// а не досчитывать чужие минуты.
	pendingChecks    int
	pendingLevel     int
	pendingDirection signal.Direction
}

// confirm - выдержала ли аномалия окно подтверждения.
//
// Возвращает true, когда она продержалась checks проверок подряд, и подставляет
// в результат ПИКОВЫЙ уровень за это окно. Пиковый, а не усреднённый: смысл окна
// в том, чтобы отсеять секундный дёрг рынка, а не в том, чтобы размазать
// сильнейшую минуту движения по соседним спокойным.
func confirm(state *PeriodState, result *AnomalyResult, checks int) bool {
	if checks <= 1 {
		return true
	}

	direction := directionFor(result.PriceChange)

	if state.pendingChecks == 0 || state.pendingDirection != direction {
		state.pendingChecks = 1
		state.pendingLevel = result.Level
		state.pendingDirection = direction
	} else {
		state.pendingChecks++
		if result.Level > state.pendingLevel {
			state.pendingLevel = result.Level
		}
	}

	if state.pendingChecks < checks {
		return false
	}

	result.Level = state.pendingLevel
	return true
}

// LevelFromZScore определяет уровень аномалии по z-score, учитывая пороги периода
func LevelFromZScore(zScore float64, thresholds *ThresholdsConfig) int {
	absZ := math.Abs(zScore)
	if thresholds == nil {
		return 0
	}
	if absZ >= thresholds.Level3 {
		return 3
	}
	if absZ >= thresholds.Level2 {
		return 2
	}
	if absZ >= thresholds.Level1 {
		return 1
	}
	return 0
}

// thresholdForLevel возвращает порог, который соответствует достигнутому уровню
// (а не всегда Level1, как было раньше).
func thresholdForLevel(level int, thresholds *ThresholdsConfig) float64 {
	switch level {
	case 3:
		return thresholds.Level3
	case 2:
		return thresholds.Level2
	default:
		return thresholds.Level1
	}
}

type AnomalyStrategy struct {
	Config       *Config
	Notification *notification.Notification
	TelegramMenu *AnomalyMenu

	// StrategyEnable/NotificationEnable переключаются из телеграм-меню, а
	// читаются горутиной стратегии - поэтому живут отдельно от Config, за
	// мьютексом (см. пакет toggle).
	StrategyEnable     *toggle.Bool
	NotificationEnable *toggle.Bool

	AssetsPrices *prices.AssetsPrices
	Periods      map[string]time.Duration

	// История для каждой пары + периода + метрики
	history map[string]map[string]map[string]*MetricRecord

	// Cooldown состояние для каждой пары + периода
	states map[string]map[string]*PeriodState

	// Для рыночной метрики: cooldown отдельно на каждый период, иначе
	// сработавшая аномалия по длинному периоду (1d) заглушила бы рыночные
	// уведомления по коротким (15m, 1h) на всю свою длительность.
	marketStates map[string]*PeriodState

	// Подписчики на сигналы - исполнители сделок. Стратегия не знает, кто это
	// и что они с сигналом сделают.
	subscribers []chan signal.Signal

	mu sync.RWMutex
}

// Subscribe подписывает исполнителя на сигналы стратегии
func (s *AnomalyStrategy) Subscribe(ch chan signal.Signal) {
	s.subscribers = append(s.subscribers, ch)
}

// publish рассылает сигналы подписчикам НЕблокирующе: торговый модуль не должен
// тормозить детектор, а протухший сигнал никому не нужен.
func (s *AnomalyStrategy) publish(results []*AnomalyResult) {
	if len(s.subscribers) == 0 {
		return
	}

	for _, result := range results {
		// Без ценовой метрики у сигнала нет направления, а исполнитель обязан
		// выбрать сторону сделки. Молча отдать ему DirectionUp - значит купить
		// на любом всплеске объёма, в том числе на сбросе.
		if !result.HasPrice {
			continue
		}

		sig := s.signalFrom(result)

		for _, sub := range s.subscribers {
			select {
			case sub <- sig:
			default:
				anomalyLogger.Warnf("подписчик не успевает, сигнал %s %s отброшен", sig.Pair, sig.Period)
			}
		}
	}
}

func (s *AnomalyStrategy) signalFrom(result *AnomalyResult) signal.Signal {
	return signal.Signal{
		Source:        s.Config.IDName,
		Pair:          result.Pair,
		Period:        result.Period,
		Time:          time.Now(),
		Direction:     directionFor(result.PriceChange),
		Level:         result.Level,
		Strength:      result.CompositeZ,
		ChangePercent: result.PriceChange,
		Volatility:    s.priceVolatility(result.Pair, result.Period),
		Reason: fmt.Sprintf("%s %s z=%.1f level=%d",
			s.Config.IDName, result.Period, result.CompositeZ, result.Level),
	}
}

// directionFor - сторона движения ЦЕНЫ, и только цены.
//
// Раньше направление бралось из знака CompositeZ, а это z метрики с максимальным
// |z| среди всех - в том числе объёма или числа сделок. Но объём ненаправлен: он
// растёт и когда пару разгоняют, и когда её сливают. Если цена падала на 8%
// (z=-5) при взлетевшем объёме (z=+9), композит был +9, сигнал уходил как UP, и
// исполнитель с onUp:buy покупал падающий нож.
func directionFor(priceChange float64) signal.Direction {
	if priceChange < 0 {
		return signal.DirectionDown
	}
	return signal.DirectionUp
}

// priceVolatility - типичное движение цены пары за период (%), по той же
// робастной оценке, из которой считается z-score.
func (s *AnomalyStrategy) priceVolatility(pair, period string) float64 {
	record, ok := s.history[pair][period]["price"]
	if !ok || record == nil {
		return 0
	}
	return record.Scale()
}

var anomalyLogger = logger.AddFields(map[string]interface{}{
	"package": "anomaly",
})

func NewStrategy(
	assetsPrices *prices.AssetsPrices,
	periods map[string]time.Duration,
	pairs []string,
	notify *notification.Notification,
) (*AnomalyStrategy, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	if cfg.AllPairs {
		cfg.Pairs = pairs
	}

	str := &AnomalyStrategy{
		Config:             cfg,
		AssetsPrices:       assetsPrices,
		Periods:            periods,
		Notification:       notify,
		StrategyEnable:     toggle.New(cfg.StrategyEnable),
		NotificationEnable: toggle.New(cfg.NotificationEnable),
		history:            make(map[string]map[string]map[string]*MetricRecord),
		states:             make(map[string]map[string]*PeriodState),
		marketStates:       make(map[string]*PeriodState),
	}

	for period := range periods {
		str.marketStates[period] = &PeriodState{}
	}

	// Инициализация истории и состояний для каждой пары + периода
	for _, pair := range cfg.Pairs {
		str.history[pair] = make(map[string]map[string]*MetricRecord)
		str.states[pair] = make(map[string]*PeriodState)
		for period := range periods {
			str.history[pair][period] = make(map[string]*MetricRecord)
			str.states[pair][period] = &PeriodState{}
			str.initMetricsForPair(pair, period, str.windowSizeForPeriod(period))
		}
	}

	if !cfg.Metrics.Price {
		anomalyLogger.Warn("метрика price выключена: у сигналов нет направления, торговые подписчики их не получат")
	}

	if str.StrategyEnable.Get() {
		str.seedHistory()
	}

	return str, nil
}

func (s *AnomalyStrategy) WithTelegramMenu() *AnomalyStrategy {
	s.TelegramMenu = NewMenu(s.Config.Name, s.Config.IDName, s)
	return s
}

// seedHistory заполняет историю метрик из БД при старте.
//
// Агрегированная свеча периода = ровно одна выборка истории: рантайм считает
// price как close(t)/close(t-P)-1, а дельты - как объём за последние P против
// предыдущих P, то есть и то и другое равно отношению соседних свечей таблицы
// candles_{period}. Поэтому история восстанавливается один-в-один, без
// приближений, и выборки идут с шагом ровно P - как и при троттлинге
// nextSampleAt в рантайме.
//
// Без сидирования детектор на 4h ждал бы боевой готовности несколько суток, а
// каждый рестарт бота обнулял бы накопленное.
func (s *AnomalyStrategy) seedHistory() {
	now := time.Now()

	for period, duration := range s.Periods {
		periodCfg, ok := s.Config.Periods[period]
		if !ok || !periodCfg.Enabled || duration <= 0 {
			continue
		}

		// window выборок требуют window+1 свечу: каждая выборка - отношение соседних
		window := s.windowSizeForPeriod(period)
		from := now.Add(-time.Duration(window+1) * duration)

		candles, err := s.AssetsPrices.GetPeriodCandles(period, from)
		if err != nil {
			anomalyLogger.Errorf("не удалось прочитать свечи %s для сидирования истории: %v", period, err)
			continue
		}

		byPair := make(map[string][]exModel.Candle, len(s.Config.Pairs))
		for _, candle := range candles {
			if _, tracked := s.history[candle.Pair]; !tracked {
				continue
			}
			byPair[candle.Pair] = append(byPair[candle.Pair], candle)
		}

		seededPairs := 0
		samplesPerPair := 0

		for pair, pairCandles := range byPair {
			sort.Slice(pairCandles, func(i, j int) bool {
				return pairCandles[i].Time.Before(pairCandles[j].Time)
			})

			added := s.seedPairPeriod(pair, period, duration, pairCandles)
			if added == 0 {
				continue
			}
			seededPairs++
			// Пары набирают одинаковое число выборок (свечи пишутся всем разом),
			// поэтому для отчёта достаточно минимума - он и определяет, заработает
			// период или нет.
			if samplesPerPair == 0 || added < samplesPerPair {
				samplesPerPair = added
			}
		}

		s.reportSeeding(period, duration, seededPairs, samplesPerPair)
	}
}

// reportSeeding честно сообщает, заработает период или нет.
//
// Раньше здесь было просто "засеяна: N пар" - и это вводило в заблуждение: пара
// считалась засеянной, если в неё попала ХОТЬ ОДНА выборка. Периоды 4h и 1d так
// и молчали сутками, а лог рапортовал об успехе.
func (s *AnomalyStrategy) reportSeeding(period string, duration time.Duration, pairs, samples int) {
	required := s.requiredSamples(period)

	if pairs == 0 {
		anomalyLogger.Warnf("история %s НЕ засеяна: в БД нет свечей этого периода — период не работает", period)
		return
	}

	if samples < required {
		// Сколько ещё ждать: не хватает (required - samples) свечей, каждая
		// набирается за duration.
		wait := time.Duration(required-samples) * duration
		anomalyLogger.Warnf(
			"история %s засеяна из БД: %d пар, но выборок только %d из %d — ПЕРИОД НЕ РАБОТАЕТ (z-score не считается), нужно ещё ~%s данных",
			period, pairs, samples, required, wait.Round(time.Hour))
		return
	}

	anomalyLogger.Infof("история %s засеяна из БД: %d пар, выборок %d (минимум %d) — период работает",
		period, pairs, samples, required)
}

// requiredSamples - сколько значений должно быть в буфере, чтобы z-score
// вообще начал считаться (см. MetricRecord.minSamples и его клампинг по окну).
func (s *AnomalyStrategy) requiredSamples(period string) int {
	required := s.Config.MinSamples
	if window := s.windowSizeForPeriod(period); required > window {
		required = window
	}
	if required < 2 {
		required = 2
	}
	return required
}

// seedPairPeriod засеивает историю одной пары за один период.
// Возвращает число добавленных выборок: по нему видно, наберётся ли minSamples,
// то есть заработает ли период вообще.
func (s *AnomalyStrategy) seedPairPeriod(pair, period string, duration time.Duration, candles []exModel.Candle) int {
	metrics := s.getActiveMetricNames()
	added := 0

	for i := 1; i < len(candles); i++ {
		prev, current := candles[i-1], candles[i]

		// Пропуск в данных: отношение через дырку - это уже другой горизонт,
		// такую выборку в историю не берём, иначе она раздует разброс.
		gap := current.Time.Sub(prev.Time)
		if gap < duration/2 || gap > duration*3/2 {
			continue
		}

		for _, metricName := range metrics {
			value, ok := candleMetricValue(metricName, prev, current)
			if !ok {
				continue
			}
			record := s.history[pair][period][metricName]
			if record == nil {
				continue
			}
			record.Add(statValue(metricName, value))
		}
		added++
	}

	if added == 0 {
		return 0
	}

	// Последняя засеянная выборка - это последняя завершённая свеча, поэтому
	// следующее значение пишем в историю не раньше чем через период: иначе
	// текущее (сильно перекрывающееся с ней) окно попало бы в буфер сразу.
	if state := s.states[pair][period]; state != nil {
		state.nextSampleAt = time.Now().Add(duration)
	}

	return added
}

// candleMetricValue считает значение метрики как отношение соседних свечей
// периода - в тех же процентах и с той же семантикой, что checkValuesDividing
// в пакете prices (ноль в любой из частей означает "изменения нет").
func candleMetricValue(metric string, prev, current exModel.Candle) (float64, bool) {
	switch metric {
	case "price":
		return changePercent(current.Close, prev.Close), true
	case "volume":
		return changePercent(current.Volume, prev.Volume), true
	case "volumeBuy":
		return changePercent(current.ActiveBuyVolume, prev.ActiveBuyVolume), true
	case "volumeAsk":
		return changePercent(current.ActiveAskVolume, prev.ActiveAskVolume), true
	case "trades":
		return changePercent(float64(current.AmountTrade), float64(prev.AmountTrade)), true
	case "tradesBuy":
		return changePercent(float64(current.AmountTradeBuy), float64(prev.AmountTradeBuy)), true
	case "tradesAsk":
		return changePercent(float64(current.AmountTradeAsk), float64(prev.AmountTradeAsk)), true
	}
	return 0, false
}

func changePercent(current, previous float64) float64 {
	if current == 0 || previous == 0 {
		return 0
	}
	return current/previous*100 - 100
}

// windowSizeForPeriod возвращает размер окна истории для периода: если для
// периода задан собственный historyWindowSize в конфиге - используется он,
// иначе - глобальный Config.HistoryWindowSize. Это нужно, потому что теперь
// (после фикса автокорреляции) в историю пишется одно значение за period,
// и для длинных периодов (1d) глобальное окно в 30 означало бы 30 суток
// до первого срабатывания детектора.
func (s *AnomalyStrategy) windowSizeForPeriod(period string) int {
	if pc, ok := s.Config.Periods[period]; ok && pc.HistoryWindowSize > 0 {
		return pc.HistoryWindowSize
	}
	return s.Config.HistoryWindowSize
}

// cooldownForPeriod - cooldown равен реальной длительности периода, взятой из
// основного конфига (s.Periods): собственная таблица длительностей здесь
// разошлась бы с тем, что стратегия использует для nextSampleAt.
func (s *AnomalyStrategy) cooldownForPeriod(period string) time.Duration {
	return s.Periods[period]
}

func (s *AnomalyStrategy) initMetricsForPair(pair, period string, windowSize int) {
	metrics := s.getActiveMetricNames()
	for _, metricName := range metrics {
		s.history[pair][period][metricName] = NewMetricRecord(windowSize, s.Config.MinSamples, s.Config.MinScale[metricName])
	}
}

// minChangeFor - порог экономической значимости метрики в сырых процентах
func (s *AnomalyStrategy) minChangeFor(metric string) float64 {
	return s.Config.MinChange[metric]
}

// liquidPairs отбирает пары с достаточным суточным оборотом.
// MarketsStat.Volume - это QuoteVolume за 24ч, то есть оборот сразу в USDT.
func (s *AnomalyStrategy) liquidPairs() []string {
	if s.Config.MinDailyVolume <= 0 {
		return s.Config.Pairs
	}

	pairs := make([]string, 0, len(s.Config.Pairs))
	for _, pair := range s.Config.Pairs {
		stat, err := s.AssetsPrices.GetMarketsStatForPair(pair)
		if err != nil {
			continue
		}
		if stat.Volume >= s.Config.MinDailyVolume {
			pairs = append(pairs, pair)
		}
	}
	return pairs
}

func (s *AnomalyStrategy) getActiveMetricNames() []string {
	names := make([]string, 0, 7)
	if s.Config.Metrics.Price {
		names = append(names, "price")
	}
	if s.Config.Metrics.Volume {
		names = append(names, "volume")
	}
	if s.Config.Metrics.VolumeBuy {
		names = append(names, "volumeBuy")
	}
	if s.Config.Metrics.VolumeAsk {
		names = append(names, "volumeAsk")
	}
	if s.Config.Metrics.Trades {
		names = append(names, "trades")
	}
	if s.Config.Metrics.TradesBuy {
		names = append(names, "tradesBuy")
	}
	if s.Config.Metrics.TradesAsk {
		names = append(names, "tradesAsk")
	}
	// Сортируем, чтобы порядок обхода метрик (и, как следствие, порядок
	// в уведомлениях) был детерминированным.
	sort.Strings(names)
	return names
}

// needsDelta сообщает, берётся ли метрика из ChangeDelta (а не из ChangePrices)
func needsDelta(metric string) bool {
	return metric != "price"
}

// statValue переводит сырое значение метрики в шкалу, в которой z-score
// осмысленен, и в которой хранится история.
//
// Дельты (volume*/trades*) - это отношение "новое окно / старое окно" в
// процентах: снизу оно ограничено -100%, а сверху не ограничено ничем. Такое
// распределение сильно скошено вправо, и z по нему систематически завышает
// положительные аномалии и занижает отрицательные (падение объёма вдвое = -50%,
// рост вдвое = +100%, хотя по сути это одинаковые по силе события). Логарифм
// отношения делает метрику симметричной: -0.69 и +0.69 соответственно.
//
// Цена (price) - это уже % изменение за период, оно симметрично, и его
// оставляем как есть, чтобы не терять привычную интерпретацию.
func statValue(metric string, value float64) float64 {
	if !needsDelta(metric) {
		return value
	}

	ratio := 1 + value/100
	// checkValuesDividing отдаёт 0, если одна из сторон нулевая, так что ratio<=0
	// на практике не встречается - но log(<=0) даёт -Inf, поэтому подстрахуемся.
	if ratio <= 0 {
		return 0
	}
	return math.Log(ratio)
}

// metricSource - ОТКУДА берутся сырые значения метрик за период.
//
// Это единственное место, где детектор касается внешнего мира. В бою источник -
// скользящие окна AssetsPrices; в бэктесте - пара соседних свечей из БД. Всё
// остальное (история, MAD, z-score, классификация, подтверждение, cooldown)
// работает поверх и о разнице не знает.
//
// Без этого шва бэктестер был бы вынужден держать собственную копию правил
// детекции - и она неизбежно разошлась бы с боевой, а мы бы об этом не узнали.
type metricSource interface {
	value(metric string) (float64, bool)
}

// liveSource - боевой источник: снимки ChangePrices/ChangeDelta.
type liveSource struct {
	cp prices.ChangePrices
	cd prices.ChangeDelta
}

func (s liveSource) value(metric string) (float64, bool) {
	return extractMetricValue(metric, s.cp, s.cd)
}

// candleSource - источник бэктеста: отношение соседних свечей периода.
//
// Это ровно та же величина, что считает рантайм: price = close(t)/close(t-P)-1,
// дельта объёма = объём за последние P против предыдущих P. Свеча периода и есть
// одна выборка - на этом же тождестве построено сидирование истории из БД.
type candleSource struct {
	prev, current exModel.Candle
}

func (s candleSource) value(metric string) (float64, bool) {
	return candleMetricValue(metric, s.prev, s.current)
}

// extractMetricValue извлекает значение метрики из уже прочитанных снимков
// ChangePrices/ChangeDelta
func extractMetricValue(metric string, cp prices.ChangePrices, cd prices.ChangeDelta) (float64, bool) {
	switch metric {
	case "price":
		return cp.ChangePercent, true
	case "volume":
		return cd.Volume, true
	case "volumeBuy":
		return cd.VolumeBuy, true
	case "volumeAsk":
		return cd.VolumeAsk, true
	case "trades":
		return cd.Trades, true
	case "tradesBuy":
		return cd.TradesBuy, true
	case "tradesAsk":
		return cd.TradesAsk, true
	}
	return 0, false
}

// determineLevelForMetrics вычисляет максимальный уровень аномалии для всех метрик.
// commitToHistory управляет тем, добавляется ли текущее значение в MetricRecord:
// z-score считается ВСЕГДА (проверка происходит каждую минуту), но запись в
// историю разрешена не чаще одного раза за period - иначе буфер набивается
// почти идентичными перекрывающимися окнами и статистика ломается (см.
// комментарий у PeriodState.nextSampleAt).
func (s *AnomalyStrategy) determineLevelForMetrics(pair, period string, thresholds *ThresholdsConfig, commitToHistory bool) *AnomalyResult {
	src, ok := s.liveSource(pair, period)
	if !ok {
		return nil
	}
	return s.detect(pair, period, thresholds, commitToHistory, src)
}

// liveSource читает снимки источников под локами AssetsPrices и требует
// заполненности только тех датасетов, которые реально нужны активным метрикам:
// дельта набирается вдвое дольше цены (2*period), и ждать её ради одной лишь
// метрики price было бы незачем.
func (s *AnomalyStrategy) liveSource(pair, period string) (metricSource, bool) {
	var src liveSource

	for _, metricName := range s.getActiveMetricNames() {
		if needsDelta(metricName) {
			var ok bool
			if src.cd, ok = s.AssetsPrices.GetChangeDelta(pair, period); !ok {
				return nil, false
			}
			break
		}
	}

	if s.Config.Metrics.Price {
		var ok bool
		if src.cp, ok = s.AssetsPrices.GetChangePrices(pair, period); !ok {
			return nil, false
		}
	}

	return src, true
}

// detect - ЯДРО ДЕТЕКТОРА, единственное для боя и для бэктеста.
//
// Всё, что отличает бэктест от боя, спрятано в metricSource. Здесь - история,
// z-score, классификация: то, что обязано быть одинаковым, иначе бэктест меряет
// не ту стратегию, которая торгует.
func (s *AnomalyStrategy) detect(pair, period string, thresholds *ThresholdsConfig, commitToHistory bool, src metricSource) *AnomalyResult {
	metrics := s.getActiveMetricNames()
	if len(metrics) == 0 || thresholds == nil {
		return nil
	}

	result := &AnomalyResult{
		Pair:    pair,
		Period:  period,
		Level:   0,
		Metrics: make(map[string]MetricAnomaly),
	}

	// activityZ - насколько торговая активность выше обычной. Берём МАКСИМАЛЬНЫЙ
	// z среди метрик семейства (volume*/trades*): они сильно скоррелированы, и
	// смысл у них один - "торгуют больше, чем обычно".
	//
	// Здесь нужен СЫРОЙ z, не прошедший через minChange: порог значимости для
	// объёмов задан как "изменился хотя бы вдвое", то есть ни одна отрицательная
	// дельта его никогда не пройдёт (упасть на 100% объём не может). Гоняя
	// activityZ через тот же фильтр, мы бы навсегда ослепли к падению активности -
	// а это ровно то, что нужно для детекции рассогласования.
	activityZ := 0.0
	hasActivity := false

	for _, metricName := range metrics {
		value, ok := src.value(metricName)
		if !ok {
			continue
		}

		record, exists := s.history[pair][period][metricName]
		if !exists {
			continue
		}

		// В истории и в z-score живут преобразованные значения (см. statValue),
		// а в уведомление уходит сырое - его читает человек.
		//
		// ВАЖНО: считаем z-score ДО добавления значения в историю. Иначе новое
		// (потенциально аномальное) значение попадает в собственную базу и
		// занижает свой же z-score.
		stat := statValue(metricName, value)
		zScore := record.ZScore(stat)
		if commitToHistory {
			record.Add(stat)
		}

		if math.IsNaN(zScore) || math.IsInf(zScore, 0) {
			zScore = 0
		}

		// Уровень отдельной метрики нужен только для отчёта: что именно
		// сработало, видно в уведомлении и в логе. Итоговый уровень события
		// решает classify - по цене или по активности, смотря что это за событие.
		metricLevel := s.metricLevel(metricName, value, zScore, thresholds)

		result.Metrics[metricName] = MetricAnomaly{
			Value:     value,
			ZScore:    zScore,
			Threshold: thresholdForLevel(metricLevel, thresholds),
			IsAnomaly: metricLevel > 0,
		}

		if metricFamily(metricName) == familyActivity {
			if !hasActivity || zScore > activityZ {
				activityZ = zScore
			}
			hasActivity = true
		}
	}

	// Цену выносим в отдельные поля: она задаёт направление сигнала, и её нельзя
	// путать с остальными метриками, которые направления не имеют.
	if price, ok := result.Metrics["price"]; ok {
		result.HasPrice = true
		result.PriceChange = price.Value
		result.PriceZ = price.ZScore
	}
	result.ActivityZ = activityZ
	result.HasActivity = hasActivity

	s.classify(result, thresholds)
	return result
}

// divergenceZ - порог рассогласования: цена сходила, а активность была НИЖЕ
// обычной хотя бы на столько сигм. Не настраивается: это не ручка стратегии,
// а определение "движения по пустому стакану" для пометки в дайджесте.
const divergenceZ = 1.0

// classify решает, есть ли событие и насколько оно сильное.
//
// Уровень задаёт ЦЕНА, и только она. Раньше здесь было level = max по всем
// метрикам, а composite = z метрики с максимальным |z|. Это худшая из возможных
// агрегаций: метрики volume, volumeBuy, trades, tradesBuy почти полностью
// скоррелированы (выросла активность - выросло всё), поэтому max по ним означает
// "берём самую шумную". Уровень 3 можно было получить на одном лишь объёме при
// стоящей на месте цене - и такой сигнал уходил исполнителю как полноценный.
//
// Активность уровень не меняет, но дополняет картину: движение при упавшей
// активности помечается Divergent - оно прошло по пустому стакану и чаще всего
// откатывается. Пометка уходит в дайджест и лог, решение по ней - за человеком.
func (s *AnomalyStrategy) classify(result *AnomalyResult, thresholds *ThresholdsConfig) {
	priceLevel := 0
	if result.HasPrice {
		priceLevel = s.metricLevel("price", result.PriceChange, result.PriceZ, thresholds)
	}

	if priceLevel == 0 {
		result.Level = 0
		result.IsAnomalous = false
		return
	}

	result.CompositeZ = result.PriceZ
	result.Level = priceLevel
	result.IsAnomalous = true
	result.Divergent = result.HasActivity && result.ActivityZ <= -divergenceZ
}

// metricLevel - уровень метрики с учётом порога экономической значимости.
//
// Z-score меряет статистическую неожиданность, а не силу события: у стейблкоинов
// и низковолатильных пар разброс близок к нулю, поэтому движение на 0.01% честно
// даёт z=5. Торговать там нечего.
func (s *AnomalyStrategy) metricLevel(metric string, value, zScore float64, thresholds *ThresholdsConfig) int {
	if math.Abs(value) < s.minChangeFor(metric) {
		return 0
	}
	return LevelFromZScore(zScore, thresholds)
}

// checkAndNotify проверяет все пары и уведомляет об аномалиях
func (s *AnomalyStrategy) checkAndNotify() {
	if !s.StrategyEnable.Get() {
		return
	}

	now := time.Now()

	// allAnomalousResults - ВСЕ пары/периоды, где реально есть аномалия,
	// вне зависимости от cooldown. Используется для рыночной метрики,
	// чтобы процент аномальных пар не занижался из-за подавленных
	// cooldown'ом персональных уведомлений.
	allAnomalousResults := make([]*AnomalyResult, 0)

	// notifyResults - подмножество, которое реально должно быть отправлено
	// пользователю как персональное уведомление по паре (с учётом cooldown).
	notifyResults := make([]*AnomalyResult, 0)

	// Неликвид отсекаем до всех расчётов: там дельта объёма даёт +87000% просто
	// потому, что в предыдущем окне оборота почти не было.
	pairs := s.liquidPairs()

	s.mu.Lock()

	for _, pair := range pairs {
		for period := range s.Periods {
			periodCfg, ok := s.Config.Periods[period]
			if !ok || !periodCfg.Enabled {
				continue
			}

			state := s.states[pair][period]
			if state == nil {
				continue
			}

			// Пишем в историю не чаще раза в period (защита от автокорреляции
			// скользящих окон ChangePrices/ChangeDelta, которые сами
			// пересчитываются каждую минуту). Проверка (z-score) всё равно
			// идёт каждую минуту - только запись в буфер троттлится.
			periodDuration := s.Periods[period]
			commitToHistory := !now.Before(state.nextSampleAt)

			result := s.determineLevelForMetrics(pair, period, &periodCfg.Thresholds, commitToHistory)
			if result == nil {
				continue
			}

			if commitToHistory {
				state.nextSampleAt = now.Add(periodDuration)
			}

			if !result.IsAnomalous {
				// Аномалия прервалась - окно подтверждения начинается заново.
				// Именно так и отсеивается секундный дёрг рынка: он не переживает
				// следующую проверку.
				state.pendingChecks = 0

				// Сбрасывать lastLevel можно ТОЛЬКО после истечения cooldown.
				// Иначе метрика, болтающаяся вокруг порога, шлёт уведомление
				// каждую минуту: на "спокойной" минуте lastLevel обнулялся, а на
				// следующей правило эскалации (level > lastLevel) видело 1 > 0 и
				// пропускало алерт мимо ещё активного cooldown.
				if !now.Before(state.cooldownUntil) {
					state.lastLevel = 0
				}
				continue
			}

			allAnomalousResults = append(allAnomalousResults, result)

			// Неподтверждённая аномалия в рыночную метрику попадает (она там для
			// того и нужна - показать, что творится со всем рынком), но сигналом
			// и уведомлением ещё не становится.
			if !confirm(state, result, periodCfg.ConfirmChecks) {
				continue
			}

			// Уровни ниже minNotifyLevel в дайджест не идут и cooldown не тратят,
			// но в рыночной метрике (allAnomalousResults) продолжают учитываться.
			if result.Level < s.Config.MinNotifyLevel {
				continue
			}

			// Если cooldown еще активен
			if now.Before(state.cooldownUntil) {
				// Если новый уровень выше последнего отправленного — отправляем и сбрасываем cooldown
				if result.Level > state.lastLevel {
					state.lastLevel = result.Level
					state.cooldownUntil = now.Add(s.cooldownForPeriod(period))
					notifyResults = append(notifyResults, result)
				}
				continue
			}

			// Cooldown истек или не был установлен — отправляем
			state.lastLevel = result.Level
			state.cooldownUntil = now.Add(s.cooldownForPeriod(period))
			notifyResults = append(notifyResults, result)
		}
	}

	s.mu.Unlock()

	// В лог пишем ВСЁ, что нашли, - включая подавленное cooldown'ом. Telegram и
	// лог решают разные задачи: там читаемая сводка для человека, здесь полная
	// картина, по которой потом можно разобраться, что происходило.
	if s.Config.LogAnomalies {
		s.logAnomalies(allAnomalousResults, notifyResults)
	}

	// Все аномалии одного тика уходят ОДНИМ дайджестом. Отдельным сообщением на
	// пару это нечитаемо: на рыночном движении сотни пар аномальны одновременно.
	if s.NotificationEnable.Get() && len(notifyResults) > 0 {
		s.NotificationDigest(notifyResults)
	}

	// Исполнителям отдаём ТЕ ЖЕ результаты, что и в Telegram: они уже прошли
	// через cooldown, то есть это "новая или усилившаяся" аномалия, а не одно и
	// то же событие каждую минуту.
	s.publish(notifyResults)

	// Рыночная метрика считается по ВСЕМ аномальным парам, а не только
	// по тем, что прошли через cooldown-фильтр персональных уведомлений.
	if s.Config.MarketMetric.Enabled && len(allAnomalousResults) > 0 {
		s.checkMarketAnomaly(allAnomalousResults, len(pairs))
	}
}

// checkMarketAnomaly проверяет и уведомляет о рыночной аномалии.
// totalPairs - число реально проверяемых (ликвидных) пар: именно оно должно быть
// знаменателем, иначе доля аномальных занижается на отфильтрованном неликвиде.
func (s *AnomalyStrategy) checkMarketAnomaly(anomalousResults []*AnomalyResult, totalPairs int) {
	if totalPairs < s.Config.MarketMetric.MinPairs {
		return
	}

	// Группируем по периодам
	anomalyByPeriod := make(map[string]int)
	topByPeriod := make(map[string][]AnomalyResult)

	for _, result := range anomalousResults {
		anomalyByPeriod[result.Period]++
		topByPeriod[result.Period] = append(topByPeriod[result.Period], *result)
	}

	for period := range s.Periods {
		periodCfg, ok := s.Config.Periods[period]
		if !ok || !periodCfg.Enabled {
			continue
		}

		anomalous := anomalyByPeriod[period]
		percent := float64(anomalous) / float64(totalPairs) * 100

		// Сортируем топ по композитному z-score
		results := topByPeriod[period]
		sort.Slice(results, func(i, j int) bool {
			return math.Abs(results[i].CompositeZ) > math.Abs(results[j].CompositeZ)
		})
		if len(results) > 3 {
			results = results[:3]
		}

		if percent < s.Config.MarketMetric.AnomalyPercentThreshold || !s.NotificationEnable.Get() {
			continue
		}

		s.mu.Lock()
		marketState, ok := s.marketStates[period]
		if !ok {
			s.mu.Unlock()
			continue
		}
		now := time.Now()
		if now.Before(marketState.cooldownUntil) {
			s.mu.Unlock()
			continue
		}
		marketState.cooldownUntil = now.Add(s.cooldownForPeriod(period))
		s.mu.Unlock()

		s.NotificationMarketAnomaly(period, percent, anomalous, totalPairs, results)
	}
}

// Start запускает стратегию
func (s *AnomalyStrategy) Start(ctx context.Context) error {
	updates := s.AssetsPrices.Subscribe()

	for {
		select {
		case <-updates:
			s.checkAndNotify()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// GetTelegramMenu возвращает меню для телеграма
func (s *AnomalyStrategy) GetTelegramMenu() model.WindowHandler {
	return s.TelegramMenu
}

// OnMarket получает рыночные данные
func (s *AnomalyStrategy) OnMarket(ms exModel.MarketsStat) {
	// Данные уже обрабатываются в AssetsPrices
}
