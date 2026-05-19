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
	Time      time.Time `gorm:"column:time"`
	Volume    float64   `gorm:"column:volume"`
	VolumeBuy float64   `gorm:"column:active_buy_volume"`
	VolumeAsk float64   `gorm:"-"`
	Trades    int64     `gorm:"column:amount_trade"`
	TradesBuy int64     `gorm:"column:amount_trade_buy"`
	TradesAsk int64     `gorm:"-"`
	Open      float64   `gorm:"column:open"`
	High      float64   `gorm:"column:high"`
	Low       float64   `gorm:"column:low"`
	Close     float64   `gorm:"column:close"`
}
