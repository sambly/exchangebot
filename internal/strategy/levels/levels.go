// Package levels - ДЕТЕКТОР на основе свечных swing high/low
// (entrysetup.GetPriceLevels/LevelsSideScore, тот же примитив, что даёт
// компонент "Уровни" в композите "Потенциал", см. entrysetup/strength.go).
//
// В отличие от anomaly (смотрит на СЫРОЕ изменение цены/объёма) и potential
// (свод нескольких компонентов), здесь один-единственный сигнал: цена
// подошла к уже состоявшемуся развороту (swing high/low за последние
// levelsLookbackCandles свечей), и дальняя граница диапазона заметно дальше
// ближней (Score = TakeDistance/StopDistance) - т.е. потенциальная награда
// оправдывает риск структурно, а не по статистической оценке волатильности.
//
// signal.Direction здесь означает не "куда пошла цена" (как у anomaly), а
// "какую сторону подсказывает структура" - LevelsSideScore уже сама решает
// BUY/SELL по тому, какая граница ближе (см. Go-комментарий там же).
//
// Нет ни Telegram-меню, ни дайджеста - та же ситуация, что у potential:
// стратегия совсем новая, единственный канал наблюдения - лог
// (Config.LogSignals).
package levels

import (
	"context"
	"fmt"
	"sync"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/entrysetup"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/signal"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	"github.com/sambly/exchangebot/internal/toggle"
)

var levelsLogger = logger.AddFields(map[string]interface{}{
	"package": "levels",
})

// pairPeriodState - cooldown/confirm-состояние на пару+период. Тот же
// принцип, что у potential.pairPeriodState (см. Go-комментарий там же) -
// локальная копия, а не общий пакет: маленький чистый стейт-машин, не
// стоящий отдельной зависимости между детекторами (тот же аргумент, что у
// depth.sideFromZScore/prices.vpinSideFromZScore).
type pairPeriodState struct {
	cooldownUntil time.Time
	lastLevel     int

	pendingChecks int
	pendingLevel  int
	pendingSide   string
}

// confirmStreak - тот же принцип, что у anomaly.confirm()/potential.
// confirmStreak: сторона должна продержаться checks тиков подряд, отдаётся
// ПИКОВЫЙ уровень за это окно.
func confirmStreak(state *pairPeriodState, side string, level, checks int) (confirmedLevel int, ok bool) {
	if checks <= 1 {
		return level, true
	}

	if state.pendingChecks == 0 || state.pendingSide != side {
		state.pendingChecks = 1
		state.pendingLevel = level
		state.pendingSide = side
	} else {
		state.pendingChecks++
		if level > state.pendingLevel {
			state.pendingLevel = level
		}
	}

	if state.pendingChecks < checks {
		return 0, false
	}
	return state.pendingLevel, true
}

// levelFromScore - тот же принцип, что у anomaly.LevelFromZScore/potential.
// levelFromComposite, применённый к Score (соотношение дальнего свечного
// уровня к ближнему) вместо z-score.
func levelFromScore(score float64, t ThresholdsConfig) int {
	switch {
	case score >= t.Level3:
		return 3
	case score >= t.Level2:
		return 2
	case score >= t.Level1:
		return 1
	default:
		return 0
	}
}

type LevelsStrategy struct {
	Config *Config

	StrategyEnable *toggle.Bool

	AssetsPrices *prices.AssetsPrices
	AssetsSetup  *entrysetup.AssetsSetup

	states map[string]map[string]*pairPeriodState

	subscribers []chan signal.Signal

	mu sync.Mutex
}

func NewStrategy(
	assetsPrices *prices.AssetsPrices,
	assetsSetup *entrysetup.AssetsSetup,
	pairs []string,
) (*LevelsStrategy, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	if cfg.AllPairs {
		cfg.Pairs = pairs
	}

	str := &LevelsStrategy{
		Config:         cfg,
		StrategyEnable: toggle.New(cfg.StrategyEnable),
		AssetsPrices:   assetsPrices,
		AssetsSetup:    assetsSetup,
		states:         make(map[string]map[string]*pairPeriodState),
	}

	for _, pair := range cfg.Pairs {
		str.states[pair] = make(map[string]*pairPeriodState, len(cfg.Periods))
		for _, period := range cfg.Periods {
			str.states[pair][period] = &pairPeriodState{}
		}
	}

	return str, nil
}

// Subscribe подписывает исполнителя на сигналы стратегии - см. пакет signal.
func (s *LevelsStrategy) Subscribe(ch chan signal.Signal) {
	s.subscribers = append(s.subscribers, ch)
}

// publish рассылает сигнал неблокирующе - см. тот же комментарий у
// anomaly.publish/potential.publish.
func (s *LevelsStrategy) publish(sig signal.Signal) {
	for _, sub := range s.subscribers {
		select {
		case sub <- sig:
		default:
			levelsLogger.Warnf("подписчик не успевает, сигнал %s %s отброшен", sig.Pair, sig.Period)
		}
	}
}

// GetIDName/GetName - см. strategy.WebToggle.
func (s *LevelsStrategy) GetIDName() string { return s.Config.IDName }
func (s *LevelsStrategy) GetName() string   { return s.Config.Name }

// IsEnabled/SetEnabled - см. strategy.WebToggle.
func (s *LevelsStrategy) IsEnabled() bool   { return s.StrategyEnable.Get() }
func (s *LevelsStrategy) SetEnabled(v bool) { s.StrategyEnable.Set(v) }

// GetTelegramMenu - нет меню, см. Go-комментарий в начале файла.
func (s *LevelsStrategy) GetTelegramMenu() model.WindowHandler { return nil }

// OnMarket - детектору не нужны тики маркетстата напрямую, только пересчёт
// по AssetsPrices.Subscribe() (см. Start) - то же соображение, что у
// base/potential.
func (s *LevelsStrategy) OnMarket(ms exModel.MarketsStat) {}

func (s *LevelsStrategy) Start(ctx context.Context) error {
	updates := s.AssetsPrices.Subscribe()
	for {
		select {
		case <-updates:
			if s.StrategyEnable.Get() {
				s.checkAndPublish()
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// liquidPairs - тот же фильтр, что у anomaly.liquidPairs/potential.
// liquidPairs.
func (s *LevelsStrategy) liquidPairs() []string {
	if s.Config.MinDailyVolume <= 0 {
		return s.Config.Pairs
	}

	pairs := make([]string, 0, len(s.Config.Pairs))
	for _, pair := range s.Config.Pairs {
		stat, err := s.AssetsPrices.GetMarketsStatForPair(pair)
		if err != nil {
			continue
		}
		if stat.Volume >= s.Config.MinDailyVolume {
			pairs = append(pairs, pair)
		}
	}
	return pairs
}

func (s *LevelsStrategy) cooldown() time.Duration {
	return time.Duration(s.Config.CooldownMinutes) * time.Minute
}

// checkAndPublish - основной тик: считает Score по всем отслеживаемым
// парам+периодам (entrysetup.GetAllPriceLevels - один поход в БД на период,
// закэширован на priceLevelsCacheTTL) и публикует сигналы для тех, что
// прошли пороги и confirm-стрик.
func (s *LevelsStrategy) checkAndPublish() {
	pairs := s.liquidPairs()
	now := time.Now()

	allLevels := s.AssetsSetup.GetAllPriceLevels()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, pair := range pairs {
		byPeriod, ok := allLevels[pair]
		if !ok {
			continue
		}
		periodStates, ok := s.states[pair]
		if !ok {
			continue
		}

		for _, period := range s.Config.Periods {
			levelsForPeriod, ok := byPeriod[period]
			if !ok {
				continue
			}
			state := periodStates[period]
			if state == nil {
				continue
			}

			side, score, ok := entrysetup.LevelsSideScore(levelsForPeriod)
			if !ok {
				state.pendingChecks = 0
				if !now.Before(state.cooldownUntil) {
					state.lastLevel = 0
				}
				continue
			}

			level := levelFromScore(score, s.Config.Thresholds)
			if level == 0 {
				state.pendingChecks = 0
				if !now.Before(state.cooldownUntil) {
					state.lastLevel = 0
				}
				continue
			}

			sideStr := string(side)

			confirmedLevel, confirmed := confirmStreak(state, sideStr, level, s.Config.ConfirmChecks)
			if !confirmed {
				continue
			}

			if now.Before(state.cooldownUntil) && confirmedLevel <= state.lastLevel {
				continue
			}

			state.lastLevel = confirmedLevel
			state.cooldownUntil = now.Add(s.cooldown())

			sig := s.signalFrom(pair, period, side, score, confirmedLevel)
			if s.Config.LogSignals {
				levelsLogger.Infof("сигнал %s %s side=%s score=%.2f level=%d",
					pair, period, side, score, confirmedLevel)
			}
			s.publish(sig)
		}
	}
}

func (s *LevelsStrategy) signalFrom(pair, period string, side order.SideType, score float64, level int) signal.Signal {
	direction := signal.DirectionUp
	if side == order.SideTypeSell {
		direction = signal.DirectionDown
	}

	changePercent := 0.0
	if cp, ok := s.AssetsPrices.GetChangePrices(pair, period); ok {
		changePercent = cp.ChangePercent
	}
	// Volatility - фолбэк для structsale.Plan на случай, если для этого
	// источника не включён useStructuralPriceLevels (см.
	// internal/strategy/sales/structsale/config.example.yaml, секция
	// sources) - без него план откатился бы на статичные % из конфига.
	volatility, _ := s.AssetsPrices.GetVolatility(pair, period)

	return signal.Signal{
		Source:        s.Config.IDName,
		Pair:          pair,
		Period:        period,
		Time:          time.Now(),
		Direction:     direction,
		Level:         level,
		Strength:      score,
		ChangePercent: changePercent,
		Volatility:    volatility,
		Reason: fmt.Sprintf("%s %s side=%s score=%.2f level=%d",
			s.Config.IDName, period, side, score, level),
	}
}
