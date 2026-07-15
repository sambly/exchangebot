package cobra

import (
	"fmt"
	"strings"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/database"
	"github.com/sambly/exchangebot/internal/strategy/anomaly"
	"github.com/sambly/exchangebot/internal/strategy/backtest"
	"github.com/sambly/exchangebot/internal/strategy/executor"
	"github.com/sambly/exchangebot/internal/strategy/sales/simplesale"
	"github.com/spf13/cobra"
)

// backtestCmd - прогон стратегии по историческим свечам из БД.
//
// Использует ТЕ ЖЕ боевые компоненты и ТЕ ЖЕ config.yaml, что и торговля:
// детекцию anomaly, правила входа executor, политику выхода simplesale.
// Единственное, чего бэктест не воспроизводит, - минутную гранулярность
// проверок (confirmChecks) и рыночную метрику: он видит одну точку на период.
var backtestCmd = &cobra.Command{
	Use:    "backtest",
	Short:  "Прогон стратегии anomaly по историческим свечам из БД",
	PreRun: preRun,
	RunE:   runBacktest,
}

var (
	backtestPeriod      string
	backtestDays        int
	backtestPairs       string
	backtestFee         float64
	backtestSlippage    float64
	backtestEntryOffset float64
	backtestEntryTTL    int
	backtestMaxShare    float64
)

func init() {
	backtestCmd.Flags().StringVar(&backtestPeriod, "period", "15m", "сигнальный период (таблица candles_{period})")
	backtestCmd.Flags().IntVar(&backtestDays, "days", 30, "глубина истории в днях")
	backtestCmd.Flags().StringVar(&backtestPairs, "pairs", "", "пары через запятую (пусто - все, что есть в БД)")
	backtestCmd.Flags().Float64Var(&backtestFee, "fee", 0.1, "комиссия за сторону, % (Binance spot taker = 0.1)")
	backtestCmd.Flags().Float64Var(&backtestSlippage, "slippage", 0.05, "проскальзывание на сторону, %")
	backtestCmd.Flags().Float64Var(&backtestEntryOffset, "entry-offset", 0, "лимитный вход: отступ от закрытия в волатильностях пары (0 - вход по рынку)")
	backtestCmd.Flags().IntVar(&backtestEntryTTL, "entry-ttl", 4, "лимитный вход: сколько баров живёт заявка")
	backtestCmd.Flags().Float64Var(&backtestMaxShare, "max-anomalous-share", 0, "фильтр режима: макс. доля аномальных пар за такт (0.05 = 5%; 0 - выключен)")

	RootCmd.AddCommand(backtestCmd)
}

func runBacktest(cmd *cobra.Command, args []string) error {
	cfg, err := config.NewConfig()
	if err != nil {
		return err
	}

	db, err := database.DbInit(cfg.Database)
	if err != nil {
		return err
	}
	repo := database.NewPricesDb(db)

	// Та же карта периодов, что в root.go: детектор берёт из неё длительности
	periods := map[string]time.Duration{
		"1m":  time.Minute,
		"3m":  3 * time.Minute,
		"15m": 15 * time.Minute,
		"1h":  time.Hour,
		"4h":  4 * time.Hour,
		"12h": 12 * time.Hour,
	}
	periodDuration, ok := periods[backtestPeriod]
	if !ok {
		return fmt.Errorf("неизвестный период %q", backtestPeriod)
	}

	from := time.Now().Add(-time.Duration(backtestDays) * 24 * time.Hour)
	candles, err := repo.SelectCandlesFromPeriod(backtestPeriod, from)
	if err != nil {
		return fmt.Errorf("чтение свечей %s: %w", backtestPeriod, err)
	}
	if len(candles) == 0 {
		return fmt.Errorf("в candles_%s нет данных с %s", backtestPeriod, from.Format("2006-01-02"))
	}

	candles = filterPairs(candles, backtestPairs)
	pairs := distinctPairs(candles)

	detector, err := anomaly.NewBacktestDetector(periods, pairs)
	if err != nil {
		return fmt.Errorf("детектор anomaly: %w", err)
	}

	entry, err := executor.NewConfig()
	if err != nil {
		return fmt.Errorf("конфиг executor: %w", err)
	}

	// Политике выхода OrderController нужен только для боевого Execute;
	// бэктест зовёт чистый ShouldExit.
	exits, err := simplesale.NewStrategy(nil)
	if err != nil {
		return fmt.Errorf("политика выхода: %w", err)
	}

	engine := backtest.New(detector, entry, exits, backtest.Options{
		Period:            backtestPeriod,
		PeriodDuration:    periodDuration,
		Periods:           periods,
		FeePercent:        backtestFee,
		SlippagePercent:   backtestSlippage,
		EntryOffsetVol:    backtestEntryOffset,
		EntryTTLBars:      backtestEntryTTL,
		MaxAnomalousShare: backtestMaxShare,
	})

	report := engine.Run(candles)
	fmt.Println(report.Render())
	return nil
}

func filterPairs(candles []exModel.Candle, csv string) []exModel.Candle {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return candles
	}

	wanted := make(map[string]bool)
	for _, pair := range strings.Split(csv, ",") {
		wanted[strings.ToUpper(strings.TrimSpace(pair))] = true
	}

	filtered := candles[:0]
	for _, c := range candles {
		if wanted[c.Pair] {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func distinctPairs(candles []exModel.Candle) []string {
	seen := make(map[string]bool)
	pairs := make([]string, 0)
	for _, c := range candles {
		if !seen[c.Pair] {
			seen[c.Pair] = true
			pairs = append(pairs, c.Pair)
		}
	}
	return pairs
}
