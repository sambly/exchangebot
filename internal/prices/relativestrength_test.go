package prices

import (
	"math"
	"testing"
	"time"
)

// Без накопленной истории GetRelativeStrengthZScore должен честно отдавать
// ready=false, а не 0 как будто посчитано.
func TestRelativeStrengthNotReadyWithoutHistory(t *testing.T) {
	pairs := []string{"AAAUSDT", "BBBUSDT"}
	periods := map[string]time.Duration{"1m": time.Minute}
	ap := newTestPrices(t, pairs, periods, &stubRepo{})

	if _, _, ready := ap.GetRelativeStrengthZScore("AAAUSDT", "1m"); ready {
		t.Fatal("без единого тика GetRelativeStrengthZScore должен вернуть ready=false")
	}
}

// После первого же тика с известным рынком (2+ пары) GetRelativeStrengthZScore
// НЕ должен становиться ready=true - state.history ещё не набрал
// relativeStrengthMinSamples значений, и z-score в этот момент - фиктивный 0
// (см. stat.MetricRecord.ZScore). Именно этот сценарий поймал реальный баг:
// первая версия помечала состояние "готовым" при первом же вызове
// recordRelativeStrength, независимо от размера истории - тот же класс
// ошибки, что уже правился у VPIN (см. vpinState.committed).
func TestRelativeStrengthNotReadyBeforeMinSamples(t *testing.T) {
	pairs := []string{"AAAUSDT", "BBBUSDT"}
	periods := map[string]time.Duration{"1m": time.Minute}
	ap := newTestPrices(t, pairs, periods, &stubRepo{})

	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	tick := func(priceA, priceB float64, at time.Time) {
		ap.MarketsStat["AAAUSDT"].Price, ap.MarketsStat["AAAUSDT"].Time = priceA, at
		ap.MarketsStat["BBBUSDT"].Price, ap.MarketsStat["BBBUSDT"].Time = priceB, at
		ap.updateChangePrices()
	}

	// Меньше relativeStrengthMinSamples тиков с известным рынком.
	for i := 0; i < relativeStrengthMinSamples-1; i++ {
		tick(100+float64(i), 100, base.Add(time.Duration(i)*time.Minute))
	}

	if _, z, ready := ap.GetRelativeStrengthZScore("AAAUSDT", "1m"); ready {
		t.Fatalf("до набора %d сэмплов ready должен быть false, получено ready=true z=%v", relativeStrengthMinSamples, z)
	}
}

// С одной-единственной отслеживаемой парой "рынок" не определить (медиана по
// одному значению - это само это значение, любая пара оказалась бы "точно
// как рынок") - recordRelativeStrength должен не набирать историю вообще.
func TestRelativeStrengthRequiresAtLeastTwoPairs(t *testing.T) {
	pairs := []string{"AAAUSDT"}
	periods := map[string]time.Duration{"1m": time.Minute}
	ap := newTestPrices(t, pairs, periods, &stubRepo{})

	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		ap.MarketsStat["AAAUSDT"].Price = 100 + float64(i)
		ap.MarketsStat["AAAUSDT"].Time = base.Add(time.Duration(i) * time.Minute)
		ap.updateChangePrices()
	}

	if _, _, ready := ap.GetRelativeStrengthZScore("AAAUSDT", "1m"); ready {
		t.Fatal("с одной парой относительную силу к рынку считать не из чего, ready должен остаться false")
	}
}

// Пара, устойчиво обгоняющая рынок (два других тикера стоят на месте), а
// затем внезапно вырывающаяся намного сильнее обычного, должна получить
// положительный z-score и в итоге подтверждённую сторону BUY. "Рынок"
// (сами BBB/CCC) при этом не должен считаться обгоняющим самого себя.
func TestRelativeStrengthConfirmsWhenPairOutpacesMarket(t *testing.T) {
	pairs := []string{"AAAUSDT", "BBBUSDT", "CCCUSDT"}
	periods := map[string]time.Duration{"1m": time.Minute}
	ap := newTestPrices(t, pairs, periods, &stubRepo{})

	priceA, priceB, priceC := 100.0, 100.0, 100.0
	minute := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	tick := func() {
		ap.MarketsStat["AAAUSDT"].Price, ap.MarketsStat["AAAUSDT"].Time = priceA, minute
		ap.MarketsStat["BBBUSDT"].Price, ap.MarketsStat["BBBUSDT"].Time = priceB, minute
		ap.MarketsStat["CCCUSDT"].Price, ap.MarketsStat["CCCUSDT"].Time = priceC, minute
		ap.updateChangePrices()
		minute = minute.Add(time.Minute)
	}

	// Первый тик - только заполняет окно (period=1m), ChangePercent ещё не
	// считается ни у кого.
	tick()

	// История: AAA стабильно растёт на 2% за тик, BBB/CCC (рынок) стоят на
	// месте - относительная сила AAA одна и та же (~+2 п.п.) каждый тик.
	for i := 0; i < relativeStrengthMinSamples+2; i++ {
		priceA *= 1.02
		tick()
	}

	if _, _, ready := ap.GetRelativeStrengthZScore("AAAUSDT", "1m"); !ready {
		t.Fatal("после достаточной истории GetRelativeStrengthZScore должен быть ready=true")
	}

	// Резкий выброс: AAA внезапно растёт намного сильнее обычного, рынок
	// (BBB/CCC) по-прежнему стоит на месте.
	for i := 0; i < relativeStrengthConfirmMinStreak; i++ {
		priceA *= 1.20 // +20% за тик - намного больше типичных +2%
		tick()
	}

	value, zScore, ready := ap.GetRelativeStrengthZScore("AAAUSDT", "1m")
	if !ready {
		t.Fatal("ожидался ready=true")
	}
	if value <= 0 {
		t.Fatalf("AAA обгоняет рынок - ожидалось положительное значение, получено %v", value)
	}
	if zScore < relativeStrengthConfirmZThreshold {
		t.Fatalf("ожидался z-score не ниже порога подтверждения (%v), получено %v", relativeStrengthConfirmZThreshold, zScore)
	}

	side, confirmed := ap.GetRelativeStrengthConfirmedSide("AAAUSDT", "1m")
	if !confirmed || side != "BUY" {
		t.Fatalf("ожидался Confirmed=BUY после %d выбросов подряд, получено side=%q confirmed=%v",
			relativeStrengthConfirmMinStreak, side, confirmed)
	}

	// "Рынок" сам по себе не должен считаться обгоняющим себя же.
	bValue, _, bReady := ap.GetRelativeStrengthZScore("BBBUSDT", "1m")
	if !bReady {
		t.Fatal("BBB тоже должен иметь готовую историю")
	}
	if math.Abs(bValue) > 0.01 {
		t.Fatalf("BBB двигался ровно как рынок - относительная сила должна быть ~0, получено %v", bValue)
	}
}

// Резкое отставание от рынка (не только опережение) должно давать
// отрицательный z-score и подтверждённую сторону SELL.
func TestRelativeStrengthConfirmsSellWhenPairLagsMarket(t *testing.T) {
	pairs := []string{"AAAUSDT", "BBBUSDT", "CCCUSDT"}
	periods := map[string]time.Duration{"1m": time.Minute}
	ap := newTestPrices(t, pairs, periods, &stubRepo{})

	priceA, priceB, priceC := 100.0, 100.0, 100.0
	minute := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	tick := func() {
		ap.MarketsStat["AAAUSDT"].Price, ap.MarketsStat["AAAUSDT"].Time = priceA, minute
		ap.MarketsStat["BBBUSDT"].Price, ap.MarketsStat["BBBUSDT"].Time = priceB, minute
		ap.MarketsStat["CCCUSDT"].Price, ap.MarketsStat["CCCUSDT"].Time = priceC, minute
		ap.updateChangePrices()
		minute = minute.Add(time.Minute)
	}

	tick()

	// BBB/CCC (рынок) стабильно растут на 2% за тик, AAA стоит на месте -
	// значит AAA стабильно ОТСТАЁТ от рынка на одну и ту же величину.
	for i := 0; i < relativeStrengthMinSamples+2; i++ {
		priceB *= 1.02
		priceC *= 1.02
		tick()
	}

	// Резкий провал: AAA внезапно падает, рынок продолжает расти как обычно.
	for i := 0; i < relativeStrengthConfirmMinStreak; i++ {
		priceA *= 0.80 // -20% за тик
		priceB *= 1.02
		priceC *= 1.02
		tick()
	}

	side, confirmed := ap.GetRelativeStrengthConfirmedSide("AAAUSDT", "1m")
	if !confirmed || side != "SELL" {
		t.Fatalf("ожидался Confirmed=SELL после устойчивого отставания от рынка, получено side=%q confirmed=%v", side, confirmed)
	}
}
