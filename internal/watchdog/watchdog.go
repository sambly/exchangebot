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
	Help: "Количество пар, которые присылали данные и замолчали",
})

var noDataPairsGauge = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "market_feed_no_data_pairs",
	Help: "Количество пар, по которым не пришло ни одного тика (скорее всего нет подписки)",
})

// Watchdog пишет только в логи и в метрику market_feed_stale_pairs:
// в Telegram это не уведомление для человека, а сигнал для мониторинга.
type Watchdog struct {
	assetsPrices *prices.AssetsPrices
	pairs        []string

	startedAt time.Time
	// stale - пары, которые присылали данные и замолчали. Нужен, чтобы писать
	// о переходе (замолчала / снова заговорила), а не каждую минуту об одном и том же.
	stale map[string]bool
	// noData - пары, по которым не пришло НИ ОДНОГО тика. Это другое состояние:
	// такая пара не "замолчала", у неё скорее всего просто нет подписки, и
	// сообщать о ней надо один раз, а не мигать ею вечно.
	noData map[string]bool
}

func New(assetsPrices *prices.AssetsPrices, pairs []string) *Watchdog {
	return &Watchdog{
		assetsPrices: assetsPrices,
		pairs:        pairs,
		stale:        make(map[string]bool),
		noData:       make(map[string]bool),
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

	// Три РАЗНЫХ состояния, которые раньше сваливались в одно сообщение
	// "тишина > 15m" - и оно врало про пары, по которым данных не было вообще.
	wentSilent := make([]string, 0)  // была живой и замолчала
	neverStarted := make([]string, 0) // не прислала ни одного тика с запуска
	recovered := make([]string, 0)

	for _, pair := range w.pairs {
		stat, ok := stats[pair]
		hasData := ok && !stat.Time.IsZero()

		// Пара не прислала НИ ОДНОГО тика. Это не "замолчала" - это скорее всего
		// вообще нет подписки: exchange_service на такие пары отвечает
		// "no such pair" (делистинг, опечатка в pairs.txt). Сообщаем один раз и
		// больше не трогаем: ждать от неё нечего, а мигать ей нечем.
		if !hasData {
			if now.Sub(w.startedAt) < startGrace {
				continue // подписки поднимаются не мгновенно
			}
			if !w.noData[pair] {
				w.noData[pair] = true
				neverStarted = append(neverStarted, pair)
			}
			continue
		}

		// Данные по паре пришли - значит про подписку мы больше не гадаем
		delete(w.noData, pair)

		// Неликвид не сторожим: по нему просто нет сделок, и молчание - норма.
		// Оборот берём из MarketsStat.Volume (QuoteVolume за 24ч, сразу в USDT).
		// Пару, УЖЕ помеченную молчащей, ведём до восстановления, иначе она тихо
		// исчезнет из-под наблюдения вместе с протухшим оборотом.
		if stat.Volume < minDailyVolume && !w.stale[pair] {
			continue
		}

		fresh := now.Sub(stat.Time) <= staleAfter

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
	noDataPairsGauge.Set(float64(len(w.noData)))

	if len(neverStarted) > 0 {
		sort.Strings(neverStarted)
		watchdogLogger.Errorf("нет подписки: по %d парам не пришло ни одного тика с запуска: %s",
			len(neverStarted), formatPairs(neverStarted))
	}

	if len(wentSilent) > 0 {
		sort.Strings(wentSilent)
		watchdogLogger.Errorf("данные пропали: %d пар молчат дольше %v: %s",
			len(wentSilent), staleAfter, formatPairs(wentSilent))
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
