package anomaly

import (
	"fmt"
	"sort"

	"github.com/sambly/exchangebot/internal/telegram/menu/base"
	"github.com/sambly/exchangebot/internal/telegram/menu/global"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	tele "gopkg.in/telebot.v3"
)

var (
	// Кнопка точка входа
	entryButton = tele.Btn{Text: "🚨 Аномалии"}
	// Базовые кнопки в меню
	replyButtons = [][]tele.Btn{
		{global.BtnBack, global.BtnMainMenu},
	}

	// Inline кнопки
	btnEnableStrategy  = tele.Btn{Text: "✅ Включить детектор", Unique: "enable_anomaly"}
	btnDisableStrategy = tele.Btn{Text: "❌ Отключить детектор", Unique: "disable_anomaly"}

	btnEnableNotifications  = tele.Btn{Text: "🔔 Включить уведомления", Unique: "enable_notif_anomaly"}
	btnDisableNotifications = tele.Btn{Text: "🔕 Отключить уведомления", Unique: "disable_notif_anomaly"}

	inlineButtons = [][]tele.Btn{
		{btnEnableStrategy, btnDisableStrategy},
		{btnEnableNotifications, btnDisableNotifications},
	}
)

type AnomalyMenu struct {
	*base.BaseMenu
	Strategy *AnomalyStrategy
}

func NewMenu(name, id string, str *AnomalyStrategy) *AnomalyMenu {
	menu := &AnomalyMenu{
		BaseMenu: base.NewBaseMenu(name, id),
		Strategy: str,
	}

	menu.AddButtonRows(replyButtons...)
	menu.WithEntryButton(entryButton)
	menu.AddButtonRowsInline(inlineButtons...)

	return menu
}

// activePeriods возвращает включённые периоды в детерминированном порядке.
func activePeriods(periods map[string]*PeriodConfig) []string {
	names := make([]string, 0, len(periods))
	for period, pc := range periods {
		if pc != nil && pc.Enabled {
			names = append(names, period)
		}
	}
	sort.Strings(names)
	return names
}

func (m *AnomalyMenu) Show(c tele.Context, handler model.MenuHandler) error {

	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show, nil)
	handler.DeleteUserMessages(c, userID)

	cfg := m.Strategy.Config

	text := fmt.Sprintf("🚨 %s\n", cfg.Name)
	if cfg.Description != "" {
		text += cfg.Description + "\n"
	}
	text += "\n"

	if m.Strategy.StrategyEnable.Get() {
		text += "Детектор: ✅ включён\n"
	} else {
		text += "Детектор: ❌ отключён\n"
	}
	if m.Strategy.NotificationEnable.Get() {
		text += "Уведомления: 🔔 включены\n"
	} else {
		text += "Уведомления: 🔕 отключены\n"
	}

	text += fmt.Sprintf("\nПар отслеживается: %d\n", len(cfg.Pairs))
	text += fmt.Sprintf("Мин. суточный оборот: %.0f USDT\n", cfg.MinDailyVolume)
	text += fmt.Sprintf("Мин. уровень для уведомления: %d\n", cfg.MinNotifyLevel)

	periods := activePeriods(cfg.Periods)
	text += "\nАктивные периоды:\n"
	if len(periods) == 0 {
		text += "  нет включённых периодов\n"
	}
	for _, period := range periods {
		pc := cfg.Periods[period]
		text += fmt.Sprintf("  %s: z ≥ %.1f/%.1f/%.1f, подтверждение %d мин.\n",
			period, pc.Thresholds.Level1, pc.Thresholds.Level2, pc.Thresholds.Level3, pc.ConfirmChecks)
	}

	if cfg.MarketMetric.Enabled {
		text += fmt.Sprintf("\nРыночная метрика: включена (порог %.0f%% пар, мин. пар %d)\n",
			cfg.MarketMetric.AnomalyPercentThreshold, cfg.MarketMetric.MinPairs)
	}

	if err := c.Send(text, m.Markup); err != nil {
		return err
	}

	// Кнопки inline отправляем отдельно
	if len(m.InlineButtons) > 0 {
		if err := c.Send("Выберите действие:", m.InlineMarkup); err != nil {
			return err
		}
	}
	return nil
}

// Handle обрабатывает кнопки меню стратегии
func (m *AnomalyMenu) Handle(b *tele.Bot, handler model.MenuHandler) {
	// Обработчик кнопки входа в меню стратегии
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})

	b.Handle(&btnEnableStrategy, func(c tele.Context) error {
		m.Strategy.StrategyEnable.Set(true)
		return c.Respond(&tele.CallbackResponse{Text: "Детектор включён ✅", ShowAlert: true})
	})

	b.Handle(&btnDisableStrategy, func(c tele.Context) error {
		m.Strategy.StrategyEnable.Set(false)
		return c.Respond(&tele.CallbackResponse{Text: "Детектор отключён ❌", ShowAlert: true})
	})

	b.Handle(&btnEnableNotifications, func(c tele.Context) error {
		m.Strategy.NotificationEnable.Set(true)
		return c.Respond(&tele.CallbackResponse{Text: "Уведомления включены ✅", ShowAlert: true})
	})

	b.Handle(&btnDisableNotifications, func(c tele.Context) error {
		m.Strategy.NotificationEnable.Set(false)
		return c.Respond(&tele.CallbackResponse{Text: "Уведомления отключены ❌", ShowAlert: true})
	})
}
