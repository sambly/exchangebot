package simpleindicator

import (
	"time"

	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/prices"
)

type Strategy struct {
	Config       *Config
	Notification *notification.Notification

	Periods      map[string]time.Duration
	AssetsPrices *prices.AssetsPrices
}

func NewStrategy(assetsPrices *prices.AssetsPrices, periods map[string]time.Duration, pairs []string, notify *notification.Notification) (*Strategy, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}

	str := &Strategy{
		AssetsPrices: assetsPrices,
		Periods:      periods,
		Config:       cfg,
		Notification: notify,
	}
	return str, nil
}

// RSICheck checks the RSI value for a given asset and sets a flag if it reaches a certain level.
func (s *Strategy) RSICheck(asset string, rsiThreshold float64) bool {
	// Calculate RSI based on historical price data from AssetsPrices.
	rsiValue := s.calculateRSI(asset)

	// Temporary variable to indicate if RSI threshold is reached.
	isRSIReached := rsiValue >= rsiThreshold

	if isRSIReached {
		// Using simple string concatenation for now.
		s.Notification.SendMessage("RSI threshold reached for asset " + asset + ": " + s.floatToString(rsiValue))
	}

	return isRSIReached
}

// calculateRSI calculates the Relative Strength Index (RSI) for a given asset using historical price data.
func (s *Strategy) calculateRSI(asset string) float64 {
	// RSI period, typically 14 candles.
	const rsiPeriod = 14

	// Access historical price data for the asset.
	// Assuming we use the shortest period available for price changes.
	var selectedPeriod string
	for period := range s.Periods {
		if selectedPeriod == "" || s.Periods[period] < s.Periods[selectedPeriod] {
			selectedPeriod = period
		}
	}

	if selectedPeriod == "" {
		return 0.0 // No period available, return default.
	}

	// Note: Direct access to dataset is not possible due to unexported field.
	// This is a simplified placeholder. In a real implementation, we would need
	// to access historical data through a public method or adjust the structure.
	// For now, return a dummy value to avoid compilation errors.
	// TODO: Implement proper data retrieval for RSI calculation.
	return 70.0 // Temporary dummy value until proper data access is implemented.
}

// floatToString converts a float64 to a string with 2 decimal places.
func (s *Strategy) floatToString(value float64) string {
	// Simple conversion of float to string with limited decimal places.
	intPart := int(value)
	fracPart := int((value - float64(intPart)) * 100)
	if fracPart < 0 {
		fracPart = -fracPart
	}
	return s.intToString(intPart) + "." + s.intToString(fracPart)
}

// intToString converts an integer to a string.
func (s *Strategy) intToString(value int) string {
	if value == 0 {
		return "0"
	}
	if value < 0 {
		return "-" + s.intToString(-value)
	}
	digits := "0123456789"
	result := ""
	for value > 0 {
		result = string(digits[value%10]) + result
		value /= 10
	}
	return result
}
