package levels

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ThresholdsConfig - пороги Score (см. entrysetup.LevelsSideScore) для
// уровня 1..3, тот же принцип, что у anomaly.ThresholdsConfig/
// potential.ThresholdsConfig, только применяется к соотношению дальнего
// свечного уровня к ближнему, а не к z-score.
type ThresholdsConfig struct {
	Level1 float64 `yaml:"level1"`
	Level2 float64 `yaml:"level2"`
	Level3 float64 `yaml:"level3"`
}

type Config struct {
	Name           string `yaml:"name"`
	IDName         string `yaml:"idName"`
	Description    string `yaml:"description"`
	StrategyEnable bool   `yaml:"strategyEnable"`

	AllPairs bool     `yaml:"allPairs"`
	Pairs    []string `yaml:"pairs"`

	// MinDailyVolume - тот же фильтр неликвида, что у anomaly/potential (см.
	// комментарий там): на мёртвой паре swing high/low легко находятся там,
	// где никто не торговал, а не там, где реально разворачивалась цена.
	MinDailyVolume float64 `yaml:"minDailyVolume"`

	// Periods - какие из глобальных периодов (settings.ChangePeriods)
	// проверять. entrysetup.GetAllPriceLevels считает их все, здесь -
	// подмножество, которое реально хотим торговать.
	Periods []string `yaml:"periods"`

	// Thresholds - пороги Score для уровня 1..3. Ниже Level1 - сигнала нет
	// вообще (тот же смысл, что у structsale.MinStructuralPriceLevelsScore:
	// уровни есть, но расположены слишком близко друг к другу - слабая
	// структура хуже её отсутствия).
	Thresholds ThresholdsConfig `yaml:"thresholds"`

	// ConfirmChecks - сколько тиков (AssetsPrices.Subscribe(), примерно раз в
	// минуту) сторона должна продержаться подряд, прежде чем стать сигналом -
	// защита от того, что кэш GetAllPriceLevels (см. priceLevelsCacheTTL,
	// 30с) обновился и швейцарский уровень на границе порога на мгновение
	// исчез/появился. Тот же принцип, что у anomaly.ConfirmChecks.
	ConfirmChecks int `yaml:"confirmChecks"`

	// CooldownMinutes - после публикации сигнала по паре+периоду, сколько
	// минут не публиковать повторно (если уровень не выше уже отправленного).
	// Исполнитель сам не даст открыть вторую позицию по той же паре
	// (Executor.riskAllows) - это защита от повторной публикации в
	// лог/подписчикам, пока цена держится у того же уровня.
	CooldownMinutes int `yaml:"cooldownMinutes"`

	// LogSignals - писать каждый опубликованный сигнал в лог. Тот же смысл,
	// что у potential.Config.LogSignals - нет ни Telegram-меню, ни дайджеста,
	// единственный канал наблюдения сейчас - лог.
	LogSignals bool `yaml:"logSignals"`
}

func NewConfig() (*Config, error) {
	var config Config

	primaryPath := "internal/strategy/levels/config.yaml"
	fallbackPath := "internal/strategy/levels/config.example.yaml"

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

	if config.MinDailyVolume == 0 {
		config.MinDailyVolume = 5_000_000
	}
	if len(config.Periods) == 0 {
		config.Periods = []string{"15m"}
	}
	if config.Thresholds.Level1 == 0 {
		config.Thresholds.Level1 = 1.5
	}
	if config.Thresholds.Level2 == 0 {
		config.Thresholds.Level2 = 2.5
	}
	if config.Thresholds.Level3 == 0 {
		config.Thresholds.Level3 = 4.0
	}
	if config.ConfirmChecks == 0 {
		config.ConfirmChecks = 2
	}
	if config.CooldownMinutes == 0 {
		config.CooldownMinutes = 15
	}

	return &config, nil
}
