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
	defaultButtons = [][]tele.Btn{
		{global.BtnBack, global.BtnMainMenu},
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

	// Добавление точек входа в подменю стратегий
	for _, strategy := range strategyCtrl.Strategies {
		if strMenu := strategy.GetTelegramMenu(); strMenu != nil {
			menu.AddButton(strMenu.GetEntryButton(), true)
		}
	}

	return menu
}

// Показать главное меню
func (m *StrategyMenu) Show(c tele.Context, handler model.MenuHandler) error {
	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show)
	handler.DeleteUserMessages(c, userID)

	msg, err := c.Bot().Send(c.Chat(), "Меню стратегий:", m.Markup)
	if err == nil {
		handler.SaveMessage(userID, msg)
	}
	return err
}

// Handle обрабатывает кнопки меню стратегий
func (m *StrategyMenu) Handle(b *tele.Bot, handler model.MenuHandler) {

	// обработка меню подстратегий
	for _, strategy := range m.StrategyController.Strategies {
		strategy.GetTelegramMenu().Handle(b, handler)
	}

	// Обработчик кнопки входа в меню стратегий
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})
}
