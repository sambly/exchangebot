package potential

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ComponentsConfig - какие компоненты композита участвуют в голосовании, тот
// же смысл, что у чекбоксов "Потенциала" во фронте (DataStrength.vue). Отдельно
// от entrysetup.StrengthComponents (сырые наблюдения) - свод в композит и
// выбор состава - решение потребителя, см. комментарий в strength.go.
type ComponentsConfig struct {
	OFI              bool `yaml:"ofi"`
	VPIN             bool `yaml:"vpin"`
	Levels           bool `yaml:"levels"`
	RelativeStrength bool `yaml:"relativeStrength"`
	Regime           bool `yaml:"regime"`
}

// ThresholdsConfig - пороги |композита| для уровня 1..3, тот же принцип, что
// у anomaly.ThresholdsConfig (см. LevelFromZScore там) - только здесь порог
// применяется к усреднённому, клипованному composite, а не к z-score одной
// метрики.
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

	// MinDailyVolume - тот же фильтр неликвида, что у anomaly (см. комментарий
	// там): без него z-score компонентов на мёртвой паре легко выглядит
	// внушительно просто потому, что база (объём/сделки) почти нулевая.
	MinDailyVolume float64 `yaml:"minDailyVolume"`

	// Periods - какие из глобальных периодов (settings.ChangePeriods)
	// проверять. entrysetup.GetAllStrengthComponents считает их все, здесь -
	// подмножество, которое реально хотим торговать.
	Periods []string `yaml:"periods"`

	Components ComponentsConfig `yaml:"components"`
	Thresholds ThresholdsConfig `yaml:"thresholds"`

	// MinTotal - минимум присутствующих (голосующих) компонентов, ниже
	// которого сигнал не формируется вообще, даже если единственный
	// присутствующий компонент даёт огромный z. Один голос - это не
	// консенсус, а мнение одной метрики; ровно то же соображение, из-за
	// которого во фронте есть CONTRIBUTION_CLAMP - защита от того, что один
	// выброс (см. ARCHITECTURE.md §10.5 про хвосты OFI) один решает всё.
	MinTotal int `yaml:"minTotal"`

	// MinAgree - минимум компонентов, СОГЛАСНЫХ с итоговым знаком композита
	// (agree/total во фронте). MinAgree <= MinTotal по построению.
	MinAgree int `yaml:"minAgree"`

	// ConfirmChecks - сколько проверок подряд (тик AssetsPrices.Subscribe(),
	// то же соображение о секундном шуме, что у anomaly.PeriodConfig.
	// ConfirmChecks) композит должен продержаться на текущей стороне, прежде
	// чем стать сигналом. 1 - как раньше, срабатывание с первого тика.
	ConfirmChecks int `yaml:"confirmChecks"`

	// CooldownMinutes - после публикации сигнала по паре+периоду, сколько
	// минут не публиковать повторно (если уровень не выше уже отправленного).
	// Исполнитель сам не даст открыть вторую позицию по той же паре
	// (Executor.riskAllows), это - защита от спама одинаковыми сигналами в
	// лог/подписчикам, пока композит держится выше порога.
	CooldownMinutes int `yaml:"cooldownMinutes"`

	// LogSignals - писать каждый опубликованный сигнал в лог. Единственный
	// канал наблюдения за стратегией сейчас - нет ни Telegram-меню, ни
	// дайджеста (см. Go-комментарий в potential.go), поэтому по умолчанию
	// включено.
	LogSignals bool `yaml:"logSignals"`
}

func NewConfig() (*Config, error) {
	var config Config

	primaryPath := "internal/strategy/potential/config.yaml"
	fallbackPath := "internal/strategy/potential/config.example.yaml"

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
		config.Thresholds.Level1 = 1.0
	}
	if config.Thresholds.Level2 == 0 {
		config.Thresholds.Level2 = 2.0
	}
	if config.Thresholds.Level3 == 0 {
		config.Thresholds.Level3 = 3.5
	}
	if config.MinTotal == 0 {
		config.MinTotal = 2
	}
	if config.MinAgree == 0 {
		config.MinAgree = 2
	}
	if config.ConfirmChecks == 0 {
		config.ConfirmChecks = 2
	}
	if config.CooldownMinutes == 0 {
		config.CooldownMinutes = 15
	}

	return &config, nil
}
