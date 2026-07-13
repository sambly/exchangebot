package anomaly

import (
	"math"
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
)

type stubPricesRepo struct{}

func (stubPricesRepo) SelectMarketStateTimev2(time.Time) ([]exModel.Candle, error) {
	return nil, nil
}
func (stubPricesRepo) SelectDeltaPeriod(string, string) ([]model.ChangeDeltaForCandle, error) {
	return nil, nil
}
func (stubPricesRepo) SelectCandlesFromPeriod(string, time.Time) ([]exModel.Candle, error) {
	return nil, nil
}

func filledRecord(t *testing.T, values []float64, minSamples int, scaleFloor float64) *MetricRecord {
	t.Helper()

	rec := NewMetricRecord(len(values)+10, minSamples, scaleFloor)
	for _, v := range values {
		rec.Add(v)
	}
	return rec
}

// Пока в буфере меньше minSamples значений, z-score не считается вообще.
// Без этого порога разброс оценивается по шуму: два случайно близких значения
// дают крошечный MAD, и следующее рядовое значение получает z в десятки.
func TestZScoreRequiresMinSamples(t *testing.T) {
	rec := NewMetricRecord(30, 20, 0)

	for i := 0; i < 19; i++ {
		rec.Add(float64(i%3) * 0.1)
		if z := rec.ZScore(100); z != 0 {
			t.Fatalf("при %d значениях z-score должен быть 0, получено %v", rec.Len(), z)
		}
	}

	rec.Add(0.1)
	if z := rec.ZScore(100); z == 0 {
		t.Fatal("при достижении minSamples z-score должен считаться")
	}
}

// Медиана и MAD устойчивы к выбросам: прошлый всплеск, осевший в буфере, не
// должен "ослеплять" детектор к следующему такому же. На mean/stddev он бы
// раздул разброс, и вторая волна аномалии не прошла бы порог.
func TestZScoreRobustToOutlierInHistory(t *testing.T) {
	values := make([]float64, 0, 30)
	for i := 0; i < 30; i++ {
		values = append(values, 0.1*float64(i%3))
	}
	rec := filledRecord(t, values, 20, 0)

	before := rec.ZScore(10)
	rec.Add(10) // выброс попал в историю
	after := rec.ZScore(10)

	if math.Abs(before-after) > 0.01*math.Abs(before) {
		t.Fatalf("выброс в истории изменил z-score: было %.2f, стало %.2f", before, after)
	}
}

// Пол разброса гасит стейблкоины: у них цена почти не движется, MAD вырождается,
// и движение на 0.01%% честно даёт z=6. Торговать там нечего.
func TestScaleFloorSuppressesFlatSeries(t *testing.T) {
	values := make([]float64, 0, 30)
	for i := 0; i < 30; i++ {
		values = append(values, 0.001*float64(i%3)) // дрейф в тысячные доли процента
	}

	noFloor := filledRecord(t, values, 20, 0)
	withFloor := filledRecord(t, values, 20, 0.35)

	if z := noFloor.ZScore(0.01); math.Abs(z) < 3 {
		t.Fatalf("без пола разброса шум стейблкоина должен давать большой z, получено %.2f", z)
	}
	if z := withFloor.ZScore(0.01); math.Abs(z) >= 3 {
		t.Fatalf("с полом разброса шум 0.01%% не должен быть аномалией, получено z=%.2f", z)
	}
	if z := withFloor.ZScore(3.0); math.Abs(z) < 4 {
		t.Fatalf("настоящее движение 3%% должно оставаться аномалией, получено z=%.2f", z)
	}
}

// MAD вырождается в ноль, если больше половины значений одинаковы.
// Тогда должен использоваться запасной разброс, а не деление на ноль.
func TestZScoreZeroMADFallback(t *testing.T) {
	values := make([]float64, 0, 30)
	for i := 0; i < 25; i++ {
		values = append(values, 0) // объём стабильно нулевой
	}
	for i := 0; i < 5; i++ {
		values = append(values, 1)
	}
	rec := filledRecord(t, values, 20, 0)

	z := rec.ZScore(10)
	if math.IsNaN(z) || math.IsInf(z, 0) {
		t.Fatalf("z-score выродился: %v", z)
	}
	if z == 0 {
		t.Fatal("при вырожденном MAD должен работать запасной разброс, а не нулевой z")
	}
}

// Дельты - это отношение окон в процентах: снизу -100%, сверху без границы.
// Логарифм делает метрику симметричной: рост вдвое и падение вдвое должны быть
// событиями одной силы.
func TestStatValueLogTransform(t *testing.T) {
	// price не логарифмируется - он уже симметричен
	if got := statValue("price", 5); got != 5 {
		t.Errorf("price не должен преобразовываться: получено %v", got)
	}

	doubled := statValue("volume", 100) // объём вырос вдвое
	halved := statValue("volume", -50)  // объём упал вдвое
	unchanged := statValue("volume", 0) // без изменений

	if unchanged != 0 {
		t.Errorf("нулевая дельта должна давать 0, получено %v", unchanged)
	}
	if math.Abs(doubled+halved) > 1e-9 {
		t.Errorf("рост и падение вдвое должны быть симметричны: %+.4f и %+.4f", doubled, halved)
	}
	// Отношение <= 0 невозможно по построению метрики, но не должно давать -Inf
	if got := statValue("volume", -100); math.IsInf(got, 0) || math.IsNaN(got) {
		t.Errorf("падение на 100%% не должно давать %v", got)
	}
}

func TestLevelFromZScore(t *testing.T) {
	thresholds := &ThresholdsConfig{Level1: 4, Level2: 6, Level3: 8}

	cases := []struct {
		z    float64
		want int
	}{
		{3.9, 0},
		{4.0, 1},
		{5.9, 1},
		{6.0, 2},
		{8.0, 3},
		{-9.0, 3}, // уровень определяется модулем: падение - тоже аномалия
	}

	for _, c := range cases {
		if got := LevelFromZScore(c.z, thresholds); got != c.want {
			t.Errorf("LevelFromZScore(%v) = %d, ожидалось %d", c.z, got, c.want)
		}
	}
}

func testStrategy(pairs []string, periods map[string]time.Duration) *AnomalyStrategy {
	cfg := &Config{
		StrategyEnable: true,
		Pairs:          pairs,
		// Только цена: метрики объёма требуют заполненного дельта-датасета,
		// а он набирается вдвое дольше (2 x period) - в юнит-тесте это лишнее.
		Metrics:    MetricsConfig{Price: true},
		MinSamples: 2,
		MinChange:      map[string]float64{"price": 1.0, "volume": 100.0},
		MinScale:       map[string]float64{"price": 0.35, "volume": 0.25},
		Periods:        map[string]*PeriodConfig{},
	}
	for period := range periods {
		cfg.Periods[period] = &PeriodConfig{
			Enabled:           true,
			HistoryWindowSize: 30,
			Thresholds:        ThresholdsConfig{Level1: 4, Level2: 6, Level3: 8},
		}
	}

	str := &AnomalyStrategy{
		Config:       cfg,
		Periods:      periods,
		history:      make(map[string]map[string]map[string]*MetricRecord),
		states:       make(map[string]map[string]*PeriodState),
		marketStates: make(map[string]*PeriodState),
	}

	for period := range periods {
		str.marketStates[period] = &PeriodState{}
	}
	for _, pair := range pairs {
		str.history[pair] = make(map[string]map[string]*MetricRecord)
		str.states[pair] = make(map[string]*PeriodState)
		for period := range periods {
			str.history[pair][period] = make(map[string]*MetricRecord)
			str.states[pair][period] = &PeriodState{}
			str.initMetricsForPair(pair, period, 30)
		}
	}
	return str
}

// Свеча периода = одна выборка истории: метрика считается как отношение
// соседних свечей. Разрыв в данных такую выборку обесценивает - её надо
// пропускать, иначе она раздует разброс.
func TestSeedPairPeriodSkipsGaps(t *testing.T) {
	period := 15 * time.Minute
	str := testStrategy([]string{"BTCUSDT"}, map[string]time.Duration{"15m": period})

	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	candles := []exModel.Candle{
		{Pair: "BTCUSDT", Time: base, Close: 100, Volume: 1000},
		{Pair: "BTCUSDT", Time: base.Add(period), Close: 101, Volume: 1100},
		{Pair: "BTCUSDT", Time: base.Add(2 * period), Close: 102, Volume: 1200},
		// Разрыв: следующая свеча через 10 периодов, а не через один
		{Pair: "BTCUSDT", Time: base.Add(12 * period), Close: 103, Volume: 1300},
		{Pair: "BTCUSDT", Time: base.Add(13 * period), Close: 104, Volume: 1400},
	}

	// 5 свечей = 4 стыка, но один из них - разрыв => 3 выборки
	added := str.seedPairPeriod("BTCUSDT", "15m", period, candles)
	if added != 3 {
		t.Fatalf("сидирование добавило %d выборок, ожидалось 3 (разрыв должен быть пропущен)", added)
	}

	if got := str.history["BTCUSDT"]["15m"]["price"].Len(); got != 3 {
		t.Fatalf("в истории %d выборок, ожидалось 3", got)
	}
}

// setPrice подставляет паре готовое изменение цены за период, как если бы его
// посчитал минутный цикл prices.
func setPrice(t *testing.T, ap *prices.AssetsPrices, pair, period string, changePercent float64) {
	t.Helper()

	ap.ChangePricesMu.Lock()
	defer ap.ChangePricesMu.Unlock()

	ap.ChangePricesDataset[pair][period].Fill = true
	ap.ChangePrices[pair][period].ChangePercent = changePercent
}

// requiredSamples определяет, заработает ли период: пока в буфере меньше этого
// числа значений, ZScore возвращает 0, и стратегия по периоду молчит.
//
// Именно на этом мы и обожглись: 4h и 1d молчали сутками, потому что в БД не
// хватало глубины истории, а лог сидирования рапортовал "засеяно 418 пар".
func TestRequiredSamples(t *testing.T) {
	periods := map[string]time.Duration{
		"15m": 15 * time.Minute,
		"1d":  12 * time.Hour,
	}
	str := testStrategy([]string{"BTCUSDT"}, periods)
	str.Config.MinSamples = 20

	// Окно 15m больше minSamples - нужен полный minSamples
	str.Config.Periods["15m"].HistoryWindowSize = 48
	if got := str.requiredSamples("15m"); got != 20 {
		t.Errorf("для 15m нужно %d выборок, ожидалось 20", got)
	}

	// Окно 1d МЕНЬШЕ minSamples: требование клампится по окну, иначе период
	// не заработал бы никогда - в буфер просто не влезло бы 20 значений.
	str.Config.Periods["1d"].HistoryWindowSize = 14
	if got := str.requiredSamples("1d"); got != 14 {
		t.Errorf("для 1d нужно %d выборок, ожидалось 14 (клампинг по окну)", got)
	}
}

// Cooldown: пока он активен, повторное уведомление того же уровня не уходит,
// но усиление аномалии (уровень вырос) пробивает его.
//
// Главное, что проверяет тест, - что метрика, болтающаяся вокруг порога, не шлёт
// уведомление каждую минуту. Раньше lastLevel обнулялся на любой спокойной
// минуте, и на следующей правило эскалации видело 1 > 0 и пробивало ещё активный
// cooldown.
func TestCooldownSuppressesFlapping(t *testing.T) {
	const (
		pair   = "BTCUSDT"
		period = "15m"
	)
	periods := map[string]time.Duration{period: 15 * time.Minute}

	ap, err := prices.NewAssetsPrices([]string{pair}, periods, periods, stubPricesRepo{})
	if err != nil {
		t.Fatal(err)
	}

	notify := notification.NewNotificationService(true)

	str := testStrategy([]string{pair}, periods)
	str.Config.NotificationEnable = true
	str.AssetsPrices = ap
	str.Notification = notify

	// Спокойная история: изменения цены в пределах ±0.3%
	record := str.history[pair][period]["price"]
	for i := 0; i < 25; i++ {
		record.Add(0.1 * float64(i%3))
	}

	// 1. Аномалия уровня 1: +1.8% при обычных десятых долях процента
	setPrice(t, ap, pair, period, 1.8)
	str.checkAndNotify()

	if got := len(notify.Message); got != 1 {
		t.Fatalf("первая аномалия должна дать уведомление, получено %d", got)
	}
	<-notify.Message

	// 2. Спокойная минута: аномалии нет, уведомлять не о чем
	setPrice(t, ap, pair, period, 0.2)
	str.checkAndNotify()

	if got := len(notify.Message); got != 0 {
		t.Fatalf("на спокойной минуте уведомлений быть не должно, получено %d", got)
	}

	// 3. Та же аномалия того же уровня, cooldown ещё активен - молчим.
	// Ровно здесь раньше и был баг.
	setPrice(t, ap, pair, period, 1.8)
	str.checkAndNotify()

	if got := len(notify.Message); got != 0 {
		t.Fatalf("повторная аномалия того же уровня не должна пробивать cooldown, получено %d уведомлений", got)
	}

	// 4. Аномалия усилилась до уровня 3 - уведомление уходит, несмотря на cooldown
	setPrice(t, ap, pair, period, 5.0)
	str.checkAndNotify()

	if got := len(notify.Message); got != 1 {
		t.Fatalf("усиление аномалии должно пробивать cooldown, получено %d уведомлений", got)
	}
}

// Порог экономической значимости: z-score меряет статистическую неожиданность,
// а не силу события. Движение меньше minChange гасится, каким бы большим ни был z.
func TestMinChangeSuppressesTinyMoves(t *testing.T) {
	const (
		pair   = "BTCUSDT"
		period = "15m"
	)
	periods := map[string]time.Duration{period: 15 * time.Minute}

	ap, err := prices.NewAssetsPrices([]string{pair}, periods, periods, stubPricesRepo{})
	if err != nil {
		t.Fatal(err)
	}

	notify := notification.NewNotificationService(true)

	str := testStrategy([]string{pair}, periods)
	str.Config.NotificationEnable = true
	str.AssetsPrices = ap
	str.Notification = notify

	// История стейблкоина: цена стоит намертво
	record := str.history[pair][period]["price"]
	for i := 0; i < 25; i++ {
		record.Add(0.001 * float64(i%3))
	}

	// Движение 0.5% статистически огромно для такой истории, но меньше
	// minChange.price = 1.0% - значит это не событие
	setPrice(t, ap, pair, period, 0.5)
	str.checkAndNotify()

	if got := len(notify.Message); got != 0 {
		t.Fatalf("движение ниже minChange не должно давать уведомление, получено %d", got)
	}
}

func TestCooldownEqualsPeriodDuration(t *testing.T) {
	periods := map[string]time.Duration{
		"15m": 15 * time.Minute,
		"1d":  12 * time.Hour, // в конфиге приложения "1d" - это 12 часов
	}
	str := testStrategy([]string{"BTCUSDT"}, periods)

	// Cooldown берётся из реальной длительности периода, а не из таблицы имён:
	// иначе "1d" дал бы 24 часа и разошёлся с тем, что бот считает периодом.
	if got := str.cooldownForPeriod("1d"); got != 12*time.Hour {
		t.Fatalf("cooldown для 1d = %v, ожидалось 12h", got)
	}
	if got := str.cooldownForPeriod("15m"); got != 15*time.Minute {
		t.Fatalf("cooldown для 15m = %v, ожидалось 15m", got)
	}
}
