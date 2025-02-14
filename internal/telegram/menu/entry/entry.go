package entry

import (
	"github.com/sambly/exchangebot/internal/telegram/menu/base"
	"github.com/sambly/exchangebot/internal/telegram/menu/global"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	tele "gopkg.in/telebot.v3"
)

// Cтруктура главного меню
type MainMenu struct {
	*base.BaseMenu
}

// NewMainMenu создаёт главное меню.
func NewMainMenu(name, id string) *MainMenu {
	menu := &MainMenu{
		BaseMenu: base.NewBaseMenu(name, id),
	}
	return menu
}

func (m *MainMenu) ShowMainMenu(c tele.Context, handler model.MenuHandler) error {
	userID := c.Sender().ID

	// Сохраняем текущее меню и состояние пользователя
	handler.SetUserState(userID, m.ID)
	handler.SetUserMenu(userID, m.ID, m.Markup)

	// Отправляем главное меню
	return c.Send("Главное меню:", m.Markup)
}

func (m *MainMenu) Handle(b *tele.Bot, handler model.MenuHandler) {
	// Обрабатываем команду /start, чтобы показать главное меню
	b.Handle("/start", func(c tele.Context) error {
		return m.ShowMainMenu(c, handler)
	})

	b.Handle(&global.BtnBack, func(c tele.Context) error {
		userID := c.Sender().ID

		// Получаем предыдущее меню и клавиатуру
		previousMenu := handler.GetPreviousMenu(userID)
		previousMarkup := handler.GetPreviousMarkup(userID)

		// Устанавливаем состояние пользователя на предыдущее
		handler.SetUserState(userID, previousMenu)

		// Удаляем сообщение пользователя, если оно есть
		if c.Message() != nil {
			_ = c.Delete()
		}

		// Отправляем предыдущее меню с соответствующей клавиатурой
		return c.Send("Возврат в предыдущее меню:", previousMarkup)
	})

}
