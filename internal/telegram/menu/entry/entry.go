package entry

import (
	"github.com/sambly/exchangebot/internal/telegram/menu/manager"
	tele "gopkg.in/telebot.v3"
)

// Cтруктура главного меню
type MainMenu struct {
	Name   string
	ID     string
	Markup *tele.ReplyMarkup
	Buttons
}

// NewMainMenu создаёт главное меню.
func NewMainMenu() *MainMenu {
	m := &MainMenu{
		Name:   "Главное меню",
		ID:     "main",
		Markup: &tele.ReplyMarkup{},
	}
	return m
}

// Handle обрабатывает кнопки главного меню.
func (m *MainMenu) Handle(b *tele.Bot, manager *manager.MenuManager) {
	b.Handle(&m.Account, func(c tele.Context) error {
		manager.UserState[c.Sender().ID] = "account"

		if c.Message() != nil {
			_ = c.Delete()
		}

		return c.Send("Меню аккаунта:", manager.Account.Markup)
	})

	b.Handle(&m.Strategy, func(c tele.Context) error {
		manager.UserState[c.Sender().ID] = "strategy"

		if c.Message() != nil {
			_ = c.Delete()
		}

		return c.Send("Меню стратегий:", manager.Strategy.Markup)
	})
}
