// Package potential - ДЕТЕКТОР на основе композитного индикатора "Потенциал"
// (см. entrysetup.GetAllStrengthComponents, ARCHITECTURE.md §10.2/10.7/10.10):
// OFI + VPIN + Уровни + Относительная сила, со сжатием волатильности как
// множителем. До этой стратегии композит был только скринером (таблица во
// фронте, DataStrength.vue) - здесь та же логика свода портирована в Go и
// подключена к общему конвейеру детектор -> исполнитель (см. пакет signal и
// Go-комментарий в internal/strategy/strategy.go про три роли).
//
// В отличие от anomaly (которая смотрит на СЫРОЕ изменение цены/объёма и
// определяет НАПРАВЛЕНИЕ ДВИЖЕНИЯ), композит "Потенциала" уже сам по себе
// перевес в конкретную сторону (за счёт направленных компонентов - OFI,
// уровней, относительной силы). Поэтому signal.Direction здесь означает не
// "куда пошла цена", а "какую сторону рекомендует композит" - Direction=Up
// это BUY, Direction=Down это SELL, независимо от знака ChangePercent.
//
// Нет ни Telegram-меню, ни дайджеста уведомлений (в отличие от anomaly/base) -
// стратегия совсем новая, непроверенная на реальной торговле, единственный
// канал наблюдения сейчас - лог (см. Config.LogSignals). Появится, если
// стратегия покажет себя и потребуется ручное подтверждение сделок из
// Telegram.
package potential

import (
	"context"
	"fmt"
	"sync"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/entrysetup"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/signal"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	"github.com/sambly/exchangebot/internal/toggle"
)

var potentialLogger = logger.AddFields(map[string]interface{}{
	"package": "potential",
})

// pairPeriodState - cooldown/confirm-состояние на пару+период. Тот же принцип,
// что у anomaly.PeriodState, но проще: нет истории метрик (её уже считают и
// z-скорят depth/prices/entrysetup), нужно только "держится ли сторона
// композита N тиков подряд" и "когда можно публиковать по этой паре+периоду
// снова".
type pairPeriodState struct {
	cooldownUntil time.Time
	lastLevel     int

	pendingChecks int
	pendingLevel  int
	pendingSide   string
}

// confirmStreak - тот же принцип, что у anomaly.confirm(): сторона должна
// продержаться checks тиков подряд, отдаётся ПИКОВЫЙ уровень за это окно (не
// усреднённый - усреднение срезало бы пик и не пропустило бы через minLevel
// исполнителя лучшие сигналы вместе с шумом).
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

type PotentialStrategy struct {
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
) (*PotentialStrategy, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	if cfg.AllPairs {
		cfg.Pairs = pairs
	}

	str := &PotentialStrategy{
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
func (s *PotentialStrategy) Subscribe(ch chan signal.Signal) {
	s.subscribers = append(s.subscribers, ch)
}

// publish рассылает сигнал неблокирующе - см. тот же комментарий у
// anomaly.publish/base.publish.
func (s *PotentialStrategy) publish(sig signal.Signal) {
	for _, sub := range s.subscribers {
		select {
		case sub <- sig:
		default:
			potentialLogger.Warnf("подписчик не успевает, сигнал %s %s отброшен", sig.Pair, sig.Period)
		}
	}
}

// GetIDName/GetName - см. strategy.WebToggle.
func (s *PotentialStrategy) GetIDName() string { return s.Config.IDName }
func (s *PotentialStrategy) GetName() string   { return s.Config.Name }

// IsEnabled/SetEnabled - см. strategy.WebToggle.
func (s *PotentialStrategy) IsEnabled() bool   { return s.StrategyEnable.Get() }
func (s *PotentialStrategy) SetEnabled(v bool) { s.StrategyEnable.Set(v) }

// GetTelegramMenu - нет меню, см. Go-комментарий в начале файла.
func (s *PotentialStrategy) GetTelegramMenu() model.WindowHandler { return nil }

// OnMarket - потенциалу не нужны тики маркетстата напрямую, только пересчёт
// по AssetsPrices.Subscribe() (см. Start) - то же соображение, что у base.
func (s *PotentialStrategy) OnMarket(ms exModel.MarketsStat) {}

func (s *PotentialStrategy) Start(ctx context.Context) error {
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

// liquidPairs - тот же фильтр, что у anomaly.liquidPairs.
func (s *PotentialStrategy) liquidPairs() []string {
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

func (s *PotentialStrategy) cooldown() time.Duration {
	return time.Duration(s.Config.CooldownMinutes) * time.Minute
}

// checkAndPublish - основной тик: считает композит по всем отслеживаемым
// парам+периодам и публикует сигналы для тех, что прошли пороги, agree/total
// и confirm-стрик. Тот же принцип, что у anomaly.checkAndNotify, но без
// истории метрик - composite уже готовый z-подобный сигнал от entrysetup.
func (s *PotentialStrategy) checkAndPublish() {
	pairs := s.liquidPairs()
	now := time.Now()

	components := s.AssetsSetup.GetAllStrengthComponents()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, pair := range pairs {
		byPeriod, ok := components[pair]
		if !ok {
			continue
		}
		periodStates, ok := s.states[pair]
		if !ok {
			continue
		}

		for _, period := range s.Config.Periods {
			comps, ok := byPeriod[period]
			if !ok {
				continue
			}
			state := periodStates[period]
			if state == nil {
				continue
			}

			composite, agree, total := compositeFromComponents(comps, s.Config.Components)

			if total < s.Config.MinTotal || agree < s.Config.MinAgree || composite == 0 {
				state.pendingChecks = 0
				if !now.Before(state.cooldownUntil) {
					state.lastLevel = 0
				}
				continue
			}

			level := levelFromComposite(composite, s.Config.Thresholds)
			if level == 0 {
				state.pendingChecks = 0
				if !now.Before(state.cooldownUntil) {
					state.lastLevel = 0
				}
				continue
			}

			side := "BUY"
			if composite < 0 {
				side = "SELL"
			}

			confirmedLevel, confirmed := confirmStreak(state, side, level, s.Config.ConfirmChecks)
			if !confirmed {
				continue
			}

			if now.Before(state.cooldownUntil) && confirmedLevel <= state.lastLevel {
				continue
			}

			state.lastLevel = confirmedLevel
			state.cooldownUntil = now.Add(s.cooldown())

			sig := s.signalFrom(pair, period, side, composite, confirmedLevel, agree, total)
			if s.Config.LogSignals {
				potentialLogger.Infof("сигнал %s %s side=%s composite=%.2f agree=%d/%d level=%d",
					pair, period, side, composite, agree, total, confirmedLevel)
			}
			s.publish(sig)
		}
	}
}

func (s *PotentialStrategy) signalFrom(pair, period, side string, composite float64, level, agree, total int) signal.Signal {
	direction := signal.DirectionUp
	if side == "SELL" {
		direction = signal.DirectionDown
	}

	changePercent := 0.0
	if cp, ok := s.AssetsPrices.GetChangePrices(pair, period); ok {
		changePercent = cp.ChangePercent
	}
	// Volatility - см. Go-комментарий у signal.Signal.Volatility: та же
	// величина, на которую опирается GetVolatilityRegime (робастный разброс
	// цены пары за период), нужна политике выхода для расчёта тейка/стопа
	// без неё она откатывается на статичные % из своего конфига.
	volatility, _ := s.AssetsPrices.GetVolatility(pair, period)

	return signal.Signal{
		Source:        s.Config.IDName,
		Pair:          pair,
		Period:        period,
		Time:          time.Now(),
		Direction:     direction,
		Level:         level,
		Strength:      composite,
		ChangePercent: changePercent,
		Volatility:    volatility,
		Reason: fmt.Sprintf("%s %s composite=%.2f agree=%d/%d level=%d",
			s.Config.IDName, period, composite, agree, total, level),
	}
}
