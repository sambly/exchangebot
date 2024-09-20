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

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("error writing config file: %v", err)
	}

	// Сохраняем текущее состояние конфигурации
	previousConfig = viper.AllSettings()

	// Сброс чтобы убрать флаг AutomaticEnv, так как переменные считываются с него минуя ReadInConfig
	// TODO BindPFlags  нужно ли это сюда добавлять? учитывая что я полностью скидываю viper
	viper.Reset()
	viper.SetConfigFile(filename)

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

		// Сравниваем старую и новую конфигурации
		compareConfigs(previousConfig, newConfig)

		// Обновляем предыдущее состояние конфигурации
		previousConfig = newConfig
	})
	viper.WatchConfig()
	return nil
}

func compareConfigs(oldConfig, newConfig map[string]interface{}) {
	for key, newValue := range newConfig {
		oldValue, exists := oldConfig[key]
		if !exists {
			reloadLogger.Infof("New parameter added: %s = %v\n", key, newValue)
		} else if oldValue != newValue {
			reloadLogger.Infof("Parameter changed: %s changed from %v to %v\n", key, oldValue, newValue)
			updateConfigField(key, newValue)
		}
	}
	for key := range oldConfig {
		if _, exists := newConfig[key]; !exists {
			reloadLogger.Infof("Parameter removed: %s\n", key)
		}
	}
}

// TODO добавить сюда изменяемые данные
func updateConfigField(key string, newValue interface{}) {
	switch key {
	case "debug-log":
		cfg.DebugLog = newValue.(bool)
		logger.LoggerSetLevel(cfg.DebugLog)
		reloadLogger.Infof("logger init debug-log to %v\n", cfg.DebugLog)

	case "production-log":
		cfg.ProductionLog = newValue.(bool)
		logger.LoggerSetFormatter(cfg.ProductionLog)
		reloadLogger.Infof("logger init production-log to %v\n", cfg.ProductionLog)

	}
}

// Красивый вывод настроек viper.AllSettings()
//
// fmt.Println("↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓")
// printSettings(viper.AllSettings(), 0)
// fmt.Println("↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑")
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
