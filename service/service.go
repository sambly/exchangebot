package service

import (
	"context"
	"main/model"
	"time"
)

type Exchange interface {
	Feeder
}

type Feeder interface {
	AssetsInfo(pair string) model.AssetInfo
	LastQuote(ctx context.Context, pair string) (float64, error)
	CandlesByPeriod(ctx context.Context, pair, period string, start, end time.Time) ([]model.Candle, error)
	CandlesByLimit(ctx context.Context, pair, period string, limit int) ([]model.Candle, error)
	CandlesSubscription(ctx context.Context, pair, timeframe string) (chan model.Candle, chan error)
	GetPairsToUSDT() ([]string, error)
	GetAssetsSpot(ctx context.Context) ([]model.AssetData, error)
	GetAssetsFlexible(ctx context.Context) ([]model.AssetData, error)
	GetAssetsStaking(ctx context.Context) ([]model.AssetData, error)
}
