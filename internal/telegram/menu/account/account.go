package account

import (
	"github.com/sambly/exchangebot/internal/telegram/menu/base"
	"github.com/sambly/exchangebot/internal/telegram/menu/global"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"

	tele "gopkg.in/telebot.v3"
)

var (
	btnLabelAccount = "📌 Аккаунт"

	// Кнопка точка входа
	entryButton = global.Markup.Text(btnLabelAccount)

	// Базовые кнопки в меню
	defaultButtons = []tele.Btn{
		global.BtnBack,
		global.BtnMainMenu,
	}
)

// Структура меню аккаунта
type AccountMenu struct {
	*base.BaseMenu
}

func NewAccountMenu(name, id string) *AccountMenu {
	menu := &AccountMenu{
		BaseMenu: base.NewBaseMenu(name, id),
	}

	menu.BaseMenu.WithEntryButton(entryButton)
	menu.BaseMenu.AddButtons(defaultButtons...)

	return menu
}

func (m *AccountMenu) Show(c tele.Context, handler model.MenuHandler) error {
	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show)
	return c.Send("Меню аккаунта:", m.Markup)
}

// Handle обрабатывает кнопки меню аккаунта
func (m *AccountMenu) Handle(b *tele.Bot, handler model.MenuHandler) {
	// Обработчик кнопки входа в аккаунт
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})
}
