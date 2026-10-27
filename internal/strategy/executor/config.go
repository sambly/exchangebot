package executor

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sambly/exchangebot/internal/order"
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

	// OnUp / OnDown - ЧТО ДЕЛАТЬ с сигналом каждого направления: buy, sell, skip.
	// Глобальный дефолт; конкретный период может его переопределить через Periods.
	//
	// Раньше здесь был один флаг direction, и он решал только "торговать ли",
	// а сторона сделки всегда была BUY. Получалось, что на аномальное ПАДЕНИЕ
	// на 10% приходило предложение купить - то есть поймать падающий нож.
	//
	// Теперь направление сигнала и сторона сделки - разные вещи:
	//   onUp: buy    - цена аномально выросла, играем на продолжение (лонг)
	//   onUp: sell   - выросла, играем на откат (шорт против импульса)
	//   onDown: sell - упала, играем на продолжение падения (шорт)
	//   onDown: buy  - упала, играем на отскок (лонг; нож, стоп обязателен)
	//   skip         - сигналы этого направления не торгуем
	OnUp   string `yaml:"onUp"`
	OnDown string `yaml:"onDown"`

	// Periods - переопределение OnUp/OnDown для конкретного сигнального периода.
	// Пусто в самом переопределении - берётся глобальное значение выше.
	//
	// Бэктест anomaly (см. cmd backtest) показал, что momentum и mean-reversion
	// работают на разных периодах по-разному: на 15m/1h цена после аномалии в
	// среднем откатывается (fade выгоднее), на 4h слабо, но продолжает (momentum
	// выгоднее). Один OnUp/OnDown на все периоды сразу игнорирует эту разницу.
	//
	// ВАЖНО: это переопределение относится ТОЛЬКО к сигналам, у которых
	// Direction означает "куда пошла цена" (anomaly, base) - у них momentum и
	// fade осмысленно разные стратегии. У потенциал/levels-детекторов
	// Direction уже сама по себе рекомендация стороны (композит/структура УЖЕ
	// решили BUY или SELL), а не сырое движение - "fade" для них означало бы
	// буквально торговать НАОБОРОТ собственной рекомендации. Такие источники
	// должны быть в SourceDirections ниже, который проверяется РАНЬШЕ и
	// полностью обходит эту таблицу.
	Periods map[string]PeriodDirection `yaml:"periods"`

	// SourceDirections - оверрайд OnUp/OnDown ПО ИСТОЧНИКУ СИГНАЛА (sig.
	// Source), проверяется в actionFor ПЕРВЫМ и, если для источника есть
	// запись, полностью ЗАМЕНЯЕТ и глобальный OnUp/OnDown, и таблицу Periods -
	// период-зависимая fade/momentum логика Periods предназначена для anomaly
	// (см. комментарий там же) и не должна молча применяться к источникам, где
	// Direction - уже готовая рекомендация стороны (potential, levels).
	SourceDirections map[string]PeriodDirection `yaml:"sourceDirections"`

	// SkipDivergent - не входить в сигналы с Divergent=true: движение цены
	// прошло при активности НИЖЕ обычной ("пустой стакан"), такие движения
	// чаще откатываются, чем продолжаются (см. AnomalyResult.Divergent).
	SkipDivergent bool `yaml:"skipDivergent"`

	// MinLevel - минимальная сила сигнала для входа (у anomaly это уровень 1..3)
	MinLevel int `yaml:"minLevel"`

	// Sources - от каких детекторов принимаем сигналы. Пусто - от всех.
	Sources []string `yaml:"sources"`

	// Size - размер сделки
	Size float64 `yaml:"size"`

	// MaxPositions - сколько позиций держим одновременно НА КАЖДЫЙ источник
	// сигнала (sig.Source) НЕЗАВИСИМО, а не общий потолок на всех сразу: у
	// anomaly свой бюджет в MaxPositions позиций, у potential (или любого
	// будущего детектора) - свой такой же, один источник не может выесть
	// лимит у другого. На рыночном движении аномальными становятся десятки
	// пар разом - без лимита это было бы не диверсификацией, а плечом.
	MaxPositions int `yaml:"maxPositions"`

	// PairCooldownMinutes - сколько ждать после закрытия позиции по паре,
	// прежде чем открывать по ней новую. Защита от "лестницы" по одной паре.
	PairCooldownMinutes int `yaml:"pairCooldownMinutes"`

	// MaxVolatilityRegime - верхняя граница entrysetup.GetVolatilityRegime
	// (ОТНОШЕНИЕ недавней волатильности пары к типичной, не путать с Volatility
	// в signal.Signal - той, что нужна политике выхода) для входа. >1 - пара
	// уже разогналась сильнее обычного ПРЯМО СЕЙЧАС. 0 - фильтр выключен.
	//
	// Проверяется в Executor.marketFiltersAllow (нужен AssetsSetup, поэтому не
	// здесь, в SideFor - тот чистая функция сигнала и конфига). Если режим не
	// посчитан (мало истории) - НЕ блокирует, фильтр только отсекает
	// подтверждённо разогнавшиеся пары, а не "неизвестные".
	//
	// См. ARCHITECTURE.md §10.15: одна такая пара (HEIUSDT) дала anomaly
	// больше всего убытка за 20ч форвард-теста - серия сигналов на
	// экстремально тонком/волатильном инструменте.
	MaxVolatilityRegime float64 `yaml:"maxVolatilityRegime"`

	// RequireLevelsAgree - перед входом сверять сторону сделки со свечными
	// swing-уровнями (entrysetup.GetPriceLevels + LevelsSideScore): ближе
	// поддержка - должны покупать, ближе сопротивление - продавать. Если
	// уровней нет или они не дают чёткой стороны - НЕ блокирует (нечем
	// подтвердить не значит "противоречит"), блокирует только явное
	// РАСХОЖДЕНИЕ с резолвнутой стороной сделки. Действует на ВСЕ источники
	// сразу (глобальный фильтр, не per-source).
	RequireLevelsAgree bool `yaml:"requireLevelsAgree"`

	// MaxTrendStrengthBySource - верхняя граница entrysetup.GetTrendStrength
	// (0..1, efficiency ratio: 1 - цена прошла путь по прямой, сильный тренд;
	// 0 - зигзаг без чистого сдвига, боковик) ПО ИСТОЧНИКУ СИГНАЛА. Источник
	// БЕЗ записи здесь - фильтр для него не применяется вовсе. Это ДРУГАЯ ось,
	// чем MaxVolatilityRegime (та - амплитуда движения относительно своей же
	// нормы, эта - направленность движения) - map, а не одно число: в отличие
	// от MaxVolatilityRegime, здесь смысл в том, что источники ведут себя
	// принципиально по-разному. anomaly на 4h настроена на momentum (см.
	// periods в config.yaml) - сильный тренд ей скорее помогает, добавлять её
	// сюда бездумно нельзя. potential/levels - mean-reversion по своей природе
	// (торгуют от уровня/разворота композита), см. ARCHITECTURE.md §10.16:
	// реальный бэктест на BTC показал profit factor 1.62 в боковике против
	// -0.74 в тренде на ОДНОЙ И ТОЙ ЖЕ логике входа, разница только в фильтре
	// режима.
	//
	// Как и остальные MarketContext-фильтры - если сила тренда не посчитана
	// (мало истории), НЕ блокирует.
	MaxTrendStrengthBySource map[string]float64 `yaml:"maxTrendStrengthBySource"`
}

// PeriodDirection - переопределение OnUp/OnDown для одного периода.
// Пустая строка в поле означает "не переопределять", берётся глобальное значение.
type PeriodDirection struct {
	OnUp   string `yaml:"onUp"`
	OnDown string `yaml:"onDown"`
}

// Reject - почему сигнал не стал сделкой.
type Reject struct {
	Reason string
	// Code - стабильный ключ причины, без подставленных в Reason чисел.
	// По нему бэктест агрегирует отказы в отчёте.
	Code string
}

func (r Reject) String() string { return r.Reason }

func routine(code, format string, args ...interface{}) Reject {
	return Reject{Reason: fmt.Sprintf(format, args...), Code: code}
}

// SideFor решает, ЧТО делать с сигналом: купить, продать или пропустить.
//
// Возвращает сторону сделки, причину отказа и признак "торгуем".
// Именно здесь направление сигнала (цена выросла/упала) превращается в сторону
// сделки - раньше сторона была жёстко BUY, и на аномальное падение приходило
// предложение купить.
func (c *Config) SideFor(sig signal.Signal) (order.SideType, Reject, bool) {
	if sig.Level < c.MinLevel {
		return "", routine("min-level", "уровень %d < minLevel %d", sig.Level, c.MinLevel), false
	}

	if len(c.Sources) > 0 && !containsFold(c.Sources, sig.Source) {
		return "", routine("source-filter", "источник %s не в списке sources", sig.Source), false
	}

	if c.SkipDivergent && sig.Divergent {
		return "", routine("divergent", "движение по пустому стакану (activity ниже обычной) - пропуск"), false
	}

	action := c.actionFor(sig)

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "buy":
		return order.SideTypeBuy, Reject{}, true
	case "sell":
		return order.SideTypeSell, Reject{}, true
	default: // skip и всё непонятное
		return "", routine("direction-skip", "направление %s не торгуем (действие %q)", sig.Direction, action), false
	}
}

// actionFor - buy/sell/skip для направления сигнала с учётом переопределения
// по периоду (Periods): период важнее глобального OnUp/OnDown, если задан.
func (c *Config) actionFor(sig signal.Signal) string {
	onUp, onDown := c.OnUp, c.OnDown

	// SourceDirections - см. Go-комментарий у поля: для источников с записью
	// здесь Periods (fade/momentum по периоду сигнала) не применяется вовсе -
	// эта логика имеет смысл только для сигналов типа "куда пошла цена"
	// (anomaly), а не для готовых рекомендаций стороны (potential/levels).
	if sd, ok := c.SourceDirections[sig.Source]; ok {
		if sd.OnUp != "" {
			onUp = sd.OnUp
		}
		if sd.OnDown != "" {
			onDown = sd.OnDown
		}
		if sig.Direction == signal.DirectionDown {
			return onDown
		}
		return onUp
	}

	if pd, ok := c.Periods[sig.Period]; ok {
		if pd.OnUp != "" {
			onUp = pd.OnUp
		}
		if pd.OnDown != "" {
			onDown = pd.OnDown
		}
	}

	if sig.Direction == signal.DirectionDown {
		return onDown
	}
	return onUp
}

// containsFold - есть ли значение в списке, без учёта регистра
func containsFold(list []string, value string) bool {
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return true
		}
	}
	return false
}

func NewConfig() (*Config, error) {
	var config Config

	primaryPath := "internal/strategy/executor/config.yaml"
	fallbackPath := "internal/strategy/executor/config.example.yaml"

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
	// только сильные сигналы, только лонг на росте, падения не трогаем.
	if config.OnUp == "" {
		config.OnUp = "buy"
	}
	if config.OnDown == "" {
		config.OnDown = "skip"
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
