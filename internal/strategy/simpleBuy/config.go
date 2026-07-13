package simplebuy

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sambly/exchangebot/internal/strategy/signal"
)

type Config struct {
	Name           string `yaml:"name"`
	IDName         string `yaml:"idName"`
	Description    string `yaml:"description"`
	StrategyEnable bool   `yaml:"strategyEnable"`

	// Auto - входить без подтверждения в Telegram.
	// Без риск-лимитов ниже автомат на одном движении откроет десяток позиций по
	// одной паре по всё более высокой цене, поэтому они обязательны.
	Auto bool `yaml:"auto"`

	// Direction - какие сигналы торгуем: up (рост), down (падение), both.
	Direction string `yaml:"direction"`

	// MinLevel - минимальная сила сигнала для входа (у anomaly это уровень 1..3)
	MinLevel int `yaml:"minLevel"`

	// Sources - от каких детекторов принимаем сигналы. Пусто - от всех.
	Sources []string `yaml:"sources"`

	// Size - размер сделки
	Size float64 `yaml:"size"`

	// MaxPositions - сколько позиций держим одновременно суммарно.
	// На рыночном движении аномальными становятся десятки пар разом.
	MaxPositions int `yaml:"maxPositions"`

	// PairCooldownMinutes - сколько ждать после закрытия позиции по паре,
	// прежде чем открывать по ней новую. Защита от "лестницы" по одной паре.
	PairCooldownMinutes int `yaml:"pairCooldownMinutes"`
}

// Allows проверяет, подходит ли сигнал под фильтры конфига
func (c *Config) Allows(sig signal.Signal) (string, bool) {
	if sig.Level < c.MinLevel {
		return fmt.Sprintf("уровень %d < minLevel %d", sig.Level, c.MinLevel), false
	}

	if !c.allowsDirection(sig.Direction) {
		return fmt.Sprintf("направление %s не торгуем (direction=%s)", sig.Direction, c.Direction), false
	}

	if len(c.Sources) > 0 {
		allowed := false
		for _, source := range c.Sources {
			if strings.EqualFold(source, sig.Source) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Sprintf("источник %s не в списке sources", sig.Source), false
		}
	}

	return "", true
}

func (c *Config) allowsDirection(direction signal.Direction) bool {
	switch strings.ToLower(c.Direction) {
	case "both":
		return true
	case "down":
		return direction == signal.DirectionDown
	default: // "up"
		return direction == signal.DirectionUp
	}
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

	// Дефолты подобраны так, чтобы случайно включённый auto не наделал бед:
	// только сильные сигналы, только рост, одна позиция на пару, потолок по
	// количеству позиций.
	if config.Direction == "" {
		config.Direction = "up"
	}
	if config.MinLevel == 0 {
		config.MinLevel = 2
	}
	if config.Size == 0 {
		config.Size = 1.0
	}
	if config.MaxPositions == 0 {
		config.MaxPositions = 5
	}
	if config.PairCooldownMinutes == 0 {
		config.PairCooldownMinutes = 60
	}

	return &config, nil
}
