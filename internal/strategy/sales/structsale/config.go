package structsale

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name           string `yaml:"name"`
	IDName         string `yaml:"idName"`
	Description    string `yaml:"description"`
	StrategyEnable bool   `yaml:"strategyEnable"`

	// Пороги в единицах волатильности пары - см. simplesale.Config, та же
	// математика для базового плана.
	TakeProfitVol float64 `yaml:"takeProfitVol"`
	StopLossVol   float64 `yaml:"stopLossVol"`

	// Откат в процентах - для сигналов, которые волатильность не считают.
	TakeProfitPercent float64 `yaml:"takeProfitPercent"`
	StopLossPercent   float64 `yaml:"stopLossPercent"`

	// Границы для посчитанных порогов - см. simplesale.Config.
	MinTakeProfitPercent float64 `yaml:"minTakeProfitPercent"`
	MaxTakeProfitPercent float64 `yaml:"maxTakeProfitPercent"`
	MinStopLossPercent   float64 `yaml:"minStopLossPercent"`
	MaxStopLossPercent   float64 `yaml:"maxStopLossPercent"`

	// MaxHoldPeriods - см. simplesale.Config.
	MaxHoldPeriods int `yaml:"maxHoldPeriods"`

	// StrengthScalePerLevel - на сколько увеличивать ТЕЙК (не стоп) за
	// каждый уровень силы сигнала выше 1 (anomaly Level 1..3): level=2 даёт
	// x(1+StrengthScalePerLevel), level=3 - x(1+2*StrengthScalePerLevel).
	// Только тейк, а не оба порога: у более сильной аномалии обычно есть
	// запас двигаться дальше, но это не повод рисковать шире на входе.
	// 0 - масштабирование выключено, поведение как у simplesale.
	StrengthScalePerLevel float64 `yaml:"strengthScalePerLevel"`

	// TrailingActivationPercent - минимальная прибыль (% от цены входа), по
	// достижении которой включается трейлинг-стоп. До этого порога позиция
	// ведёт себя как в simplesale: фиксированный тейк/стоп/таймаут. Пока
	// прибыль не набежала, трейлить нечего - тесный трейлинг с самого входа
	// выбивал бы позицию первым же шумом.
	// 0 - трейлинг выключен вовсе, поведение как у simplesale.
	TrailingActivationPercent float64 `yaml:"trailingActivationPercent"`

	// TrailingCallbackPercent - сколько процентных пунктов прибыли можно
	// отдать от лучшего значения с момента входа (не от входа!), прежде чем
	// трейлинг закроет позицию. Меньше - фиксирует больше прибыли, но чаще
	// выбивает шумом на откате; больше - даёт цене больше свободы дышать.
	TrailingCallbackPercent float64 `yaml:"trailingCallbackPercent"`

	// UseStructuralLevels - учитывать ли стены стакана (entrysetup.GetQuality)
	// как источник тейка/стопа вместо волатильности. Работает ТОЛЬКО в бою -
	// у стакана нет истории для бэктеста, поэтому в backtest.go эта фича
	// физически не может сработать (entrySetup=nil), даже если здесь true.
	UseStructuralLevels bool `yaml:"useStructuralLevels"`

	// MinStructuralScore - минимальное соотношение дальней стены к ближней
	// (Quality.Score), при котором структурный план вообще рассматривается.
	// Ниже - стены есть, но расположены слишком близко друг к другу: тесная
	// цель, узкий стоп, это не лучше волатильностной оценки, а хуже.
	MinStructuralScore float64 `yaml:"minStructuralScore"`

	// PartialTakeProfitPercent - на какой ДОЛЕ дистанции до полного тейка
	// (0..1, от position.TakeProfitPercent - конкретной цели уже посчитанного
	// плана, а не заново от волатильности) закрывать часть позиции. 0 -
	// частичный тейк выключен.
	//
	// РАБОТАЕТ ТОЛЬКО В БЭКТЕСТЕ: реальное частичное закрытие ордера на бирже
	// не реализовано (OrderController.ClosePosition закрывает позицию только
	// целиком) - Execute/ShouldExit в бою эту фичу не используют вовсе.
	PartialTakeProfitPercent float64 `yaml:"partialTakeProfitPercent"`

	// PartialTakeProfitFraction - какую долю ОТ ПЕРВОНАЧАЛЬНОГО объёма
	// позиции закрывать на этом уровне (0..1, эксклюзивно). Остальное
	// продолжает жить по общим правилам (тейк/трейлинг/стоп/таймаут).
	PartialTakeProfitFraction float64 `yaml:"partialTakeProfitFraction"`

	// AdaptiveTimeoutMinProfitPercent - если на исходном дедлайне прибыль
	// позиции (% от входа) НИЖЕ этого порога, закрываем по таймауту как
	// обычно - сигнал не сыграл, держать нечего. Если прибыль на дедлайне УЖЕ
	// достигла порога, дедлайн продлевается на AdaptiveTimeoutExtensionPeriods
	// вместо принудительного закрытия работающей сделки. 0 - адаптивность
	// выключена, дедлайн всегда жёсткий (как у simplesale).
	AdaptiveTimeoutMinProfitPercent float64 `yaml:"adaptiveTimeoutMinProfitPercent"`

	// AdaptiveTimeoutExtensionPeriods - на сколько ДОПОЛНИТЕЛЬНЫХ периодов
	// сигнала продлевать дедлайн при срабатывании адаптивного правила.
	// Продление ОДНОРАЗОВОЕ на позицию (см. ShouldExit) - иначе позиция,
	// зависшая в небольшом перманентном плюсе, никогда не закрылась бы по
	// времени вовсе.
	AdaptiveTimeoutExtensionPeriods int `yaml:"adaptiveTimeoutExtensionPeriods"`
}

func NewConfig() (*Config, error) {
	var config Config

	primaryPath := "internal/strategy/sales/structsale/config.yaml"
	fallbackPath := "internal/strategy/sales/structsale/config.example.yaml"

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

	// Значения по умолчанию - см. simplesale.NewConfig: без них молчаливо
	// получилась бы позиция без стопа. Трейлинг и масштабирование по силе
	// включены ненулевыми дефолтами намеренно - иначе смысл заводить именно
	// этот модуль (а не просто продолжать пользоваться simplesale) теряется.
	if config.TakeProfitVol == 0 {
		config.TakeProfitVol = 2.0
	}
	if config.StopLossVol == 0 {
		config.StopLossVol = 1.5
	}
	if config.TakeProfitPercent == 0 {
		config.TakeProfitPercent = 2.0
	}
	if config.StopLossPercent == 0 {
		config.StopLossPercent = 1.5
	}
	if config.MinTakeProfitPercent == 0 {
		config.MinTakeProfitPercent = 0.5
	}
	if config.MaxTakeProfitPercent == 0 {
		config.MaxTakeProfitPercent = 20.0
	}
	if config.MinStopLossPercent == 0 {
		config.MinStopLossPercent = 0.5
	}
	if config.MaxStopLossPercent == 0 {
		config.MaxStopLossPercent = 10.0
	}
	if config.MaxHoldPeriods == 0 {
		config.MaxHoldPeriods = 4
	}

	// StrengthScalePerLevel/TrailingActivationPercent/TrailingCallbackPercent
	// НЕ дефолтятся здесь при значении 0, в отличие от полей выше: у них 0 -
	// осмысленный, явный выбор "эта фича выключена" (см. комментарии у полей
	// Config), а не "забыли настроить". Если завести дефолт и на них, конфиг
	// с явным нулём тихо получил бы включённую фичу - разумные ненулевые
	// значения для файла-по-умолчанию лежат в config.example.yaml.

	return &config, nil
}
