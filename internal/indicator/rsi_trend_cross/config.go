package rsitrendcross

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var (
	primaryPath  = "internal/indicator/rsi_trend_cross/config.yaml"
	fallbackPath = "internal/indicator/rrsi_trend_crosssi/config.default.yaml"
)

type Config struct {
	// Индикаторы
	RSILength     int `yaml:"rsi_length" json:"rsiLength"`          // длина RSI
	EMASlowLength int `yaml:"ema_slow_length" json:"emaSlowLength"` // длина медленной EMA

	// Уровни RSI
	RSIBuyLevel  float64 `yaml:"rsi_buy_level" json:"rsiBuyLevel"`   // уровень входа
	RSIExitLevel float64 `yaml:"rsi_exit_level" json:"rsiExitLevel"` // уровень выхода

	MinBarsBetweenTrades int `yaml:"min_bars_between_trades" json:"minBarsBetweenTrades"` // минимальное количество баров

	CountSellSignals int `yaml:"count_sell_signals" json:"count_sell_signals"`
}

func NewConfig() (*Config, error) {
	var config Config

	fileData, err := os.ReadFile(primaryPath)
	if err != nil {
		fileData, err = os.ReadFile(fallbackPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read both config.yaml and config.default.yaml: %w", err)
		}
	}

	if err := yaml.Unmarshal(fileData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &config, nil
}

func (c *Config) SaveConfig() error {

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(primaryPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
