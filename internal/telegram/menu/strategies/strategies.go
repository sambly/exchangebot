package strategies

import (
	"github.com/sambly/exchangebot/internal/telegram/menu/manager"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	tele "gopkg.in/telebot.v3"
)

const (
	menuName = "Стратегии:"
	menuID   = "strategies"

	buttonName = "📌 Стратегии"
	buttonID   = "btnStrategies"
)

type Buttons struct {
	Markup *tele.ReplyMarkup
	Back   tele.Btn
}

type StrategyMenu struct {
	Name        string
	ID          string
	EntryButton *model.EntryButton // Кнопка для входа в меню

	Buttons Buttons
}

// NewStrategyMenu создаёт меню стратегий.
func NewStrategyMenu() *StrategyMenu {
	m := &StrategyMenu{
		Name:        menuName,
		ID:          menuID,
		EntryButton: model.NewEntryButton(buttonName, buttonID),
	}

	m.initButtons()
	m.setupMarkup()

	return m
}

func (m *StrategyMenu) initButtons() {
	markup := &tele.ReplyMarkup{}
	m.Buttons.Markup = markup

	m.Buttons.Back = markup.Text("🔙 Назад")
}

func (m *StrategyMenu) setupMarkup() {
	markup := m.Buttons.Markup

	markup.Reply(
		markup.Row(m.Buttons.Back),
	)
}

// Handle обрабатывает кнопки меню стратегий.
func (m *StrategyMenu) Handle(b *tele.Bot, manager *manager.MenuManager) {
	b.Handle(&m.Buttons.Back, func(c tele.Context) error {
		manager.UserState[c.Sender().ID] = "main"

		// Удаляем старое сообщение
		if c.Message() != nil {
			_ = c.Delete()
		}

		// Отправляем новое сообщение с главным меню
		return c.Send("Главное меню:", manager.Main.Markup)
	})
}
