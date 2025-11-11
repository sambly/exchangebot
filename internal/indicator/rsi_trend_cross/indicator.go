package rsitrendcross

import (
	"fmt"
	"math"
	"time"

	"github.com/markcheno/go-talib"
	"github.com/sambly/exchangebot/internal/model"
)

type IndicatorData struct {
	Date  time.Time `json:"date"`
	Value float64   `json:"value"`
}

type Indicator struct {
	*Config
	SignalBuyPoints  []IndicatorData
	SignalSellPoints []IndicatorData
	RSIValues        []float64
	EMAValues        []float64
	MinBars          int
}

func NewIndicator() (*Indicator, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	// --- Минимальное количество баров ---
	minBars := maxInt(cfg.RSILength, cfg.EMASlowLength, 20) + 10
	return &Indicator{
		Config:           cfg,
		SignalBuyPoints:  make([]IndicatorData, 0),
		SignalSellPoints: make([]IndicatorData, 0),
		RSIValues:        make([]float64, 0),
		EMAValues:        make([]float64, 0),
		MinBars:          minBars,
	}, nil
}

func (s *Indicator) CheckData(candles model.Quote) (done bool) {

	len := len(candles.Close)
	if len < s.MinBars {
		fmt.Printf("[rsitrendcross] Недостаточно баров для анализа: имеется %d, требуется %d\n", len, s.MinBars)
		return false
	}
	return true
}

func (s *Indicator) Execute(candles model.Quote, verbose bool) (signalBuyOnLast, signalSellOnLast bool) {

	if !s.CheckData(candles) {
		return false, false
	}

	// --- Очистка сигналов ---
	s.SignalBuyPoints = s.SignalBuyPoints[:0]
	s.SignalSellPoints = s.SignalSellPoints[:0]

	closes := candles.Close
	times := candles.Date
	n := len(closes)

	// --- Индикаторы ---
	rsi := talib.Rsi(closes, s.RSILength)
	emaSlow := talib.Ema(closes, s.EMASlowLength)
	emaFast := talib.Ema(closes, 20)

	// Сохраняем для анализа
	s.RSIValues = rsi
	s.EMAValues = emaSlow

	startIndex := maxInt(s.RSILength, s.EMASlowLength, 20)
	lastBuyIndex := -9999
	lastSellIndex := -9999

	for i := startIndex; i < n; i++ {
		if i >= len(rsi) || i >= len(emaSlow) || i >= len(emaFast) {
			continue
		}
		if math.IsNaN(rsi[i]) || math.IsNaN(rsi[i-1]) ||
			math.IsNaN(emaSlow[i]) || math.IsNaN(emaSlow[i-1]) ||
			math.IsNaN(emaFast[i]) {
			continue
		}
		if math.IsInf(rsi[i], 0) || math.IsInf(emaSlow[i], 0) || math.IsInf(emaFast[i], 0) {
			continue
		}

		currClose := closes[i]
		prevClose := closes[i-1]
		currEMA := emaSlow[i]
		prevEMA := emaSlow[i-1]
		currRSI := rsi[i]
		prevRSI := rsi[i-1]
		currEMAFast := emaFast[i]

		dateStr := times[i].Format("2006-01-02 15:04")

		// --- BUY CONDITIONS ---
		buyCond1 := prevClose < prevEMA && currClose > currEMA         // пересечение ценой медленной EMA снизу вверх (трендовый сигнал)
		buyCond2 := prevRSI < s.RSIBuyLevel && currRSI > s.RSIBuyLevel // RSI пересекает уровень покупки снизу вверх → фильтр импульса (моментум)
		buyCond3 := currEMAFast > currEMA                              // быстрая EMA выше медленной
		buyCond4 := i-lastBuyIndex >= s.MinBarsBetweenTrades           //минимальное расстояние между покупками

		buySignal := buyCond1 && buyCond2 && buyCond3 && buyCond4

		if buySignal {
			lastBuyIndex = i
			s.SignalBuyPoints = append(s.SignalBuyPoints, IndicatorData{
				Date:  times[i],
				Value: currClose,
			})
			if i == n-1 {
				signalBuyOnLast = true
			}

			if verbose {
				fmt.Printf("[BUY] %s | Close=%.2f | RSI=%.1f | EMA=%.2f | FastEMA=%.2f\n", dateStr, currClose, currRSI, currEMA, currEMAFast)
				fmt.Printf("      cond1(cross up EMA)=%v cond2(RSI zone)=%v cond3(Fast>Slow)=%v cond4(cooldown)=%v\n",
					buyCond1, buyCond2, buyCond3, buyCond4)
			}
			continue
		}

		// --- SELL CONDITIONS ---
		sellCond1 := prevClose > prevEMA && currClose < currEMA           // цена пересекла EMA сверху вниз → сигнал разворота тренда
		sellCond2 := prevRSI > s.RSIExitLevel && currRSI < s.RSIExitLevel //RSI пересёк уровень выхода сверху вниз → сигнал ослабления импульса
		sellCond3 := i-lastSellIndex >= s.MinBarsBetweenTrades            //защита от слишком частых сигналов
		sellCond4 := currEMAFast < currEMA                                // быстрая EMA ниже медленной

		trueCount := 0
		if sellCond1 {
			trueCount++
		}
		if sellCond2 {
			trueCount++
		}
		if sellCond3 {
			trueCount++
		}
		if sellCond4 {
			trueCount++
		}

		sellSignal := trueCount >= s.CountSellSignals

		if sellSignal {
			lastSellIndex = i
			s.SignalSellPoints = append(s.SignalSellPoints, IndicatorData{
				Date:  times[i],
				Value: currClose,
			})
			if i == n-1 {
				signalSellOnLast = true
			}

			if verbose {
				fmt.Printf("[SELL] %s | Close=%.2f | RSI=%.1f | EMA=%.2f | FastEMA=%.2f\n", dateStr, currClose, currRSI, currEMA, currEMAFast)
				fmt.Printf("       cond1(cross down EMA)=%v cond2(RSI down)=%v cond3(cooldown)=%v\n",
					sellCond1, sellCond2, sellCond3)
			}
		}
	}

	return signalBuyOnLast, signalSellOnLast
}

// --- helpers ---
func maxInt(vals ...int) int {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
