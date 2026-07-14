package anomaly

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// tradingViewExchange - биржа в ссылке на график. Пары мы берём со спота
// Binance, поэтому префикс фиксированный.
const tradingViewExchange = "BINANCE"

// tradingViewURL - ссылка на график пары. Telegram сам делает такие ссылки
// кликабельными, поэтому разметка (и связанный с ней риск сломать отправку
// на неэкранированном символе) не нужна.
func tradingViewURL(pair string) string {
	return fmt.Sprintf("https://www.tradingview.com/chart/?symbol=%s:%s", tradingViewExchange, pair)
}

// iconForLevel возвращает иконку для уровня аномалии
func iconForLevel(level int) string {
	switch level {
	case 1:
		return "⚠️"
	case 2:
		return "🔴"
	case 3:
		return "🚨"
	default:
		return "ℹ️"
	}
}

func levelName(level int) string {
	switch level {
	case 1:
		return "Уровень 1"
	case 2:
		return "Уровень 2"
	case 3:
		return "Уровень 3"
	default:
		return ""
	}
}

// NotificationDigest отправляет все аномалии одного тика одним сообщением.
//
// Группируем по ПАРЕ, а не по периоду: одно и то же движение обычно видно сразу
// на нескольких периодах (15m и 1h), и разносить его по разным секциям значит
// заставлять читателя самому сопоставлять строки. Пара - это то, о чём человек
// принимает решение, поэтому она и есть единица дайджеста.
func (s *AnomalyStrategy) NotificationDigest(results []*AnomalyResult) {
	byPair := make(map[string][]*AnomalyResult)
	for _, result := range results {
		byPair[result.Pair] = append(byPair[result.Pair], result)
	}

	pairs := make([]string, 0, len(byPair))
	for pair := range byPair {
		pairs = append(pairs, pair)
	}

	// Сильнейшее - наверх: сначала по максимальному уровню, потом по |z|
	sort.Slice(pairs, func(i, j int) bool {
		levelI, zI := pairStrength(byPair[pairs[i]])
		levelJ, zJ := pairStrength(byPair[pairs[j]])
		if levelI != levelJ {
			return levelI > levelJ
		}
		if zI != zJ {
			return zI > zJ
		}
		return pairs[i] < pairs[j]
	})

	out := fmt.Sprintf("📊 Аномалии (%s): %d %s\n\n",
		time.Now().Format("15:04"), len(pairs), pairsWord(len(pairs)))

	shown := pairs
	if len(shown) > s.Config.DigestMaxItems {
		shown = shown[:s.Config.DigestMaxItems]
	}

	for _, pair := range shown {
		pairResults := byPair[pair]

		// Периоды внутри пары - от коротких к длинным
		sort.Slice(pairResults, func(i, j int) bool {
			return s.Periods[pairResults[i].Period] < s.Periods[pairResults[j].Period]
		})

		maxLevel, _ := pairStrength(pairResults)
		out += fmt.Sprintf("%s %s\n", iconForLevel(maxLevel), pair)

		for _, result := range pairResults {
			out += fmt.Sprintf("   %s%s z=%.1f | %s\n",
				result.Period, divergentTag(result), result.CompositeZ, anomalousMetrics(result))
		}

		// Ссылка - последней строкой блока: она нужна, чтобы открыть график,
		// а не чтобы читать её глазами.
		out += tradingViewURL(pair) + "\n\n"
	}

	if hidden := len(pairs) - len(shown); hidden > 0 {
		out += fmt.Sprintf("… и ещё %d %s\n", hidden, pairsWord(hidden))
	}

	s.Notification.SendMessage(out)
}

// divergentTag - пометка рассогласованного движения: цена сходила, а торговая
// активность была НИЖЕ обычной. Такое движение прошло по пустому стакану и чаще
// всего откатывается - читатель дайджеста должен видеть это сразу, а не
// принимать вынос за подтверждённый импульс.
func divergentTag(result *AnomalyResult) string {
	if result.Divergent {
		return " ⚡пустой стакан"
	}
	return ""
}

// pairStrength - максимальный уровень пары и максимальный |z| среди её периодов
func pairStrength(results []*AnomalyResult) (int, float64) {
	level := 0
	maxZ := 0.0
	for _, result := range results {
		if result.Level > level {
			level = result.Level
		}
		if z := math.Abs(result.CompositeZ); z > maxZ {
			maxZ = z
		}
	}
	return level, maxZ
}

// pairsWord склоняет слово "пара" под число
func pairsWord(count int) string {
	if count%100 >= 11 && count%100 <= 14 {
		return "пар"
	}
	switch count % 10 {
	case 1:
		return "пара"
	case 2, 3, 4:
		return "пары"
	default:
		return "пар"
	}
}

// anomalousMetrics перечисляет метрики, которые реально пробили порог, с их
// сырыми значениями - именно они объясняют, почему пара попала в дайджест.
func anomalousMetrics(result *AnomalyResult) string {
	names := make([]string, 0, len(result.Metrics))
	for name, metric := range result.Metrics {
		if metric.IsAnomaly {
			names = append(names, name)
		}
	}
	// Детерминированный порядок вывода (map iteration order случаен)
	sort.Strings(names)

	if len(names) == 0 {
		return "нет данных"
	}

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s %+.2f%%", name, result.Metrics[name].Value))
	}
	return strings.Join(parts, ", ")
}

// logAnomalies пишет найденные аномалии в лог - по строке на пару+период.
//
// Формат намеренно плоский и грепаемый (pair=... period=... z=...), а не
// красивый: лог читают не глазами, а grep'ом, когда надо понять, что было.
//
// Пишем ВСЁ, что нашли, и помечаем, ушло ли это в Telegram: notified=false
// означает "аномалия была, но её съел cooldown или minNotifyLevel". Именно эти
// строки и объясняют потом, почему в чате пусто, а движение было.
func (s *AnomalyStrategy) logAnomalies(all, notified []*AnomalyResult) {
	if len(all) == 0 {
		return
	}

	sent := make(map[string]bool, len(notified))
	for _, result := range notified {
		sent[result.Pair+"|"+result.Period] = true
	}

	sorted := make([]*AnomalyResult, len(all))
	copy(sorted, all)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Level != sorted[j].Level {
			return sorted[i].Level > sorted[j].Level
		}
		return math.Abs(sorted[i].CompositeZ) > math.Abs(sorted[j].CompositeZ)
	})

	for _, result := range sorted {
		anomalyLogger.Infof("anomaly pair=%s period=%s level=%d z=%.2f activityZ=%.2f divergent=%t notified=%t metrics=[%s]",
			result.Pair, result.Period, result.Level, result.CompositeZ,
			result.ActivityZ, result.Divergent,
			sent[result.Pair+"|"+result.Period], anomalousMetrics(result))
	}
}

// NotificationMarketAnomaly отправляет уведомление о рыночной аномалии
func (s *AnomalyStrategy) NotificationMarketAnomaly(period string, percent float64, anomalous, total int, topResults []AnomalyResult) {
	if s.Config.LogAnomalies {
		anomalyLogger.Infof("market-anomaly period=%s percent=%.1f anomalous=%d total=%d",
			period, percent, anomalous, total)
	}

	out := fmt.Sprintf("🌍 Рыночная аномалия (%s)\n", period)
	out += fmt.Sprintf("  %.1f%% пар аномальны (%d из %d)\n", percent, anomalous, total)

	if len(topResults) > 0 {
		out += "\nТоп аномалий:\n"
		for _, r := range topResults {
			out += fmt.Sprintf("%s %s z=%.1f [%s]\n%s\n",
				iconForLevel(r.Level), r.Pair, r.CompositeZ, levelName(r.Level), tradingViewURL(r.Pair))
		}
	}

	s.Notification.SendMessage(out)
}

