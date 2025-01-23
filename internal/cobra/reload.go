package cobra

import (
	"fmt"
	"strconv"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/spf13/viper"
)

var reloadLogger = logger.AddFields(map[string]interface{}{
	"package": "cobra",
})

var previousConfig map[string]interface{}

// Config.yaml for hot reload
func reloadConfig() error {

	viper.SetConfigFile(filenameConfigReload)

	if err := viper.WriteConfigAs(filenameConfigReload); err != nil {
		return fmt.Errorf("error writing config file: %v", err)
	}

	// Сохраняем текущее состояние конфигурации
	previousConfig = viper.AllSettings()

	// Сброс чтобы убрать флаг AutomaticEnv, так как переменные считываются с него минуя ReadInConfig
	viper.Reset()
	viper.SetConfigFile(filenameConfigReload)

	var debounceTimer *time.Timer
	// Настройка отслеживания изменений
	viper.OnConfigChange(func(e fsnotify.Event) {

		if debounceTimer != nil {
			debounceTimer.Stop()
		}

		debounceTimer = time.AfterFunc(2*time.Second, func() {
			reloadLogger.Infof("Config file changed: %s", e.Name)

			if err := viper.ReadInConfig(); err != nil {
				reloadLogger.Errorf("error reading config file: %v", err)
			}

			newConfig := viper.AllSettings()
			// Сравниваем старую и новую конфигурации
			compareConfigs(previousConfig, newConfig)
			// Обновляем предыдущее состояние конфигурации
			previousConfig = newConfig
		})

	})
	viper.WatchConfig()
	return nil
}

func compareConfigs(oldConfig, newConfig map[string]interface{}) {

	for keyMap, _ := range newConfig {
		if newValueMap, ok := newConfig[keyMap].(map[string]interface{}); ok {
			for key, newValue := range newValueMap {
				oldValue, exists := oldConfig[keyMap].(map[string]interface{})[key]
				if !exists {
					reloadLogger.Infof("Parameter missing : %s = %v\n", key, newValue)
				} else if oldValue != newValue {
					reloadLogger.Infof("Parameter changed: %s changed from %v to %v\n", key, oldValue, newValue)
					updateConfigField(keyMap, key, newValue)
				}
			}
		}
	}
}

func updateConfigField(keyMap, key string, newValue interface{}) {

	// добавить сюда изменяемые данные
	switch keyMap {
	case "log":
		switch key {
		case "debug":

			if boolValue, ok := newValue.(bool); ok {
				cfg.Log.Debug = boolValue
			} else if strValue, ok := newValue.(string); ok {
				parsedValue, _ := strconv.ParseBool(strValue)
				cfg.Log.Debug = parsedValue
			}
			logger.LoggerSetLevel(cfg.Log.Debug)
			reloadLogger.Infof("logger init log-debug to %v\n", cfg.Log.Debug)

		case "production":

			if boolValue, ok := newValue.(bool); ok {
				cfg.Log.Production = boolValue
			} else if strValue, ok := newValue.(string); ok {
				parsedValue, _ := strconv.ParseBool(strValue)
				cfg.Log.Production = parsedValue
			}
			logger.LoggerSetFormatter(cfg.Log.Production)
			reloadLogger.Infof("logger init log-production to %v\n", cfg.Log.Production)
		}
	}
}
