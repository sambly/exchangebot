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

	// UseStructuralPriceLevels - тот же принцип, что у UseStructuralLevels, но
	// источник структуры - НЕ стены стакана (entrysetup.GetQuality), а свечные
	// swing high/low (entrysetup.GetPriceLevels): стоп = дистанция до ближнего
	// уровня, тейк = до дальнего. Проверяется РАНЬШЕ UseStructuralLevels в
	// Plan - если оба включены для одного источника, приоритет у свечных
	// уровней. В отличие от стен стакана, эта фича работает и в бэктесте (у
	// свечей есть история, в отличие от стакана) - хотя backtest.Engine пока
	// её не подключает (см. Go-комментарий в начале файла).
	//
	// Задумана для источника levels (вход у тех же swing-уровней) - см.
	// config.example.yaml, секция sources - естественно выходить по той же
	// структуре, по которой был вход, а не по независимому снимку стакана.
	UseStructuralPriceLevels bool `yaml:"useStructuralPriceLevels"`

	// MinStructuralPriceLevelsScore - тот же смысл, что у MinStructuralScore,
	// для UseStructuralPriceLevels.
	MinStructuralPriceLevelsScore float64 `yaml:"minStructuralPriceLevelsScore"`

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

	// Sources - оверрайды параметров плана ПО ИСТОЧНИКУ СИГНАЛА (sig.Source),
	// см. SourceOverride. structsale - одна политика выхода на весь Executor
	// (WithSaleStrategy принимает ровно одну), а детекторы у него разные
	// (anomaly, potential, будущий levels-детектор от swing-уровней) - без
	// этого у них не было бы способа тюниться независимо иначе, чем через
	// второй Executor целиком. Источник без записи здесь - все поля берёт из
	// глобальных значений выше, как раньше.
	Sources map[string]SourceOverride `yaml:"sources"`
}

// SourceOverride - частичный оверрайд Config для одного sig.Source. Указатели,
// а не голые значения - иначе не отличить "не задано, бери глобальное" от
// "явно задано 0/false" (например, useStructuralLevels: false для источника,
// у которого глобально true). Поля названы и задокументированы 1:1 с Config -
// см. комментарии там же.
type SourceOverride struct {
	TakeProfitVol *float64 `yaml:"takeProfitVol"`
	StopLossVol   *float64 `yaml:"stopLossVol"`

	TakeProfitPercent *float64 `yaml:"takeProfitPercent"`
	StopLossPercent   *float64 `yaml:"stopLossPercent"`

	MinTakeProfitPercent *float64 `yaml:"minTakeProfitPercent"`
	MaxTakeProfitPercent *float64 `yaml:"maxTakeProfitPercent"`
	MinStopLossPercent   *float64 `yaml:"minStopLossPercent"`
	MaxStopLossPercent   *float64 `yaml:"maxStopLossPercent"`

	MaxHoldPeriods *int `yaml:"maxHoldPeriods"`

	StrengthScalePerLevel *float64 `yaml:"strengthScalePerLevel"`

	TrailingActivationPercent *float64 `yaml:"trailingActivationPercent"`
	TrailingCallbackPercent   *float64 `yaml:"trailingCallbackPercent"`

	UseStructuralLevels *bool    `yaml:"useStructuralLevels"`
	MinStructuralScore  *float64 `yaml:"minStructuralScore"`

	UseStructuralPriceLevels      *bool    `yaml:"useStructuralPriceLevels"`
	MinStructuralPriceLevelsScore *float64 `yaml:"minStructuralPriceLevelsScore"`

	PartialTakeProfitPercent  *float64 `yaml:"partialTakeProfitPercent"`
	PartialTakeProfitFraction *float64 `yaml:"partialTakeProfitFraction"`

	AdaptiveTimeoutMinProfitPercent *float64 `yaml:"adaptiveTimeoutMinProfitPercent"`
	AdaptiveTimeoutExtensionPeriods *int     `yaml:"adaptiveTimeoutExtensionPeriods"`
}

// forSource - Config с применённым оверрайдом для source (если он задан).
// Возвращает КОПИЮ - вызывающий код читает её как обычный Config, не заботясь
// об источнике сигнала за пределами этого одного вызова.
func (c *Config) forSource(source string) Config {
	resolved := *c

	override, ok := c.Sources[source]
	if !ok {
		return resolved
	}

	if override.TakeProfitVol != nil {
		resolved.TakeProfitVol = *override.TakeProfitVol
	}
	if override.StopLossVol != nil {
		resolved.StopLossVol = *override.StopLossVol
	}
	if override.TakeProfitPercent != nil {
		resolved.TakeProfitPercent = *override.TakeProfitPercent
	}
	if override.StopLossPercent != nil {
		resolved.StopLossPercent = *override.StopLossPercent
	}
	if override.MinTakeProfitPercent != nil {
		resolved.MinTakeProfitPercent = *override.MinTakeProfitPercent
	}
	if override.MaxTakeProfitPercent != nil {
		resolved.MaxTakeProfitPercent = *override.MaxTakeProfitPercent
	}
	if override.MinStopLossPercent != nil {
		resolved.MinStopLossPercent = *override.MinStopLossPercent
	}
	if override.MaxStopLossPercent != nil {
		resolved.MaxStopLossPercent = *override.MaxStopLossPercent
	}
	if override.MaxHoldPeriods != nil {
		resolved.MaxHoldPeriods = *override.MaxHoldPeriods
	}
	if override.StrengthScalePerLevel != nil {
		resolved.StrengthScalePerLevel = *override.StrengthScalePerLevel
	}
	if override.TrailingActivationPercent != nil {
		resolved.TrailingActivationPercent = *override.TrailingActivationPercent
	}
	if override.TrailingCallbackPercent != nil {
		resolved.TrailingCallbackPercent = *override.TrailingCallbackPercent
	}
	if override.UseStructuralLevels != nil {
		resolved.UseStructuralLevels = *override.UseStructuralLevels
	}
	if override.MinStructuralScore != nil {
		resolved.MinStructuralScore = *override.MinStructuralScore
	}
	if override.UseStructuralPriceLevels != nil {
		resolved.UseStructuralPriceLevels = *override.UseStructuralPriceLevels
	}
	if override.MinStructuralPriceLevelsScore != nil {
		resolved.MinStructuralPriceLevelsScore = *override.MinStructuralPriceLevelsScore
	}
	if override.PartialTakeProfitPercent != nil {
		resolved.PartialTakeProfitPercent = *override.PartialTakeProfitPercent
	}
	if override.PartialTakeProfitFraction != nil {
		resolved.PartialTakeProfitFraction = *override.PartialTakeProfitFraction
	}
	if override.AdaptiveTimeoutMinProfitPercent != nil {
		resolved.AdaptiveTimeoutMinProfitPercent = *override.AdaptiveTimeoutMinProfitPercent
	}
	if override.AdaptiveTimeoutExtensionPeriods != nil {
		resolved.AdaptiveTimeoutExtensionPeriods = *override.AdaptiveTimeoutExtensionPeriods
	}

	return resolved
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
