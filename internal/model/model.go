package model

import (
	"time"

	"gorm.io/datatypes"
)

type Settings struct {
	ServerName     string
	Pairs          []string
	Timeframe      string
	ChangePeriods  map[string]time.Duration
	DeltaPeriods   map[string]time.Duration
	WeightProcents map[string]float64
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

type Deal struct {
	Pair     string
	SideType string
	Frame    string
	Strategy string
	Comment  string
}

type OrderInfo struct {
	ID           uint           `gorm:"primarykey;autoIncrement"`
	IdOrder      uint           `gorm:"column:id_order"`
	Frame        string         `gorm:"column:frame"`
	Strategy     string         `gorm:"column:strategy"`
	Comment      string         `gorm:"column:comment"`
	MarketsStat  datatypes.JSON `gorm:"column:markets_stat"`
	ChangePrices datatypes.JSON `gorm:"column:change_prices"`
	DeltaFast    datatypes.JSON `gorm:"column:delta_fast"`
}
