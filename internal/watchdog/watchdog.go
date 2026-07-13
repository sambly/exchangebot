// Package watchdog следит за тем, что рыночные данные реально идут по каждой паре.
//
// Зачем: в exchangeService горутина подписки на пару, исчерпав попытки
// переподключения, просто выходит. Фидер этой пары завершается, а приложение
// продолжает работать - StartMarketsStatFeeder вернёт ошибку только когда умрут
// ВСЕ пары. То есть одна пара может молча застыть на последней цене, а стратегии
// будут считать по ней метрики как ни в чём не бывало.
//
// Править exchangeService мы не можем, поэтому детектируем это снаружи: по
// свежести MarketsStat[pair].Time.
package watchdog

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/prices"
)

const (
	// checkInterval - как часто проверяем свежесть данных
	checkInterval = time.Minute

	// staleAfter - после какого молчания пара считается замолчавшей.
	//
	// Изначально здесь стояло 5 минут, исходя из того, что тикерный поток шлёт
	// обновление раз в секунду по любой паре. ЭТО ОКАЗАЛОСЬ НЕВЕРНО: по парам
	// без сделок обновлений нет минутами, и сторож выдал 359 ложных тревог за
	// 17 часов (стейблкоины TUSDUSDT/USDPUSDT, неликвид XNOUSDT/DGBUSDT...).
	// Порог поднят, а неликвид отсекается отдельно - см. minDailyVolume.
	staleAfter = 15 * time.Minute

	// minDailyVolume - минимальный суточный оборот пары в USDT.
	//
	// Главный фильтр ложных тревог. По ликвидной паре сделки идут постоянно, и
	// её молчание действительно означает мёртвую подписку. По неликвиду молчание
	// означает лишь то, что никто не торгует - о таком сообщать нечего.
	minDailyVolume = 5_000_000
	// startGrace - сколько ждём при старте, прежде чем ругаться на пары,
	// по которым не пришло ещё ни одного тика (подписки поднимаются не мгновенно)
	startGrace = 3 * time.Minute
	// maxPairsInLog - сколько пар перечислять в строке лога
	maxPairsInLog = 15
)

var watchdogLogger = logger.AddFields(map[string]interface{}{
	"package": "watchdog",
})

var stalePairsGauge = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "market_feed_stale_pairs",
	Help: "Количество пар, по которым не приходят рыночные данные",
})

// Watchdog пишет только в логи и в метрику market_feed_stale_pairs:
// в Telegram это не уведомление для человека, а сигнал для мониторинга.
type Watchdog struct {
	assetsPrices *prices.AssetsPrices
	pairs        []string

	startedAt time.Time
	// stale - пары, о молчании которых мы уже сообщили. Нужен, чтобы писать
	// о переходе (замолчала / снова заговорила), а не каждую минуту об одном и том же.
	stale map[string]bool
}

func New(assetsPrices *prices.AssetsPrices, pairs []string) *Watchdog {
	return &Watchdog{
		assetsPrices: assetsPrices,
		pairs:        pairs,
		stale:        make(map[string]bool),
	}
}

func (w *Watchdog) Start(ctx context.Context) error {
	w.startedAt = time.Now()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	watchdogLogger.Infof("сторож фида запущен: %d пар, порог тишины %v", len(w.pairs), staleAfter)

	for {
		select {
		case <-ticker.C:
			w.check(time.Now())
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// check сравнивает время последнего тика каждой пары с текущим и уведомляет
// о парах, которые замолчали или, наоборот, снова заговорили.
func (w *Watchdog) check(now time.Time) {
	stats := w.assetsPrices.GetAllMarketsStat()

	wentSilent := make([]string, 0)
	recovered := make([]string, 0)

	for _, pair := range w.pairs {
		stat, ok := stats[pair]

		// Неликвид не сторожим: по нему просто нет сделок, и молчание - норма.
		// Оборот берём из MarketsStat.Volume (это QuoteVolume за 24ч, сразу в USDT).
		//
		// Два исключения, и оба важны:
		//   - пара БЕЗ ЕДИНОГО тика (Time == 0): оборот у неё тоже нулевой, но это
		//     не признак неликвида - мы про неё просто ничего не знаем. Такая пара
		//     как раз и может быть мёртвой подпиской с самого старта;
		//   - пара, УЖЕ помеченная молчащей: ведём её до восстановления, иначе она
		//     тихо исчезнет из-под наблюдения вместе с протухшим оборотом.
		knownLiquidity := ok && !stat.Time.IsZero()
		if knownLiquidity && stat.Volume < minDailyVolume && !w.stale[pair] {
			continue
		}

		fresh := ok && !stat.Time.IsZero() && now.Sub(stat.Time) <= staleAfter

		// По паре не пришло ещё ни одного тика: на старте это нормально,
		// подписки поднимаются не мгновенно.
		if (!ok || stat.Time.IsZero()) && now.Sub(w.startedAt) < startGrace {
			continue
		}

		switch {
		case !fresh && !w.stale[pair]:
			w.stale[pair] = true
			wentSilent = append(wentSilent, pair)
		case fresh && w.stale[pair]:
			delete(w.stale, pair)
			recovered = append(recovered, pair)
		}
	}

	stalePairsGauge.Set(float64(len(w.stale)))

	if len(wentSilent) > 0 {
		sort.Strings(wentSilent)
		watchdogLogger.Errorf("нет рыночных данных (тишина > %v) по %d парам: %s",
			staleAfter, len(wentSilent), formatPairs(wentSilent))
	}

	if len(recovered) > 0 {
		sort.Strings(recovered)
		watchdogLogger.Infof("данные снова идут по %d парам: %s", len(recovered), formatPairs(recovered))
	}
}

func formatPairs(pairs []string) string {
	if len(pairs) > maxPairsInLog {
		return strings.Join(pairs[:maxPairsInLog], ", ") + ", …"
	}
	return strings.Join(pairs, ", ")
}
