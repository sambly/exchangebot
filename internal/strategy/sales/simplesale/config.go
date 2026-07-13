package simplesale

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name           string `yaml:"name"`
	IDName         string `yaml:"idName"`
	Description    string `yaml:"description"`
	StrategyEnable bool   `yaml:"strategyEnable"`

	// Пороги в единицах волатильности пары (робастная сигма движения за период).
	// Именно они и используются, когда сигнал знает волатильность.
	TakeProfitVol float64 `yaml:"takeProfitVol"`
	StopLossVol   float64 `yaml:"stopLossVol"`

	// Откат в процентах - для сигналов, которые волатильность не считают (base)
	TakeProfitPercent float64 `yaml:"takeProfitPercent"`
	StopLossPercent   float64 `yaml:"stopLossPercent"`

	// Границы: тейк ниже комиссий бессмыслен, стоп шире разумного - это уже не
	// стоп, а надежда.
	MinTakeProfitPercent float64 `yaml:"minTakeProfitPercent"`
	MaxTakeProfitPercent float64 `yaml:"maxTakeProfitPercent"`
	MinStopLossPercent   float64 `yaml:"minStopLossPercent"`
	MaxStopLossPercent   float64 `yaml:"maxStopLossPercent"`

	// MaxHoldPeriods - сколько периодов сигнала держим позицию, если ни тейк,
	// ни стоп не сработали. Ноль - без ограничения по времени.
	MaxHoldPeriods int `yaml:"maxHoldPeriods"`
}

func NewConfig() (*Config, error) {
	var config Config

	primaryPath := "internal/strategy/sales/simplesale/config.yaml"
	fallbackPath := "internal/strategy/sales/simplesale/config.example.yaml"

	fileData, err := os.ReadFile(primaryPath)
	if err != nil {
		fileData, err = os.ReadFile(fallbackPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read both config.yaml and config.example.yaml: %w", err)
		}
	}

	if err := yaml.Unmarshal(fileData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Значения по умолчанию: без них молчаливо получилась бы позиция без стопа.
	if config.TakeProfitVol == 0 {
		config.TakeProfitVol = 2.0
	}
	if config.StopLossVol == 0 {
		config.StopLossVol = 1.5
	}
	if config.TakeProfitPercent == 0 {
		config.TakeProfitPercent = 2.0
	}
	if config.StopLossPercent == 0 {
		config.StopLossPercent = 1.5
	}
	if config.MinTakeProfitPercent == 0 {
		config.MinTakeProfitPercent = 0.5
	}
	if config.MaxTakeProfitPercent == 0 {
		config.MaxTakeProfitPercent = 20.0
	}
	if config.MinStopLossPercent == 0 {
		config.MinStopLossPercent = 0.5
	}
	if config.MaxStopLossPercent == 0 {
		config.MaxStopLossPercent = 10.0
	}
	if config.MaxHoldPeriods == 0 {
		config.MaxHoldPeriods = 4
	}

	return &config, nil
}
