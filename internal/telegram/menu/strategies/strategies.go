package strategies

import (
	"github.com/sambly/exchangebot/internal/strategy"
	"github.com/sambly/exchangebot/internal/telegram/menu/base"
	"github.com/sambly/exchangebot/internal/telegram/menu/global"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	tele "gopkg.in/telebot.v3"
)

var (
	btnLabelStrategies = "📊 Стратегии"

	// Кнопка точка входа
	entryButton = global.Markup.Text(btnLabelStrategies)

	// Базовые кнопки в меню
	defaultButtons = []tele.Btn{
		global.BtnBack,
		global.BtnMainMenu,
	}
)

type StrategyMenu struct {
	*base.BaseMenu
	StrategyController *strategy.ControllerStrategy
}

func NewStrategyMenu(name, id string, strategyCtrl *strategy.ControllerStrategy) *StrategyMenu {
	menu := &StrategyMenu{
		BaseMenu:           base.NewBaseMenu(name, id),
		StrategyController: strategyCtrl,
	}

	menu.BaseMenu.WithEntryButton(entryButton)
	menu.BaseMenu.AddButtons(defaultButtons...)

	return menu
}

func (m *StrategyMenu) Show(c tele.Context, handler model.MenuHandler) error {
	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show)
	return c.Send("Меню стратегий:", m.Markup)
}

// Handle обрабатывает кнопки меню стратегий
func (m *StrategyMenu) Handle(b *tele.Bot, handler model.MenuHandler) {
	// Обработчик кнопки входа в меню стратегий
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})
}
