package anomaly

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
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
	if scale == 0 {
		return 0
	}

	return (value - med) / scale
}

// AnomalyResult результат проверки одной пары за один период
type AnomalyResult struct {
	Pair        string
	Period      string
	Level       int // 1, 2, 3
	Metrics     map[string]MetricAnomaly
	CompositeZ  float64
	IsAnomalous bool
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

	mu sync.RWMutex
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
		Config:       cfg,
		AssetsPrices: assetsPrices,
		Periods:      periods,
		Notification: notify,
		history:      make(map[string]map[string]map[string]*MetricRecord),
		states:       make(map[string]map[string]*PeriodState),
		marketStates: make(map[string]*PeriodState),
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

	if cfg.StrategyEnable {
		str.seedHistory()
	}

	return str, nil
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
		for pair, pairCandles := range byPair {
			sort.Slice(pairCandles, func(i, j int) bool {
				return pairCandles[i].Time.Before(pairCandles[j].Time)
			})
			if s.seedPairPeriod(pair, period, duration, pairCandles) {
				seededPairs++
			}
		}

		anomalyLogger.Infof("история %s засеяна из БД: %d пар", period, seededPairs)
	}
}

// seedPairPeriod засеивает историю одной пары за один период. Возвращает true,
// если удалось добавить хотя бы одну выборку.
func (s *AnomalyStrategy) seedPairPeriod(pair, period string, duration time.Duration, candles []exModel.Candle) bool {
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
		return false
	}

	// Последняя засеянная выборка - это последняя завершённая свеча, поэтому
	// следующее значение пишем в историю не раньше чем через период: иначе
	// текущее (сильно перекрывающееся с ней) окно попало бы в буфер сразу.
	if state := s.states[pair][period]; state != nil {
		state.nextSampleAt = time.Now().Add(duration)
	}

	return true
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
// основного конфига (s.Periods). Держать здесь собственную таблицу "1d -> 24h"
// нельзя: в конфиге приложения период "1d" задан как 12 часов, и таблица
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

	// Читаем снимки источников под локами AssetsPrices и требуем заполненности
	// только тех датасетов, которые реально нужны активным метрикам: дельта
	// набирается вдвое дольше цены (2*period), и ждать её ради одной лишь
	// метрики price было бы незачем.
	var (
		cp prices.ChangePrices
		cd prices.ChangeDelta
	)
	for _, metricName := range metrics {
		if needsDelta(metricName) {
			var ok bool
			if cd, ok = s.AssetsPrices.GetChangeDelta(pair, period); !ok {
				return nil
			}
			break
		}
	}
	if s.Config.Metrics.Price {
		var ok bool
		if cp, ok = s.AssetsPrices.GetChangePrices(pair, period); !ok {
			return nil
		}
	}

	maxZ := 0.0
	maxLevel := 0

	for _, metricName := range metrics {
		value, ok := extractMetricValue(metricName, cp, cd)
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

		metricLevel := LevelFromZScore(zScore, thresholds)

		// Порог экономической значимости. Z-score меряет статистическую
		// неожиданность, а не силу события: у стейблкоинов и низковолатильных
		// пар разброс близок к нулю, поэтому движение на 0.01% честно даёт z=5.
		// Торговать там нечего, поэтому такие срабатывания гасим независимо от z.
		if math.Abs(value) < s.minChangeFor(metricName) {
			metricLevel = 0
		}

		isAnomaly := metricLevel > 0

		result.Metrics[metricName] = MetricAnomaly{
			Value:     value,
			ZScore:    zScore,
			Threshold: thresholdForLevel(metricLevel, thresholds),
			IsAnomaly: isAnomaly,
		}

		// В composite берём z только тех метрик, что прошли порог значимости,
		// иначе в заголовок уведомления попадёт z от отфильтрованного шума.
		if isAnomaly && math.Abs(zScore) > math.Abs(maxZ) {
			maxZ = zScore
		}
		if metricLevel > maxLevel {
			maxLevel = metricLevel
		}
	}

	result.CompositeZ = maxZ
	result.Level = maxLevel
	result.IsAnomalous = maxLevel > 0
	return result
}

// checkAndNotify проверяет все пары и уведомляет об аномалиях
func (s *AnomalyStrategy) checkAndNotify() {
	if !s.Config.StrategyEnable {
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

	// Все аномалии одного тика уходят ОДНИМ дайджестом. Отдельным сообщением на
	// пару это нечитаемо: на рыночном движении сотни пар аномальны одновременно.
	if s.Config.NotificationEnable && len(notifyResults) > 0 {
		s.NotificationDigest(notifyResults)
	}

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

		if percent < s.Config.MarketMetric.AnomalyPercentThreshold || !s.Config.NotificationEnable {
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

// GetTelegramMenu возвращает меню для телеграма (опционально)
func (s *AnomalyStrategy) GetTelegramMenu() model.WindowHandler {
	return nil
}

// OnMarket получает рыночные данные
func (s *AnomalyStrategy) OnMarket(ms exModel.MarketsStat) {
	// Данные уже обрабатываются в AssetsPrices
}
