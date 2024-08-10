package model

import "time"

type Settings struct {
	ServerName     string
	Pairs          []string
	Timeframe      string
	ChangePeriods  map[string]time.Duration
	DeltaPeriods   map[string]time.Duration
	WeightProcents map[string]float64
}
