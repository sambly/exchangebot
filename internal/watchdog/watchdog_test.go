package watchdog

import (
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/prices"
)

type stubRepo struct{}

func (stubRepo) SelectMarketStateTimev2(time.Time) ([]exModel.Candle, error) { return nil, nil }
func (stubRepo) SelectDeltaPeriod(string, string) ([]model.ChangeDeltaForCandle, error) {
	return nil, nil
}
func (stubRepo) SelectCandlesFromPeriod(string, time.Time) ([]exModel.Candle, error) {
	return nil, nil
}

func newTestWatchdog(t *testing.T, pairs []string) (*Watchdog, *prices.AssetsPrices) {
	t.Helper()

	periods := map[string]time.Duration{"15m": 15 * time.Minute}
	ap, err := prices.NewAssetsPrices(pairs, periods, periods, stubRepo{})
	if err != nil {
		t.Fatalf("NewAssetsPrices: %v", err)
	}

	return New(ap, pairs), ap
}

// Подписка на пару может умереть в exchangeService, и приложение этого не
// заметит: оно продолжит работать на застывших данных. Сторож ловит это по
// свежести MarketsStat[pair].Time.
func TestWatchdogDetectsSilentPair(t *testing.T) {
	pairs := []string{"BTCUSDT", "ETHUSDT"}
	w, ap := newTestWatchdog(t, pairs)
	w.startedAt = time.Now().Add(-time.Hour) // grace-период давно прошёл

	now := time.Now()
	const liquid = 900_000_000.0 // суточный оборот, чтобы пары считались ликвидными

	// Обе пары только что прислали данные - тишины нет
	ap.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 100, Volume: liquid, Time: now})
	ap.OnMarket(exModel.MarketsStat{Pair: "ETHUSDT", Price: 200, Volume: liquid, Time: now})

	w.check(now)
	if len(w.stale) != 0 {
		t.Fatalf("на свежих данных замолчавших пар быть не должно: %v", w.stale)
	}

	// BTCUSDT замолчал (дольше порога), ETHUSDT продолжает слать
	later := now.Add(20 * time.Minute)
	ap.OnMarket(exModel.MarketsStat{Pair: "ETHUSDT", Price: 210, Volume: liquid, Time: later})

	w.check(later)
	if !w.stale["BTCUSDT"] {
		t.Fatal("BTCUSDT молчит 10 минут - должен быть помечен")
	}
	if w.stale["ETHUSDT"] {
		t.Fatal("ETHUSDT продолжает слать данные - помечать его нельзя")
	}

	// Пара ожила - отметка снимается
	revived := later.Add(2 * time.Minute)
	ap.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 105, Volume: liquid, Time: revived})
	ap.OnMarket(exModel.MarketsStat{Pair: "ETHUSDT", Price: 215, Volume: liquid, Time: revived})

	w.check(revived)
	if len(w.stale) != 0 {
		t.Fatalf("после восстановления данных отметки должны быть сняты: %v", w.stale)
	}
}

// Неликвид и стейблкоины молчат просто потому, что по ним нет сделок. Сторож на
// них ругаться не должен: за 17 часов такие пары выдали 359 ложных тревог.
func TestWatchdogIgnoresIlliquidPairs(t *testing.T) {
	pairs := []string{"BTCUSDT", "DEADUSDT"}
	w, ap := newTestWatchdog(t, pairs)
	w.startedAt = time.Now().Add(-time.Hour)

	now := time.Now()

	// Обе пары давно молчат, но у неликвида крошечный суточный оборот
	ap.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 100, Volume: 900_000_000, Time: now.Add(-time.Hour)})
	ap.OnMarket(exModel.MarketsStat{Pair: "DEADUSDT", Price: 1, Volume: 1_000, Time: now.Add(-time.Hour)})

	w.check(now)

	if !w.stale["BTCUSDT"] {
		t.Error("ликвидная пара, молчащая час, - это мёртвая подписка, о ней надо сообщать")
	}
	if w.stale["DEADUSDT"] {
		t.Error("неликвид молчит потому, что по нему нет сделок: тревожить не о чем")
	}
}

// Если пара уже помечена молчащей, её нельзя терять из виду только потому, что
// у неё протух суточный оборот.
func TestWatchdogKeepsWatchingAlreadyStalePair(t *testing.T) {
	w, ap := newTestWatchdog(t, []string{"BTCUSDT"})
	w.startedAt = time.Now().Add(-time.Hour)

	now := time.Now()

	// Пара замолчала, будучи ликвидной
	ap.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 100, Volume: 900_000_000, Time: now.Add(-time.Hour)})
	w.check(now)
	if !w.stale["BTCUSDT"] {
		t.Fatal("пара должна быть помечена молчащей")
	}

	// Данные вернулись, но оборот в них уже низкий - восстановление всё равно
	// должно быть замечено
	ap.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 100, Volume: 1_000, Time: now})
	w.check(now)

	if w.stale["BTCUSDT"] {
		t.Fatal("данные снова идут - отметку надо снять, а не терять пару из виду")
	}
}

// На старте подписки поднимаются не мгновенно: ругаться на пары, по которым
// ещё не было ни одного тика, в первые минуты нельзя.
func TestWatchdogSilentDuringStartGrace(t *testing.T) {
	w, _ := newTestWatchdog(t, []string{"BTCUSDT"})
	w.startedAt = time.Now()

	w.check(time.Now().Add(time.Minute))

	if len(w.stale) != 0 {
		t.Fatalf("во время grace-периода пары не помечаются: %v", w.stale)
	}
}

// Пара, по которой не пришло НИ ОДНОГО тика, - это отдельное состояние:
// скорее всего у неё вообще нет подписки (exchange_service отвечает "no such
// pair"). Раньше она валилась в общее сообщение "тишина > 15m", хотя никакой
// тишины не было - данных не было вовсе.
func TestWatchdogReportsPairWithoutTicksAfterGrace(t *testing.T) {
	w, _ := newTestWatchdog(t, []string{"DEADUSDT"})
	w.startedAt = time.Now().Add(-time.Hour)

	w.check(time.Now())

	if !w.noData["DEADUSDT"] {
		t.Fatal("пара без единого тика должна попасть в noData")
	}
	if w.stale["DEADUSDT"] {
		t.Fatal("пара без единого тика - это не 'замолчала', в stale ей не место")
	}
}

// О паре без подписки сообщаем ОДИН раз: ждать от неё нечего, мигать нечем.
func TestWatchdogReportsMissingPairOnlyOnce(t *testing.T) {
	w, _ := newTestWatchdog(t, []string{"DEADUSDT"})
	w.startedAt = time.Now().Add(-time.Hour)

	w.check(time.Now())
	first := len(w.noData)

	w.check(time.Now().Add(time.Minute))

	if len(w.noData) != first {
		t.Fatalf("состояние не должно меняться: было %d, стало %d", first, len(w.noData))
	}
}

// Если по паре без подписки внезапно пошли данные - она возвращается в обычный
// режим наблюдения.
func TestWatchdogPairWithoutTicksThenRecovers(t *testing.T) {
	w, ap := newTestWatchdog(t, []string{"BTCUSDT"})
	w.startedAt = time.Now().Add(-time.Hour)

	w.check(time.Now())
	if !w.noData["BTCUSDT"] {
		t.Fatal("пара без тиков должна попасть в noData")
	}

	now := time.Now()
	ap.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 100, Volume: 900_000_000, Time: now})
	w.check(now)

	if w.noData["BTCUSDT"] {
		t.Fatal("данные пошли - пару надо убрать из noData")
	}
}
