package cobra

import (
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/spf13/viper"
)

var reloadLogger = logger.AddFields(map[string]interface{}{
	"package": "cobra",
})

var previousConfig map[string]interface{}
var lastConfigChange time.Time

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

	// Настройка отслеживания изменений
	viper.OnConfigChange(func(e fsnotify.Event) {

		// Добавляем дебаунс, чтобы реагировать только на одно событие изменения
		now := time.Now()
		if now.Sub(lastConfigChange) < time.Second*2 {
			// Игнорируем событие, если прошло менее 1 секунды с последнего
			return
		}
		lastConfigChange = now

		reloadLogger.Infof("Config file changed: %s", e.Name)

		if err := viper.ReadInConfig(); err != nil {
			reloadLogger.Errorf("error reading config file: %v", err)
		}

		newConfig := viper.AllSettings()

		fmt.Println(newConfig["log"])

		// cfg := &config.Config{}
		// if err := viper.Unmarshal(&cfg); err != nil {
		// 	fmt.Println("ERR")
		// }

		// fmt.Printf("Debug %v", cfg.Log.Debug)
		// fmt.Println()

		// Сравниваем старую и новую конфигурации
		compareConfigs(previousConfig, newConfig)

		// Обновляем предыдущее состояние конфигурации
		previousConfig = newConfig

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
			cfg.Log.Debug = newValue.(bool)
			logger.LoggerSetLevel(cfg.Log.Debug)
			reloadLogger.Infof("logger init log-debug to %v\n", cfg.Log.Debug)

		case "production":
			cfg.Log.Production = newValue.(bool)
			logger.LoggerSetFormatter(cfg.Log.Production)
			reloadLogger.Infof("logger init log-production to %v\n", cfg.Log.Production)
		}
	}

	// config.PrintConfig(cfg, "")
}

func printSettings(settings map[string]interface{}, indent int) {
	for key, value := range settings {
		// Отступы для форматирования
		indentation := ""
		for i := 0; i < indent; i++ {
			indentation += "  "
		}

		// Выводим ключ и значение
		switch v := value.(type) {
		case map[string]interface{}:
			fmt.Printf("%s%s:\n", indentation, key)
			printSettings(v, indent+1)
		default:
			fmt.Printf("%s%s: %v\n", indentation, key, v)
		}
	}
}
