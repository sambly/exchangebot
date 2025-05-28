package base

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Pairs              []string           `yaml:"pairs"`
	AllPairs           bool               `yaml:"allPairs"`
	WeightProcents     map[string]float64 `yaml:"weightProcents"`
	NotificationEnable bool               `yaml:"notificationEnable"`
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
