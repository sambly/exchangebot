package base

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name               string `yaml:"name"`
	IDName             string `yaml:"idName"`
	Description        string `yaml:"description"`
	StrategyEnable     bool   `yaml:"strategyEnable"`
	NotificationEnable bool   `yaml:"notificationEnable"`

	Pairs          []string           `yaml:"pairs"`
	AllPairs       bool               `yaml:"allPairs"`
	WeightProcents map[string]float64 `yaml:"weightProcents"`
}

func NewConfig() (*Config, error) {
	var config Config

	primaryPath := "internal/strategy/base/config.yaml"
	fallbackPath := "internal/strategy/base/config.example.yaml"

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
	return &config, nil
}
