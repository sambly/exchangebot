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

func NewPricesDb(db *gorm.DB) *pricesDb {
	return &pricesDb{db: db}
}

func (r *pricesDb) InsertCandle(candle exModel.Candle, period string) error {
	start := time.Now()
	result := r.db.Table(fmt.Sprintf("%s%s", candlesTables, period)).Create(&candle)
	duration := time.Since(start).Seconds()

	status := "success"
	if result.Error != nil {
		status = "error"
	}

	pricesDbOperationDuration.WithLabelValues("insert_candle", status).Observe(duration)
	pricesDbOperationTotal.WithLabelValues("insert_candle", status).Inc()

	return result.Error
}

func (r *pricesDb) InsertCandles(candles []exModel.Candle, period string) error {
	if len(candles) == 0 {
		return nil
	}
	start := time.Now()
	result := r.db.Table(fmt.Sprintf("%s%s", candlesTables, period)).Create(&candles)
	duration := time.Since(start).Seconds()

	status := "success"
	if result.Error != nil {
		status = "error"
	}

	pricesDbOperationDuration.WithLabelValues("insert_candles", status).Observe(duration)
	pricesDbOperationTotal.WithLabelValues("insert_candles", status).Inc()

	return result.Error
}

func (r *pricesDb) GetCandlesByPeriod(period string) ([]exModel.Candle, error) {
	start := time.Now()
	var candles []exModel.Candle
	result := r.db.Table(fmt.Sprintf("%s%s", candlesTables, period)).Find(&candles)
	duration := time.Since(start).Seconds()

	status := "success"
	if result.Error != nil {
		status = "error"
	}

	pricesDbOperationDuration.WithLabelValues("get_candles_by_period", status).Observe(duration)
	pricesDbOperationTotal.WithLabelValues("get_candles_by_period", status).Inc()

	if result.Error != nil {
		return nil, result.Error
	}

	for i := range candles {
		candles[i].AmountTradeAsk = candles[i].AmountTrade - candles[i].AmountTradeBuy
		candles[i].ActiveAskVolume = candles[i].Volume - candles[i].ActiveBuyVolume
	}
	return candles, nil
}

func (r *pricesDb) GetCandlesBySymbol(symbol string, period string, quantity int) (model.Quote, error) {
	start := time.Now()
	var candles []exModel.Candle

	tableName := fmt.Sprintf("%s%s", candlesTables, period)
	result := r.db.
		Table(tableName).
		Where("pair = ?", symbol).
		Order("time DESC").
		Limit(int(quantity)).
		Find(&candles)

	duration := time.Since(start).Seconds()
	status := "success"
	if result.Error != nil {
		status = "error"
	}

	pricesDbOperationDuration.WithLabelValues("get_candles_by_symbol_and_period", status).Observe(duration)
	pricesDbOperationTotal.WithLabelValues("get_candles_by_symbol_and_period", status).Inc()

	if result.Error != nil {
		return model.Quote{}, result.Error
	}

	ohlcv := model.Quote{
		Date:   make([]time.Time, len(candles)),
		Open:   make([]float64, len(candles)),
		High:   make([]float64, len(candles)),
		Low:    make([]float64, len(candles)),
		Close:  make([]float64, len(candles)),
		Volume: make([]float64, len(candles)),
	}

	// Переворачиваем результат, чтобы вернуть в хронологическом порядке (от старого к новому)
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}

	for i, candle := range candles {
		ohlcv.Date[i] = candle.Time
		ohlcv.Open[i] = candle.Open
		ohlcv.High[i] = candle.High
		ohlcv.Low[i] = candle.Low
		ohlcv.Close[i] = candle.Close
		ohlcv.Volume[i] = candle.Volume
	}

	return ohlcv, nil
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
		candles[i].AmountTradeAsk = candles[i].AmountTrade - candles[i].AmountTradeBuy
		candles[i].ActiveAskVolume = candles[i].Volume - candles[i].ActiveBuyVolume
	}

	return candles, nil
}

func (r *pricesDb) SelectDeltaPeriod(pair string, period string) ([]model.ChangeDeltaForCandle, error) {
	start := time.Now()
	var candles []model.ChangeDeltaForCandle

	err := r.db.Table(fmt.Sprintf("%s%s", candlesTables, period)).
		Where("pair = ?", pair).
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
		candles[i].TradesAsk = candles[i].Trades - candles[i].TradesBuy
		candles[i].VolumeAsk = candles[i].Volume - candles[i].VolumeBuy
	}

	return candles, nil
}
