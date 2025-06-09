package simplebuy

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	NotificationEnable bool `yaml:"notificationEnable"`
}

func NewConfig() (*Config, error) {
	var config Config

	primaryPath := "internal/strategy/simpleBuy/config.yaml"
	fallbackPath := "internal/strategy/simpleBuy/config.example.yaml"

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
