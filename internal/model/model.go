package model

import (
	"time"
)

type Settings struct {
	ServerName    string
	Pairs         []string
	Timeframe     string
	ChangePeriods map[string]time.Duration
	DeltaPeriods  map[string]time.Duration
}
type ChangeDeltaForCandle struct {
	Time      time.Time
	Volume    float64
	VolumeBuy float64
	VolumeAsk float64
	Trades    int64
	TradesBuy int64
	TradesAsk int64

	// For Candles
	Open  float64
	High  float64
	Low   float64
	Close float64
}
