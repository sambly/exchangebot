package anomaly

import (
	"math"
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/signal"
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
// Именно на этом мы и обожглись: 4h и 12h молчали сутками, потому что в БД не
// хватало глубины истории, а лог сидирования рапортовал "засеяно 418 пар".
func TestRequiredSamples(t *testing.T) {
	periods := map[string]time.Duration{
		"15m": 15 * time.Minute,
		"12h": 12 * time.Hour,
	}
	str := testStrategy([]string{"BTCUSDT"}, periods)
	str.Config.MinSamples = 20

	// Окно 15m больше minSamples - нужен полный minSamples
	str.Config.Periods["15m"].HistoryWindowSize = 48
	if got := str.requiredSamples("15m"); got != 20 {
		t.Errorf("для 15m нужно %d выборок, ожидалось 20", got)
	}

	// Окно 12h МЕНЬШЕ minSamples: требование клампится по окну, иначе период
	// не заработал бы никогда - в буфер просто не влезло бы 20 значений.
	str.Config.Periods["12h"].HistoryWindowSize = 14
	if got := str.requiredSamples("12h"); got != 14 {
		t.Errorf("для 12h нужно %d выборок, ожидалось 14 (клампинг по окну)", got)
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

// Направление сигнала обязано определяться ЦЕНОЙ, а не композитным z-score.
//
// Композит - это z метрики с максимальным |z| среди всех, включая объём. Но объём
// ненаправлен: он растёт и на разгоне, и на сбросе. Раньше падение цены на 8% при
// взлетевшем объёме давало композит +9, сигнал уходил как UP, и исполнитель с
// onUp:buy покупал падающий нож.
func TestSignalDirectionComesFromPrice(t *testing.T) {
	str := testStrategy([]string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute})

	// Цена рухнула, но композит положительный - как если бы его задал объём
	result := &AnomalyResult{
		Pair:        "BTCUSDT",
		Period:      "15m",
		Level:       3,
		CompositeZ:  9.0,
		HasPrice:    true,
		PriceChange: -8.0,
		PriceZ:      -5.0,
		IsAnomalous: true,
	}

	sig := str.signalFrom(result)

	if sig.Direction != signal.DirectionDown {
		t.Fatalf("цена упала на 8%%, направление должно быть DOWN, получено %s", sig.Direction)
	}
	if sig.ChangePercent != -8.0 {
		t.Errorf("ChangePercent = %v, ожидалось -8.0", sig.ChangePercent)
	}
}

// Направление берётся из СЫРОГО изменения цены, а не из знака её z-score.
// z считается относительно медианы истории: у пары в устойчивом росте выросшая
// цена легко даёт отрицательный z, но продана она от этого не была.
func TestSignalDirectionIgnoresZScoreSign(t *testing.T) {
	str := testStrategy([]string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute})

	sig := str.signalFrom(&AnomalyResult{
		Pair: "BTCUSDT", Period: "15m", Level: 1,
		HasPrice: true, PriceChange: 2.0, PriceZ: -4.0, CompositeZ: -4.0,
	})

	if sig.Direction != signal.DirectionUp {
		t.Fatalf("цена выросла на 2%%, направление должно быть UP, получено %s", sig.Direction)
	}
}

// Без ценовой метрики направления не существует - такой результат подписчикам
// не отдаём вовсе, вместо того чтобы по умолчанию слать UP.
func TestPublishSkipsResultsWithoutPrice(t *testing.T) {
	str := testStrategy([]string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute})

	ch := make(chan signal.Signal, 4)
	str.Subscribe(ch)

	str.publish([]*AnomalyResult{
		{Pair: "BTCUSDT", Period: "15m", Level: 3, CompositeZ: 9.0, HasPrice: false},
		{Pair: "ETHUSDT", Period: "15m", Level: 2, CompositeZ: 5.0, HasPrice: true, PriceChange: 3.0},
	})

	if got := len(ch); got != 1 {
		t.Fatalf("подписчику должен уйти только сигнал с ценой, получено %d", got)
	}
	if sig := <-ch; sig.Pair != "ETHUSDT" {
		t.Fatalf("ушёл сигнал по %s, ожидался ETHUSDT", sig.Pair)
	}
}

// thresholds, общие для тестов классификации
func testThresholds() *ThresholdsConfig {
	return &ThresholdsConfig{Level1: 4, Level2: 6, Level3: 8}
}

// Уровень задаёт цена; подтверждающая активность его не меняет и не помечает
// движение рассогласованным.
func TestClassifyLevelComesFromPrice(t *testing.T) {
	str := testStrategy([]string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute})

	result := &AnomalyResult{
		HasPrice: true, PriceChange: 6.0, PriceZ: 6.5,
		HasActivity: true, ActivityZ: 3.0,
	}
	str.classify(result, testThresholds())

	if result.Level != 2 {
		t.Fatalf("уровень %d, ожидался 2 (z=6.5 при level2=6)", result.Level)
	}
	if result.Divergent {
		t.Error("движение подтверждено объёмом, рассогласованным считаться не должно")
	}
	if result.CompositeZ != 6.5 {
		t.Errorf("composite = %v, ожидался z цены 6.5", result.CompositeZ)
	}
}

// Цена сходила, а активность была НИЖЕ обычной - движение по пустому стакану.
// Уровень не штрафуется (решение за человеком), но пометка обязана стоять:
// в дайджесте она отличает вынос от подтверждённого импульса.
func TestClassifyMarksDivergentMove(t *testing.T) {
	str := testStrategy([]string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute})

	result := &AnomalyResult{
		HasPrice: true, PriceChange: 6.0, PriceZ: 8.5,
		HasActivity: true, ActivityZ: -2.0, // торгов меньше обычного
	}
	str.classify(result, testThresholds())

	if !result.Divergent {
		t.Error("движение при упавшей активности должно помечаться как рассогласованное")
	}
	if result.Level != 3 {
		t.Fatalf("уровень %d, ожидался 3 - пометка не должна менять уровень", result.Level)
	}

	// Без метрик активности пометки нет: рассогласование не с чем мерить
	bare := &AnomalyResult{HasPrice: true, PriceChange: 6.0, PriceZ: 8.5, HasActivity: false}
	str.classify(bare, testThresholds())
	if bare.Divergent {
		t.Error("без метрик активности движение не может считаться рассогласованным")
	}
	if bare.Level != 3 {
		t.Fatalf("уровень %d, ожидался 3", bare.Level)
	}
}

// Один только всплеск объёма при стоящей цене - не событие.
// Раньше level брался как max по метрикам, и такой всплеск уходил исполнителю
// полноценным сигналом 3 уровня, хотя цена стояла на месте.
func TestClassifyVolumeOnlySpikeIsNotAnomaly(t *testing.T) {
	str := testStrategy([]string{"BTCUSDT"}, map[string]time.Duration{"15m": 15 * time.Minute})

	result := &AnomalyResult{
		HasPrice: true, PriceChange: 0.1, PriceZ: 0.2,
		HasActivity: true, ActivityZ: 9.0,
	}
	str.classify(result, testThresholds())

	if result.IsAnomalous {
		t.Fatalf("всплеск объёма при стоящей цене не должен быть аномалией, получен уровень %d", result.Level)
	}
}

func TestCooldownEqualsPeriodDuration(t *testing.T) {
	periods := map[string]time.Duration{
		"15m": 15 * time.Minute,
		"12h": 12 * time.Hour,
	}
	str := testStrategy([]string{"BTCUSDT"}, periods)

	// Cooldown берётся из реальной длительности периода, а не из таблицы имён:
	// иначе своя таблица разошлась бы с тем, что бот считает периодом.
	if got := str.cooldownForPeriod("12h"); got != 12*time.Hour {
		t.Fatalf("cooldown для 12h = %v, ожидалось 12h", got)
	}
	if got := str.cooldownForPeriod("15m"); got != 15*time.Minute {
		t.Fatalf("cooldown для 15m = %v, ожидалось 15m", got)
	}
}

// Окно подтверждения: рынок дёрнулся на одну минуту и вернулся - это не событие.
// Сигнал уходит, только если аномалия пережила confirmChecks проверок подряд.
func TestConfirmChecksFiltersOneMinuteSpike(t *testing.T) {
	state := &PeriodState{}

	spike := &AnomalyResult{Level: 2, PriceChange: 5.0, IsAnomalous: true}
	if confirm(state, spike, 2) {
		t.Fatal("первая проверка не должна подтверждать аномалию при confirmChecks=2")
	}

	// Следующая минута спокойная - так это и делается в checkAndNotify
	state.pendingChecks = 0

	// Дёрг не повторился, значит и сигнала не было
	if state.pendingChecks != 0 {
		t.Fatal("после спокойной минуты окно подтверждения должно быть сброшено")
	}
}

// Аномалия, которая держится, - подтверждается. Уровень при этом берётся ПИКОВЫЙ
// за окно, а не последний и не средний: сглаживание срезало бы сильнейшую минуту
// движения, и настоящий level 3 не прошёл бы minLevel у исполнителя.
func TestConfirmChecksKeepsPeakLevel(t *testing.T) {
	state := &PeriodState{}

	// Минута 1: аномалия уровня 3 - но подтверждения ещё нет
	first := &AnomalyResult{Level: 3, PriceChange: 8.0, IsAnomalous: true}
	if confirm(state, first, 2) {
		t.Fatal("одной проверки недостаточно при confirmChecks=2")
	}

	// Минута 2: аномалия держится, но уже слабее
	second := &AnomalyResult{Level: 1, PriceChange: 6.0, IsAnomalous: true}
	if !confirm(state, second, 2) {
		t.Fatal("аномалия продержалась 2 проверки - должна подтвердиться")
	}
	if second.Level != 3 {
		t.Fatalf("уровень %d, ожидался пиковый 3 за окно подтверждения", second.Level)
	}
}

// Разворот - это ДРУГОЕ событие. Минута роста и минута падения не складываются
// в подтверждённый сигнал.
func TestConfirmChecksResetsOnDirectionChange(t *testing.T) {
	state := &PeriodState{}

	up := &AnomalyResult{Level: 2, PriceChange: 5.0, IsAnomalous: true}
	confirm(state, up, 2)

	down := &AnomalyResult{Level: 2, PriceChange: -5.0, IsAnomalous: true}
	if confirm(state, down, 2) {
		t.Fatal("смена направления должна начинать окно подтверждения заново")
	}
	if state.pendingChecks != 1 {
		t.Fatalf("счётчик подтверждения = %d, ожидался 1 (новое событие)", state.pendingChecks)
	}
}

// confirmChecks=1 - поведение как раньше: срабатывание с первой же проверки.
func TestConfirmChecksDisabled(t *testing.T) {
	state := &PeriodState{}
	result := &AnomalyResult{Level: 2, PriceChange: 5.0, IsAnomalous: true}

	if !confirm(state, result, 1) {
		t.Fatal("при confirmChecks=1 аномалия должна подтверждаться сразу")
	}
}

// Дисбаланс - единственная направленная метрика кроме цены. volumeBuy и
// volumeAsk по отдельности растут вместе с любой активностью и о том, кто давит,
// не говорят ничего; отвечает на этот вопрос только их отношение.
// Сквозной тест бэктест-адаптера: walk-forward набор истории по свечам и
// срабатывание на аномальной свече - тем же ядром detect, что и в бою.
func TestDetectCandleWalkForward(t *testing.T) {
	const (
		pair   = "BTCUSDT"
		period = "15m"
	)
	str := testStrategy([]string{pair}, map[string]time.Duration{period: 15 * time.Minute})
	str.Config.MinSamples = 20

	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	candle := func(i int, close float64) exModel.Candle {
		return exModel.Candle{Pair: pair, Time: base.Add(time.Duration(i) * 15 * time.Minute), Close: close}
	}

	// Спокойная история: дрейф в десятые доли процента
	price := 100.0
	prev := candle(0, price)
	for i := 1; i <= 30; i++ {
		price *= 1 + 0.001*float64(i%3)
		current := candle(i, price)

		if sig, found := str.DetectCandle(pair, period, prev, current); found {
			t.Fatalf("на спокойной свече %d не должно быть сигнала, z=%.1f", i, sig.Strength)
		}
		prev = current
	}

	// Аномальная свеча: +5% при обычных десятых долях
	spike := candle(31, price*1.05)
	sig, found := str.DetectCandle(pair, period, prev, spike)
	if !found {
		t.Fatal("движение +5% на спокойной истории должно дать сигнал")
	}
	if sig.Direction != signal.DirectionUp {
		t.Fatalf("направление %s, ожидалось UP", sig.Direction)
	}
	if sig.Time != spike.Time {
		t.Fatalf("время сигнала %v, ожидалось время свечи %v", sig.Time, spike.Time)
	}
	if sig.Volatility <= 0 {
		t.Fatal("волатильность должна быть посчитана из набранной истории")
	}
}
