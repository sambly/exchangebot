package strategies

import (
	"github.com/sambly/exchangebot/internal/strategy"
	"github.com/sambly/exchangebot/internal/telegram/menu/base"
	"github.com/sambly/exchangebot/internal/telegram/menu/global"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	tele "gopkg.in/telebot.v3"
)

var (

	// Кнопка точка входа
	entryButton = tele.Btn{Text: "📊 Стратегии"}

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

	menu.WithEntryButton(entryButton)
	menu.AddButtonRows(defaultButtons...)

	// Добавление точек входа в подменю стратегий
	for _, strategy := range strategyCtrl.Strategies {
		if strMenu := strategy.GetTelegramMenu(); strMenu != nil {
			menu.AddButtons(true, strMenu.GetEntryButton())
			menu.AddSubMenu(strMenu)
		}
	}

	return menu
}

// Показать главное меню
func (m *StrategyMenu) Show(c tele.Context, handler model.MenuHandler) error {
	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show, nil)
	handler.DeleteUserMessages(c, userID)
	return c.Send("Меню стратегий:", m.Markup)
}

// Handle обрабатывает кнопки меню стратегий
func (m *StrategyMenu) Handle(b *tele.Bot, handler model.MenuHandler) {

	// Подключаем обработчики подменю
	for _, subMenu := range m.SubMenus {
		subMenu.Handle(b, handler)
	}

	// Обработчик кнопки входа в меню стратегий
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})
}
