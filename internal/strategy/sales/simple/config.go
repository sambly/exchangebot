package simplebuy

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	NotificationEnable bool `yaml:"notificationEnable"`
}

func NewConfig() (*Config, error) {

	var config Config

	configPath := "internal/strategy/sales/simple/config.yaml"

	fileData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(fileData, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
