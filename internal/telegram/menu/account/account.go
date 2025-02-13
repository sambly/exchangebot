package account

import (
	"github.com/sambly/exchangebot/internal/telegram/menu/manager"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"

	tele "gopkg.in/telebot.v3"
)

const (
	menuName = "Аккаунт:"
	menuID   = "account"

	buttonName = "📌 Аккаунт"
	buttonID   = "btnAccount"
)

type Buttons struct {
	Markup *tele.ReplyMarkup
	Back   tele.Btn
}

// Структура меню аккаунта
type AccountMenu struct {
	Name        string
	ID          string
	EntryButton *model.EntryButton // Кнопка для входа в меню

	Buttons Buttons
}

func NewAccountMenu() *AccountMenu {

	m := &AccountMenu{
		Name:        menuName,
		ID:          menuID,
		EntryButton: model.NewEntryButton(buttonName, buttonID),
	}

	m.initButtons()
	m.setupMarkup()
	return m
}

func (m *AccountMenu) initButtons() {
	markup := &tele.ReplyMarkup{}
	m.Buttons.Markup = markup

	m.Buttons.Back = markup.Text("🔙 Назад")
}

func (m *AccountMenu) setupMarkup() {
	markup := m.Buttons.Markup

	markup.Reply(
		markup.Row(m.Buttons.Back),
	)
}

// Handle обрабатывает кнопки меню аккаунта.
func (m *AccountMenu) Handle(b *tele.Bot, manager *manager.MenuManager) {
	b.Handle(&m.Buttons.Back, func(c tele.Context) error {
		manager.UserState[c.Sender().ID] = "main"

		if c.Message() != nil {
			_ = c.Delete()
		}

		return c.Send("Главное меню:", manager.Main.Markup)
	})
}
