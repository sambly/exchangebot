package database

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/model"
	"gorm.io/gorm"
)

// Prometheus metrics for database operations
var (
	pricesDbOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "prices_database_operation_duration_seconds",
		Help:    "Duration of prices database operations in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation", "status"})

	pricesDbOperationTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "prices_database_operations_total",
		Help: "Total number of prices database operations",
	}, []string{"operation", "status"})
)

type pricesDb struct {
	db *gorm.DB
}

// tradesAskValue вычисляет число ask/sell-сделок как trades-tradesBuy. Но если
// tradesBuy==0 при trades>0 — это НЕ "все сделки на продажу", а отсутствие
// данных: бэкофилл и heal-on-close в feederapp берут данные из klines Binance,
// а klines не отдают число тейкер-buy сделок (см. exchangeService ARCHITECTURE.md
// §3.7 — amount_trade_buy для таких строк честно 0). Прямое trades-0 исказило бы
// картину (выглядело бы как 100% sell-давление), поэтому в этом случае
// возвращаем тот же признак "нет данных" — 0, а не искажённое значение.
// ActiveAskVolume/VolumeAsk (объём, не число сделок) этой проблемы не имеет:
// klines честно отдают taker-buy volume, поэтому Volume-VolumeBuy всегда корректно.
func tradesAskValue(trades, tradesBuy int64) int64 {
	if tradesBuy == 0 && trades > 0 {
		return 0
	}
	return trades - tradesBuy
}

func NewPricesDb(db *gorm.DB) *pricesDb {
	return &pricesDb{db: db}
}

func (r *pricesDb) SelectMarketStateTimev2(timeRounding time.Time) ([]exModel.Candle, error) {
	start := time.Now()
	var candles []exModel.Candle
	err := r.db.Table(fmt.Sprintf("%s%s", candlesTables, basePeriod)).
		Where("time >= ?", timeRounding).
		Order("time DESC").
		Find(&candles).Error
	duration := time.Since(start).Seconds()

	status := "success"
	if err != nil {
		status = "error"
	}

	pricesDbOperationDuration.WithLabelValues("select_market_state_time_v2", status).Observe(duration)
	pricesDbOperationTotal.WithLabelValues("select_market_state_time_v2", status).Inc()

	if err != nil {
		return nil, err
	}

	for i := range candles {
		candles[i].AmountTradeAsk = tradesAskValue(candles[i].AmountTrade, candles[i].AmountTradeBuy)
		candles[i].ActiveAskVolume = candles[i].Volume - candles[i].ActiveBuyVolume
	}

	return candles, nil
}

// SelectCandlesFromPeriod возвращает агрегированные свечи периода (таблица
// candles_{period}) начиная с указанного времени, по всем парам сразу.
// Используется для сидирования истории стратегий из БД при старте.
func (r *pricesDb) SelectCandlesFromPeriod(period string, from time.Time) ([]exModel.Candle, error) {
	start := time.Now()
	var candles []exModel.Candle

	err := r.db.Table(fmt.Sprintf("%s%s", candlesTables, period)).
		Where("time >= ?", from).
		Order("time DESC").
		Find(&candles).Error
	duration := time.Since(start).Seconds()

	status := "success"
	if err != nil {
		status = "error"
	}

	pricesDbOperationDuration.WithLabelValues("select_candles_from_period", status).Observe(duration)
	pricesDbOperationTotal.WithLabelValues("select_candles_from_period", status).Inc()

	if err != nil {
		return nil, err
	}

	for i := range candles {
		candles[i].AmountTradeAsk = tradesAskValue(candles[i].AmountTrade, candles[i].AmountTradeBuy)
		candles[i].ActiveAskVolume = candles[i].Volume - candles[i].ActiveBuyVolume
	}

	return candles, nil
}

func (r *pricesDb) SelectDeltaPeriod(pair string, period string) ([]model.ChangeDeltaForCandle, error) {
	start := time.Now()
	var candles []model.ChangeDeltaForCandle

	// GetDeltaPeriod (см. internal/prices/prices.go) идёт по этому срезу
	// последовательно вперёд и заполняет пропуски по времени - без ORDER BY
	// MySQL возвращает строки в порядке PK/вставки, который после бэкофилла
	// расходится с хронологическим, и lightweight-charts падает с "data must
	// be asc ordered by time".
	err := r.db.Table(fmt.Sprintf("%s%s", candlesTables, period)).
		Where("pair = ?", pair).
		Order("time ASC").
		Find(&candles).Error
	duration := time.Since(start).Seconds()

	status := "success"
	if err != nil {
		status = "error"
	}

	pricesDbOperationDuration.WithLabelValues("select_delta_period", status).Observe(duration)
	pricesDbOperationTotal.WithLabelValues("select_delta_period", status).Inc()

	if err != nil {
		return nil, err
	}

	for i := range candles {
		candles[i].TradesAsk = tradesAskValue(candles[i].Trades, candles[i].TradesBuy)
		candles[i].VolumeAsk = candles[i].Volume - candles[i].VolumeBuy
	}

	return candles, nil
}
