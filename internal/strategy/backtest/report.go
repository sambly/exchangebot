package backtest

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Trade - одна завершённая сделка симуляции
type Trade struct {
	Pair      string
	Side      string
	Level     int
	Direction string
	OpenTime  time.Time
	CloseTime time.Time
	Reason    string
	GrossPct  float64
	NetPct    float64
}

// Report - результат прогона.
//
// Все проценты - на сделку, БЕЗ реинвестирования (сумма, а не произведение):
// так проще сравнивать варианты стратегии между собой, а именно для этого
// бэктест и нужен. Абсолютную доходность депозита он не предсказывает.
type Report struct {
	Period string
	Pairs  int
	From   time.Time
	To     time.Time

	// Signals - сколько сигналов дал детектор (до фильтров входа)
	Signals int
	// Rejected - почему сигналы не стали сделками (причина - счётчик)
	Rejected map[string]int

	// Лимитный вход (если включён): сколько заявок выставлено, исполнено и
	// снято по TTL. Разница placed-filled-expired - заявки, до чьей цены бар
	// дотянулся, но вход запретили риск-правила.
	LimitPlaced  int
	LimitFilled  int
	LimitExpired int

	Trades []Trade

	// Считается в finalize
	Wins, Losses int
	SumNetPct    float64
	SumGrossPct  float64
	ProfitFactor float64
	MaxDrawdown  float64
	AvgHold      time.Duration
}

func (r *Report) addTrade(trade Trade) {
	r.Trades = append(r.Trades, trade)
}

func (r *Report) finalize() {
	sumWin, sumLoss := 0.0, 0.0
	equity, peak, maxDD := 0.0, 0.0, 0.0
	var holdTotal time.Duration

	// Просадка считается по сделкам в порядке ЗАКРЫТИЯ - это и есть хронология
	// результата.
	sorted := make([]Trade, len(r.Trades))
	copy(sorted, r.Trades)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CloseTime.Before(sorted[j].CloseTime) })

	for _, t := range sorted {
		r.SumNetPct += t.NetPct
		r.SumGrossPct += t.GrossPct
		holdTotal += t.CloseTime.Sub(t.OpenTime)

		if t.NetPct > 0 {
			r.Wins++
			sumWin += t.NetPct
		} else {
			r.Losses++
			sumLoss += t.NetPct
		}

		equity += t.NetPct
		if equity > peak {
			peak = equity
		}
		if dd := peak - equity; dd > maxDD {
			maxDD = dd
		}
	}

	r.MaxDrawdown = maxDD
	if sumLoss != 0 {
		r.ProfitFactor = sumWin / math.Abs(sumLoss)
	}
	if len(r.Trades) > 0 {
		r.AvgHold = holdTotal / time.Duration(len(r.Trades))
	}
}

// Render - текстовый отчёт. Плоский и грепаемый, как логи стратегии: его
// сравнивают между прогонами, а не разглядывают.
func (r *Report) Render() string {
	var b strings.Builder

	fmt.Fprintf(&b, "=== Бэктест %s: %s — %s, пар: %d ===\n",
		r.Period, r.From.Format("2006-01-02"), r.To.Format("2006-01-02"), r.Pairs)
	fmt.Fprintf(&b, "сигналов: %d, сделок: %d\n", r.Signals, len(r.Trades))
	if r.LimitPlaced > 0 {
		fmt.Fprintf(&b, "лимитные заявки: выставлено %d, исполнено %d (%.0f%%), снято по TTL %d\n",
			r.LimitPlaced, r.LimitFilled, float64(r.LimitFilled)/float64(r.LimitPlaced)*100, r.LimitExpired)
	}
	b.WriteString("\n")

	if len(r.Trades) == 0 {
		b.WriteString("Сделок нет.\n")
		r.renderRejected(&b)
		return b.String()
	}

	winrate := float64(r.Wins) / float64(len(r.Trades)) * 100
	fmt.Fprintf(&b, "итог (сумма, без реинвеста): net %+.2f%%  (gross %+.2f%%, комиссии+проскальзывание съели %.2f%%)\n",
		r.SumNetPct, r.SumGrossPct, r.SumGrossPct-r.SumNetPct)
	fmt.Fprintf(&b, "winrate: %.1f%% (%d/%d)  profit factor: %.2f  средняя сделка: %+.3f%%\n",
		winrate, r.Wins, len(r.Trades), r.ProfitFactor, r.SumNetPct/float64(len(r.Trades)))
	fmt.Fprintf(&b, "max drawdown: %.2f%%  среднее удержание: %s\n\n", r.MaxDrawdown, r.AvgHold.Round(time.Minute))

	r.renderBreakdown(&b, "по причине выхода", func(t Trade) string { return t.Reason })
	r.renderBreakdown(&b, "по уровню сигнала", func(t Trade) string { return fmt.Sprintf("level %d", t.Level) })
	r.renderBreakdown(&b, "по направлению", func(t Trade) string { return t.Direction })

	r.renderPairs(&b)
	r.renderRejected(&b)

	return b.String()
}

func (r *Report) renderBreakdown(b *strings.Builder, title string, key func(Trade) string) {
	type row struct {
		count int
		sum   float64
		wins  int
	}
	rows := make(map[string]*row)
	for _, t := range r.Trades {
		k := key(t)
		if rows[k] == nil {
			rows[k] = &row{}
		}
		rows[k].count++
		rows[k].sum += t.NetPct
		if t.NetPct > 0 {
			rows[k].wins++
		}
	}

	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Fprintf(b, "%s:\n", title)
	for _, k := range keys {
		v := rows[k]
		fmt.Fprintf(b, "  %-14s %4d сделок  net %+8.2f%%  winrate %5.1f%%  средняя %+.3f%%\n",
			k, v.count, v.sum, float64(v.wins)/float64(v.count)*100, v.sum/float64(v.count))
	}
	b.WriteString("\n")
}

func (r *Report) renderPairs(b *strings.Builder) {
	sums := make(map[string]float64)
	counts := make(map[string]int)
	for _, t := range r.Trades {
		sums[t.Pair] += t.NetPct
		counts[t.Pair]++
	}

	pairs := make([]string, 0, len(sums))
	for p := range sums {
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(i, j int) bool { return sums[pairs[i]] > sums[pairs[j]] })

	top := 5
	if len(pairs) < top {
		top = len(pairs)
	}

	b.WriteString("лучшие пары:\n")
	for _, p := range pairs[:top] {
		fmt.Fprintf(b, "  %-12s %+8.2f%% (%d сделок)\n", p, sums[p], counts[p])
	}
	b.WriteString("худшие пары:\n")
	for i := len(pairs) - 1; i >= len(pairs)-top && i >= 0; i-- {
		fmt.Fprintf(b, "  %-12s %+8.2f%% (%d сделок)\n", pairs[i], sums[pairs[i]], counts[pairs[i]])
	}
	b.WriteString("\n")
}

func (r *Report) renderRejected(b *strings.Builder) {
	if len(r.Rejected) == 0 {
		return
	}

	reasons := make([]string, 0, len(r.Rejected))
	for reason := range r.Rejected {
		reasons = append(reasons, reason)
	}
	sort.Slice(reasons, func(i, j int) bool { return r.Rejected[reasons[i]] > r.Rejected[reasons[j]] })

	b.WriteString("сигналы, не ставшие сделками:\n")
	for _, reason := range reasons {
		fmt.Fprintf(b, "  %6d  %s\n", r.Rejected[reason], reason)
	}
}
