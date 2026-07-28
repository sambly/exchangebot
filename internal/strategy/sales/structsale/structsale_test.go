package structsale

import (
	"math"
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/depth"
	"github.com/sambly/exchangebot/internal/entrysetup"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/strategy/sales"
	"github.com/sambly/exchangebot/internal/strategy/signal"
)

func testSale() *StrategyStructSale {
	return &StrategyStructSale{
		Config: &Config{
			IDName:                    "salestruct",
			TakeProfitVol:             2.0,
			StopLossVol:               1.5,
			TakeProfitPercent:         2.0,
			StopLossPercent:           1.5,
			MinTakeProfitPercent:      0.5,
			MaxTakeProfitPercent:      20.0,
			MinStopLossPercent:        0.5,
			MaxStopLossPercent:        10.0,
			MaxHoldPeriods:            4,
			StrengthScalePerLevel:     0.15,
			TrailingActivationPercent: 1.0,
			TrailingCallbackPercent:   0.6,
		},
	}
}

// Базовый план (без масштабирования по силе, level<=1) должен вести себя
// как у simplesale: тейк/стоп от волатильности пары.
func TestPlanScalesWithVolatility(t *testing.T) {
	str := testSale()

	calm := signal.Signal{Period: "15m", Volatility: 0.4, Level: 1}
	wild := signal.Signal{Period: "15m", Volatility: 3.0, Level: 1}

	calmTP, calmSL, hold := str.Plan(calm)
	wildTP, wildSL, _ := str.Plan(wild)

	if calmTP >= wildTP || calmSL >= wildSL {
		t.Fatalf("у волатильной пары пороги должны быть шире: спокойная tp=%.2f sl=%.2f, волатильная tp=%.2f sl=%.2f",
			calmTP, calmSL, wildTP, wildSL)
	}
	if wildTP < 5.9 || wildTP > 6.1 {
		t.Errorf("тейк волатильной пары (level=1) = %.2f%%, ожидалось ~6%% (3%% x 2.0)", wildTP)
	}
	if hold != time.Hour {
		t.Errorf("время удержания = %v, ожидался час (15m x 4)", hold)
	}
}

// Более сильный сигнал должен получать более широкий тейк, но НЕ более
// широкий стоп - масштабируется только цель, не риск на входе.
func TestPlanScalesWithSignalStrength(t *testing.T) {
	str := testSale()

	level1TP, level1SL, _ := str.Plan(signal.Signal{Period: "15m", Volatility: 1.0, Level: 1})
	level3TP, level3SL, _ := str.Plan(signal.Signal{Period: "15m", Volatility: 1.0, Level: 3})

	if level3TP <= level1TP {
		t.Fatalf("уровень 3 должен давать более широкий тейк, чем уровень 1: level1=%.2f level3=%.2f", level1TP, level3TP)
	}
	if level1SL != level3SL {
		t.Fatalf("стоп не должен зависеть от уровня силы сигнала: level1=%.2f level3=%.2f", level1SL, level3SL)
	}

	// volatility=1.0, TakeProfitVol=2.0 -> базовый тейк 2.0%; level=3 ->
	// x(1+2*0.15)=x1.3 -> 2.6%.
	want := 2.0 * 1.3
	if level3TP < want-0.01 || level3TP > want+0.01 {
		t.Errorf("тейк уровня 3 = %.3f%%, ожидалось %.3f%%", level3TP, want)
	}
}

// StrengthScalePerLevel=0 должен полностью выключать масштабирование.
func TestPlanStrengthScaleDisabled(t *testing.T) {
	str := testSale()
	str.Config.StrengthScalePerLevel = 0

	level1TP, _, _ := str.Plan(signal.Signal{Period: "15m", Volatility: 1.0, Level: 1})
	level3TP, _, _ := str.Plan(signal.Signal{Period: "15m", Volatility: 1.0, Level: 3})

	if level1TP != level3TP {
		t.Fatalf("при StrengthScalePerLevel=0 тейк не должен зависеть от уровня: level1=%.2f level3=%.2f", level1TP, level3TP)
	}
}

// Если детектор не знает волатильности - откат на проценты из конфига.
func TestPlanFallsBackToPercents(t *testing.T) {
	str := testSale()

	takeProfit, stopLoss, _ := str.Plan(signal.Signal{Period: "1h", Volatility: 0, Level: 1})

	if takeProfit != 2.0 || stopLoss != 1.5 {
		t.Fatalf("без волатильности ожидались проценты из конфига (2.0/1.5), получено %.2f/%.2f", takeProfit, stopLoss)
	}
}

func position(id int64, entry, takeProfit, stopLoss float64, deadline time.Time) sales.Position {
	return sales.Position{
		Order: order.Order{
			ID:           id,
			Pair:         "BTCUSDT",
			Side:         order.SideTypeBuy,
			PriceCreated: entry,
		},
		TakeProfitPercent: takeProfit,
		StopLossPercent:   stopLoss,
		Deadline:          deadline,
	}
}

// До активации трейлинга (мало прибыли) поведение должно быть идентично
// simplesale: фиксированный тейк/стоп/таймаут.
func TestShouldExitBeforeTrailingActivates(t *testing.T) {
	str := testSale()
	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	cases := []struct {
		name  string
		price float64
		pos   sales.Position
		want  sales.ExitReason
		exit  bool
	}{
		{"стоп сработал", 98.2, position(1, 100, 2.0, 1.5, future), sales.ExitStopLoss, true},
		{"внутри коридора - держим", 100.3, position(2, 100, 2.0, 1.5, future), "", false},
		{"сигнал протух", 100.1, position(3, 100, 2.0, 1.5, past), sales.ExitTimeout, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason, exit := str.ShouldExit(c.price, now, c.pos)
			if exit != c.exit || reason != c.want {
				t.Fatalf("price=%.2f -> (%q, %v), ожидалось (%q, %v)", c.price, reason, exit, c.want, c.exit)
			}
		})
	}
}

// Прибыль выше TrailingActivationPercent(1.0), но фиксированный тейк(2.0) ещё
// не достигнут - трейлинг уже активен и НЕ должен закрывать по фиксированному
// тейку (даём расти дальше), только по откату от максимума.
func TestTrailingSuppressesFixedTakeProfitOnceActive(t *testing.T) {
	str := testSale()
	pos := position(10, 100, 2.0, 1.5, time.Time{})

	// Прибыль 1.2% - выше активации (1.0), ниже фиксированного тейка (2.0)
	reason, exit := str.ShouldExit(101.2, time.Now(), pos)
	if exit {
		t.Fatalf("трейлинг активен, отката ещё не было - позиция не должна закрываться, получено (%q, %v)", reason, exit)
	}
}

// Разворот от максимума больше TrailingCallbackPercent должен закрывать
// позицию по трейлингу, даже если цена всё ещё намного выше входа (то есть
// НЕ является обычным стоп-лоссом от входа).
func TestTrailingStopTriggersOnCallback(t *testing.T) {
	str := testSale()
	pos := position(11, 100, 2.0, 1.5, time.Time{})

	// Разгон до 103 (профит 3%, трейлинг активен, максимум запомнен).
	if reason, exit := str.ShouldExit(103.0, time.Now(), pos); exit {
		t.Fatalf("на новом максимуме закрытия быть не должно, получено (%q, %v)", reason, exit)
	}

	// Откат до 102.3: профит 2.3%, отдали 0.7 п.п. от максимума (3%) -
	// больше callback(0.6) -> трейлинг должен сработать. Цена (102.3) всё ещё
	// далеко от исходного стопа (98.5) и даже выше фиксированного тейка (102) -
	// обычный тейк/стоп такого закрытия бы не объяснили.
	reason, exit := str.ShouldExit(102.3, time.Now(), pos)
	if !exit || reason != sales.ExitTrailingStop {
		t.Fatalf("ожидался ExitTrailingStop после отката 0.7 п.п. (callback=0.6), получено (%q, %v)", reason, exit)
	}
}

// Экстремум должен только расширяться (никогда не сужаться при промежуточном
// менее выгодном сэмпле) - важно для бэктеста, где один бар проверяется по
// худшей/лучшей/цене закрытия в этом порядке, а не в хронологическом.
func TestExtremeIsMonotonic(t *testing.T) {
	str := testSale()
	pos := position(12, 100, 2.0, 1.5, time.Time{})

	str.ShouldExit(101.0, time.Now(), pos) // профит 1% - активирует трейлинг
	str.ShouldExit(104.0, time.Now(), pos) // новый максимум
	// Хуже максимума, но откат (0.5 п.п.) ещё меньше callback(0.6) - не
	// закрывает позицию и не должен портить уже запомненный максимум.
	str.ShouldExit(103.5, time.Now(), pos)

	extreme := str.extreme[pos.Order.ID]
	if extreme != 104.0 {
		t.Fatalf("экстремум должен остаться 104.0 после менее выгодного сэмпла, получено %.2f", extreme)
	}
}

// Жёсткий стоп от входа должен продолжать действовать даже после активации
// трейлинга - как подстраховка на случай большого разрыва цены.
func TestHardStopStillAppliesAfterTrailingActivates(t *testing.T) {
	str := testSale()
	pos := position(13, 100, 2.0, 1.5, time.Time{})

	str.ShouldExit(101.5, time.Now(), pos) // активирует трейлинг

	// Обвал сразу ниже жёсткого стопа (98.5), минуя трейлинг-порог.
	reason, exit := str.ShouldExit(98.0, time.Now(), pos)
	if !exit || reason != sales.ExitStopLoss {
		t.Fatalf("резкий обвал ниже жёсткого стопа должен закрыть позицию как ExitStopLoss, получено (%q, %v)", reason, exit)
	}
}

// У шорта прибыль перевёрнута - трейлинг должен запоминать МИНИМУМ цены, а не
// максимум, и триггериться на откате вверх.
func TestTrailingStopShortSide(t *testing.T) {
	str := testSale()
	pos := position(14, 100, 2.0, 1.5, time.Time{})
	pos.Order.Side = order.SideTypeSell

	// Падение до 97 (профит 3% для шорта) - активирует трейлинг.
	if reason, exit := str.ShouldExit(97.0, time.Now(), pos); exit {
		t.Fatalf("на новом минимуме закрытия быть не должно, получено (%q, %v)", reason, exit)
	}

	// Откат вверх до 97.7: профит 2.3%, отдали 0.7 п.п. от максимума (3%).
	reason, exit := str.ShouldExit(97.7, time.Now(), pos)
	if !exit || reason != sales.ExitTrailingStop {
		t.Fatalf("ожидался ExitTrailingStop для шорта после отката 0.7 п.п., получено (%q, %v)", reason, exit)
	}
}

// Закрытая через ShouldExit позиция должна освобождать своё состояние
// экстремума - иначе оно бы копилось вечно.
func TestClearExtremeAfterExit(t *testing.T) {
	str := testSale()
	pos := position(15, 100, 2.0, 1.5, time.Time{})

	str.ShouldExit(98.2, time.Now(), pos) // стоп-лосс, закрытие

	if _, ok := str.extreme[pos.Order.ID]; ok {
		t.Fatal("после закрытия позиции запись экстремума должна быть удалена")
	}
}

// --- структурный стоп/тейк от стен стакана ---

func wallLevel(price, qty float64) exModel.DepthLevel {
	return exModel.DepthLevel{Price: price, Quantity: qty}
}

// wallLevels - n уровней с шагом step от price0, все с одинаковым объёмом
// baseQty, кроме index wallIdx - там объём wallQty (стена). Тот же приём, что
// в entrysetup/score_test.go, продублирован здесь локально - тестовые
// хелперы одного пакета не видны из другого.
func wallLevels(price0, step float64, n int, baseQty float64, wallIdx int, wallQty float64) []exModel.DepthLevel {
	out := make([]exModel.DepthLevel, 0, n)
	for i := 0; i < n; i++ {
		qty := baseQty
		if i == wallIdx {
			qty = wallQty
		}
		out = append(out, wallLevel(price0+step*float64(i), qty))
	}
	return out
}

// setupWithWalls строит AssetsSetup с одним стаканом, где support заметно
// ближе к цене, чем resistance (Score значительно больше 1).
func setupWithWalls(t *testing.T, pair string) *entrysetup.AssetsSetup {
	t.Helper()

	bids := wallLevels(100.0, -1.0, 20, 1.0, 1, 50.0)  // ближняя стена ~1% вниз
	asks := wallLevels(100.1, 1.0, 20, 1.0, 10, 50.0) // дальняя стена ~10% вверх

	ad := depth.NewAssetsDepth([]string{pair})
	ad.OnDepth(exModel.DepthUpdate{Pair: pair, Bids: bids, Asks: asks})

	return entrysetup.NewAssetsSetup(nil, ad)
}

// Когда структурные данные доступны и достаточно привлекательны,
// structuralPlan должен отдавать РОВНО дистанции Quality, а не что-то от
// волатильности.
func TestStructuralPlanUsesQualityDistances(t *testing.T) {
	pair := "BTCUSDT"
	es := setupWithWalls(t, pair)

	quality, ok := es.GetQuality(pair, "15m")
	if !ok {
		t.Fatal("ожидалась найденная Quality для теста")
	}

	str := testSale()
	str.entrySetup = es
	str.Config.UseStructuralLevels = true
	str.Config.MinStructuralScore = 1.0

	takeProfit, stopLoss, ok := str.structuralPlan(signal.Signal{Pair: pair, Period: "15m"})
	if !ok {
		t.Fatal("structuralPlan должен вернуть ok=true")
	}
	if math.Abs(takeProfit-quality.TakeDistancePercent) > 1e-9 || math.Abs(stopLoss-quality.StopDistancePercent) > 1e-9 {
		t.Fatalf("ожидались дистанции Quality (tp=%.4f sl=%.4f), получено tp=%.4f sl=%.4f",
			quality.TakeDistancePercent, quality.StopDistancePercent, takeProfit, stopLoss)
	}
}

// Plan() должен полностью заменить волатильностный план структурным, когда
// тот доступен - даже при совсем другой волатильности сигнала.
func TestPlanPrefersStructuralOverVolatility(t *testing.T) {
	pair := "BTCUSDT"
	es := setupWithWalls(t, pair)
	quality, _ := es.GetQuality(pair, "15m")

	str := testSale()
	str.entrySetup = es
	str.Config.UseStructuralLevels = true
	str.Config.MinStructuralScore = 1.0

	// Волатильность намеренно сильно другая, чтобы волатильностный план дал
	// заметно иные числа, если бы структурный не применился.
	takeProfit, stopLoss, _ := str.Plan(signal.Signal{Pair: pair, Period: "15m", Volatility: 0.1, Level: 1})

	if math.Abs(takeProfit-quality.TakeDistancePercent) > 1e-9 || math.Abs(stopLoss-quality.StopDistancePercent) > 1e-9 {
		t.Fatalf("Plan должен вернуть структурные дистанции (tp=%.4f sl=%.4f), получено tp=%.4f sl=%.4f",
			quality.TakeDistancePercent, quality.StopDistancePercent, takeProfit, stopLoss)
	}
}

// Без entrySetup структурный план должен быть недоступен - Plan откатывается
// на волатильность, как simplesale.
func TestStructuralPlanNilEntrySetup(t *testing.T) {
	str := testSale()
	str.entrySetup = nil
	str.Config.UseStructuralLevels = true

	if _, _, ok := str.structuralPlan(signal.Signal{Pair: "BTCUSDT", Period: "15m"}); ok {
		t.Fatal("без entrySetup structuralPlan должен вернуть ok=false")
	}
}

// UseStructuralLevels=false должен отключать фичу, даже если entrySetup есть
// и стены найдены.
func TestStructuralPlanDisabledByConfig(t *testing.T) {
	pair := "BTCUSDT"
	es := setupWithWalls(t, pair)

	str := testSale()
	str.entrySetup = es
	str.Config.UseStructuralLevels = false

	if _, _, ok := str.structuralPlan(signal.Signal{Pair: pair, Period: "15m"}); ok {
		t.Fatal("при UseStructuralLevels=false structuralPlan должен вернуть ok=false")
	}
}

// --- адаптивный таймаут ---

// Прибыль на дедлайне ниже порога - закрываем по таймауту как обычно,
// адаптивность здесь ничего не меняет.
func TestAdaptiveTimeoutClosesWhenBelowThreshold(t *testing.T) {
	str := testSale()
	str.Config.AdaptiveTimeoutMinProfitPercent = 0.3
	str.Config.AdaptiveTimeoutExtensionPeriods = 4

	past := time.Now().Add(-time.Minute)
	pos := position(30, 100, 10.0, 8.0, past)

	// Профит 0.1% - ниже порога 0.3%.
	reason, exit := str.ShouldExit(100.1, time.Now(), pos)
	if !exit || reason != sales.ExitTimeout {
		t.Fatalf("ожидался ExitTimeout (профит ниже порога), получено (%q, %v)", reason, exit)
	}
}

// Прибыль на дедлайне выше порога - дедлайн продлевается, позиция НЕ
// закрывается по таймауту.
func TestAdaptiveTimeoutExtendsWhenProfitable(t *testing.T) {
	str := testSale()
	str.Config.AdaptiveTimeoutMinProfitPercent = 0.3
	str.Config.AdaptiveTimeoutExtensionPeriods = 4

	past := time.Now().Add(-time.Minute)
	pos := position(31, 100, 10.0, 8.0, past)

	// Профит 3% - выше порога 0.3%, дедлайн уже прошёл.
	reason, exit := str.ShouldExit(103.0, time.Now(), pos)
	if exit {
		t.Fatalf("ожидалось продление дедлайна, а не закрытие, получено (%q, %v)", reason, exit)
	}

	if extended := str.effectiveDeadline(pos); !extended.After(time.Now()) {
		t.Fatalf("эффективный дедлайн должен сдвинуться в будущее, получено %v", extended)
	}
}

// Продление одноразовое: после того как эффективный дедлайн тоже истёк,
// позиция закрывается по таймауту, даже если всё ещё в достаточном плюсе -
// иначе позиция в перманентном плюсе никогда не закрылась бы по времени.
func TestAdaptiveTimeoutExtendsOnlyOnce(t *testing.T) {
	str := testSale()
	str.Config.AdaptiveTimeoutMinProfitPercent = 0.3
	str.Config.AdaptiveTimeoutExtensionPeriods = 4

	past := time.Now().Add(-time.Minute)
	pos := position(32, 100, 10.0, 8.0, past)

	if _, exit := str.ShouldExit(103.0, time.Now(), pos); exit {
		t.Fatal("первое продление должно пройти без закрытия")
	}

	extended := str.effectiveDeadline(pos)
	afterExtension := extended.Add(time.Minute)

	reason, exit := str.ShouldExit(103.0, afterExtension, pos)
	if !exit || reason != sales.ExitTimeout {
		t.Fatalf("после второго истечения (продление уже использовано) ожидался ExitTimeout, получено (%q, %v)", reason, exit)
	}
}

// AdaptiveTimeoutMinProfitPercent=0 (значение по умолчанию в тестовом
// конфиге) должен полностью отключать адаптивность - поведение как у
// simplesale, даже если ExtensionPeriods почему-то задан.
func TestAdaptiveTimeoutDisabledByDefault(t *testing.T) {
	str := testSale()
	str.Config.AdaptiveTimeoutMinProfitPercent = 0 // явно выключено
	str.Config.AdaptiveTimeoutExtensionPeriods = 4

	past := time.Now().Add(-time.Minute)
	pos := position(33, 100, 10.0, 8.0, past)

	// Даже с большим профитом - без MinProfitPercent продления не будет.
	reason, exit := str.ShouldExit(105.0, time.Now(), pos)
	if !exit || reason != sales.ExitTimeout {
		t.Fatalf("при выключенной адаптивности ожидался обычный ExitTimeout, получено (%q, %v)", reason, exit)
	}
}

// --- частичный тейк-профит ---

// Дистанция частичного тейка должна считаться от TakeProfitPercent КОНКРЕТНОЙ
// позиции (уже посчитанного плана), а не заново от волатильности.
func TestPartialTakeProfitComputesDistance(t *testing.T) {
	str := testSale()
	str.Config.PartialTakeProfitPercent = 0.5
	str.Config.PartialTakeProfitFraction = 0.4

	pos := position(20, 100, 4.0, 1.5, time.Time{})

	distance, fraction, ok := str.PartialTakeProfit(pos)
	if !ok {
		t.Fatal("ожидался ok=true")
	}
	if math.Abs(distance-2.0) > 1e-9 { // 4.0 x 0.5
		t.Errorf("дистанция = %.4f, ожидалось 2.0 (TakeProfitPercent x 0.5)", distance)
	}
	if fraction != 0.4 {
		t.Errorf("доля = %.4f, ожидалось 0.4", fraction)
	}
}

// PartialTakeProfitPercent=0 или PartialTakeProfitFraction=0 должны
// полностью отключать фичу.
func TestPartialTakeProfitDisabledByZero(t *testing.T) {
	pos := position(21, 100, 4.0, 1.5, time.Time{})

	str := testSale()
	str.Config.PartialTakeProfitPercent = 0
	str.Config.PartialTakeProfitFraction = 0.5
	if _, _, ok := str.PartialTakeProfit(pos); ok {
		t.Fatal("при PartialTakeProfitPercent=0 ожидался ok=false")
	}

	str2 := testSale()
	str2.Config.PartialTakeProfitPercent = 0.5
	str2.Config.PartialTakeProfitFraction = 0
	if _, _, ok := str2.PartialTakeProfit(pos); ok {
		t.Fatal("при PartialTakeProfitFraction=0 ожидался ok=false")
	}
}

// Доля >= 1 - это уже не частичный тейк, а полный: фича должна отключаться,
// а не закрывать 100%+ через "частичный" механизм.
func TestPartialTakeProfitRejectsFullFraction(t *testing.T) {
	str := testSale()
	str.Config.PartialTakeProfitPercent = 0.5
	str.Config.PartialTakeProfitFraction = 1.0

	pos := position(22, 100, 4.0, 1.5, time.Time{})
	if _, _, ok := str.PartialTakeProfit(pos); ok {
		t.Fatal("при PartialTakeProfitFraction=1.0 ожидался ok=false")
	}
}

// У simplesale частичного тейка нет вообще - см. отдельный тест в
// simplesale_test.go.

// Слишком слабое соотношение дальней стены к ближней (ниже MinStructuralScore)
// не должно приниматься - волатильностная оценка в этом случае честнее.
func TestStructuralPlanRejectsWeakScore(t *testing.T) {
	pair := "BTCUSDT"
	es := setupWithWalls(t, pair)

	str := testSale()
	str.entrySetup = es
	str.Config.UseStructuralLevels = true
	str.Config.MinStructuralScore = 1000 // заведомо недостижимо

	if _, _, ok := str.structuralPlan(signal.Signal{Pair: pair, Period: "15m"}); ok {
		t.Fatal("при недостаточном Score structuralPlan должен вернуть ok=false")
	}
}
