package anomaly

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type MetricsConfig struct {
	Price     bool `yaml:"price"`
	Volume    bool `yaml:"volume"`
	VolumeBuy bool `yaml:"volumeBuy"`
	VolumeAsk bool `yaml:"volumeAsk"`
	Trades    bool `yaml:"trades"`
	TradesBuy bool `yaml:"tradesBuy"`
	TradesAsk bool `yaml:"tradesAsk"`
}

type ThresholdsConfig struct {
	Level1 float64 `yaml:"level1"`
	Level2 float64 `yaml:"level2"`
	Level3 float64 `yaml:"level3"`
}

type PeriodConfig struct {
	Enabled    bool             `yaml:"enabled"`
	Thresholds ThresholdsConfig `yaml:"thresholds"`
	// HistoryWindowSize - опциональный override размера окна истории для
	// конкретного периода. Если 0 (не задан) - используется глобальный
	// Config.HistoryWindowSize. Нужен, потому что при записи в историю не
	// чаще раза в period (см. PeriodState.nextSampleAt в anomaly.go) время
	// заполнения буфера = HistoryWindowSize * period, и одно глобальное
	// значение может быть неудобным сразу для 15m и 1d.
	HistoryWindowSize int `yaml:"historyWindowSize"`

	// ConfirmChecks - сколько проверок подряд аномалия должна продержаться,
	// прежде чем стать сигналом. 1 (по умолчанию) - как раньше, срабатывание с
	// первой же минуты.
	//
	// Проверка идёт каждую минуту, а окна ChangePrices/ChangeDelta скользящие:
	// рынок дёрнулся на секунду и вернулся - в одной минутной проверке это
	// полноценная аномалия, и по ней открывается позиция уже после отката.
	//
	// Сглаживать z (например, EMA) для этого НЕЛЬЗЯ: соседние минутные z считаются
	// против одной и той же истории (она обновляется раз в период), то есть сильно
	// скоррелированы - усреднение даст в основном задержку. Хуже того, оно срежет
	// пик: настоящий level 3 после сглаживания прочитается как level 2 и не пройдёт
	// minLevel у исполнителя, то есть мы отфильтруем лучшие сигналы вместе с шумом.
	//
	// Поэтому здесь не усреднение, а требование УСТОЙЧИВОСТИ: аномалия должна
	// продержаться N проверок, а уровень берётся ПИКОВЫЙ за это окно. Цена вопроса -
	// задержка входа на N минут.
	ConfirmChecks int `yaml:"confirmChecks"`
}

type MarketMetricConfig struct {
	Enabled                 bool    `yaml:"enabled"`
	AnomalyPercentThreshold float64 `yaml:"anomalyPercentThreshold"`
	MinPairs                int     `yaml:"minPairs"`
}

type Config struct {
	Name               string                   `yaml:"name"`
	IDName             string                   `yaml:"idName"`
	Description        string                   `yaml:"description"`
	StrategyEnable     bool                     `yaml:"strategyEnable"`
	NotificationEnable bool                     `yaml:"notificationEnable"`
	AllPairs           bool                     `yaml:"allPairs"`
	Pairs              []string                 `yaml:"pairs"`
	HistoryWindowSize  int                      `yaml:"historyWindowSize"`
	MinSamples         int                      `yaml:"minSamples"`
	Metrics            MetricsConfig            `yaml:"metrics"`
	Periods            map[string]*PeriodConfig `yaml:"periods"`
	MarketMetric       MarketMetricConfig       `yaml:"marketMetric"`

	// MinChange - минимальное абсолютное изменение метрики (в сырых процентах),
	// ниже которого срабатывание гасится независимо от z-score. Без него
	// стейблкоины и низковолатильные пары дают честные z=5 на движении 0.01%:
	// статистически это аномалия, торговать там нечего.
	MinChange map[string]float64 `yaml:"minChange"`

	// MinScale - нижняя граница разброса (в шкале statValue: для price это
	// проценты, для дельт - логарифм отношения). Не даёт z взорваться до 30-80
	// на парах, где метрика почти всегда стоит на месте и MAD вырождается.
	MinScale map[string]float64 `yaml:"minScale"`

	// DigestMaxItems - сколько строк показывать в дайджесте на один период,
	// остальные схлопываются в "и ещё N пар".
	DigestMaxItems int `yaml:"digestMaxItems"`

	// MinDailyVolume - минимальный суточный оборот пары в USDT
	// (MarketsStat.Volume = QuoteVolume за 24ч). Пары ниже порога не проверяются
	// вообще. Это главный фильтр мусора по объёму: у неликвида объём предыдущего
	// окна близок к нулю, поэтому дельта даёт +87000% на паре, где два человека
	// что-то купили. Проблема не в величине изменения, а в величине базы, и
	// порогом minChange она не лечится.
	MinDailyVolume float64 `yaml:"minDailyVolume"`

	// MinNotifyLevel - минимальный уровень, начиная с которого аномалия попадает
	// в дайджест. Отсечённые уровни всё равно учитываются в рыночной метрике.
	MinNotifyLevel int `yaml:"minNotifyLevel"`

	// LogAnomalies - писать ли аномалии в лог (в дополнение к Telegram).
	// В лог попадают ВСЕ обнаруженные аномалии, включая подавленные cooldown'ом
	// и не дотянувшие до minNotifyLevel: в Telegram нужен читаемый дайджест,
	// а в логе - полная картина, по которой потом можно разобраться.
	LogAnomalies bool `yaml:"logAnomalies"`
}

// Дефолты порогов значимости. Дельты (volume*/trades*) - это отношение окон в
// процентах, поэтому 100 означает "объём должен измениться хотя бы вдвое".
var (
	defaultMinChange = map[string]float64{
		"price":     1.0,
		"volume":    100.0,
		"volumeBuy": 100.0,
		"volumeAsk": 100.0,
		"trades":    100.0,
		"tradesBuy": 100.0,
		"tradesAsk": 100.0,
	}

	defaultMinScale = map[string]float64{
		"price":     0.15, // % изменения цены за период
		"volume":    0.25, // в лог-шкале: ~ ±28%
		"volumeBuy": 0.25,
		"volumeAsk": 0.25,
		"trades":    0.25,
		"tradesBuy": 0.25,
		"tradesAsk": 0.25,
	}
)

func NewConfig() (*Config, error) {
	var config Config

	primaryPath := "internal/strategy/anomaly/config.yaml"
	fallbackPath := "internal/strategy/anomaly/config.example.yaml"

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

	// Значения по умолчанию
	if config.HistoryWindowSize == 0 {
		config.HistoryWindowSize = 30
	}
	if config.MinSamples == 0 {
		config.MinSamples = 20
	}
	if config.MarketMetric.AnomalyPercentThreshold == 0 {
		config.MarketMetric.AnomalyPercentThreshold = 30.0
	}
	if config.MarketMetric.MinPairs == 0 {
		config.MarketMetric.MinPairs = 5
	}
	if config.DigestMaxItems == 0 {
		config.DigestMaxItems = 10
	}
	if config.MinDailyVolume == 0 {
		config.MinDailyVolume = 5_000_000
	}
	if config.MinNotifyLevel == 0 {
		config.MinNotifyLevel = 1
	}

	// Пороги задаются пометрично, и незаданные добираются дефолтами: конфиг,
	// перечисливший только price, не должен обнулить защиту для объёмов.
	if config.MinChange == nil {
		config.MinChange = make(map[string]float64, len(defaultMinChange))
	}
	for metric, value := range defaultMinChange {
		if _, ok := config.MinChange[metric]; !ok {
			config.MinChange[metric] = value
		}
	}
	if config.MinScale == nil {
		config.MinScale = make(map[string]float64, len(defaultMinScale))
	}
	for metric, value := range defaultMinScale {
		if _, ok := config.MinScale[metric]; !ok {
			config.MinScale[metric] = value
		}
	}

	// Дефолтные пороги для периодов, если не заданы
	for _, pc := range config.Periods {
		if pc.Thresholds.Level1 == 0 {
			pc.Thresholds.Level1 = 3.0
		}
		if pc.Thresholds.Level2 == 0 {
			pc.Thresholds.Level2 = 5.0
		}
		if pc.Thresholds.Level3 == 0 {
			pc.Thresholds.Level3 = 8.0
		}
		if pc.ConfirmChecks < 1 {
			pc.ConfirmChecks = 1
		}
		// HistoryWindowSize намеренно НЕ дефолтим здесь нулём->глобальным
		// значением - 0 является валидным сигналом "использовать глобальный
		// Config.HistoryWindowSize", это разруливается в windowSizeForPeriod().
	}

	return &config, nil
}
