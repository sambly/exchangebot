package database

import (
	"fmt"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/model"
	"gorm.io/gorm"
)

type pricesDb struct {
	db *gorm.DB
}

func NewPricesDb(db *gorm.DB) *pricesDb {
	return &pricesDb{db: db}
}

func (r *pricesDb) InsertCandle(candle exModel.Candle, period string) error {
	result := r.db.Table(fmt.Sprintf("%s%s", candlesTables, period)).Create(&candle)
	return result.Error
}

func (r *pricesDb) InsertCandles(candles []exModel.Candle, period string) error {
	if len(candles) == 0 {
		return nil
	}
	result := r.db.Table(fmt.Sprintf("%s%s", candlesTables, period)).Create(&candles)
	return result.Error
}

func (r *pricesDb) GetCandlesByPeriod(period string) ([]exModel.Candle, error) {

	var candles []exModel.Candle
	result := r.db.Table(fmt.Sprintf("%s%s", candlesTables, period)).Find(&candles)
	if result.Error != nil {
		return nil, result.Error
	}

	for i := range candles {
		candles[i].AmountTradeAsk = candles[i].AmountTrade - candles[i].AmountTradeBuy
		candles[i].ActiveAskVolume = candles[i].Volume - candles[i].ActiveBuyVolume

	}
	return candles, nil
}

func (r *pricesDb) SelectMarketStateTimev2(timeRounding time.Time) ([]exModel.Candle, error) {

	var candles []exModel.Candle
	err := r.db.Table(fmt.Sprintf("%s%s", candlesTables, basePeriod)).
		Where("time >= ?", timeRounding).
		Order("time DESC").
		Find(&candles).Error

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
	var candles []model.ChangeDeltaForCandle

	err := r.db.Table(fmt.Sprintf("%s%s", candlesTables, period)).
		Where("pair = ?", pair).
		Find(&candles).Error

	if err != nil {
		return nil, err
	}

	for i := range candles {
		candles[i].TradesAsk = candles[i].Trades - candles[i].TradesBuy
		candles[i].VolumeAsk = candles[i].Volume - candles[i].VolumeBuy
	}

	return candles, nil
}
