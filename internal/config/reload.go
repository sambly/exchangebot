package config

import (
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

var reloadLogger = logger.AddFieldsEmpty()

// Config.yaml for hot reload
func ReloadConfig(filename string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error marshal config.yaml - %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("error writre file config.yaml - %v", err)
	}

	fmt.Println(cfg.String())

	viper.SetConfigFile(filename)

	// Настройка отслеживания изменений
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		reloadLogger.Infof("Config file changed: %s", e.Name)
		// Загрузка конфигурации
		if err := viper.Unmarshal(cfg); err != nil {
			reloadLogger.Errorf("Error unmarshaling config: %v\n", err)
			return
		}
		fmt.Println(cfg.String())
	})
	return nil
}
