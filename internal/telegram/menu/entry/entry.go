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

func (m *MainMenu) Show(c tele.Context, handler model.MenuHandler) error {
	// Отправляем главное меню
	return c.Send("Главное меню:", m.Markup)
}

// Handle регистрирует обработчики главного меню.
func (m *MainMenu) Handle(b *tele.Bot, handler model.MenuHandler) {
	// Обрабатываем команду /start, чтобы показать главное меню
	b.Handle("/start", func(c tele.Context) error {
		return m.Show(c, handler)
	})

	// Обрабатываем кнопку "Назад" для всех меню
	b.Handle(&global.BtnBack, func(c tele.Context) error {
		userID := c.Sender().ID

		// Получаем функцию возврата в предыдущее меню
		prevMenu := handler.GetPreviousMenu(userID)
		if prevMenu != nil {
			return prevMenu(c, handler) // Переключаем пользователя обратно
		}

		// Если предыдущее меню отсутствует — значит, мы уже в главном
		return c.Send("Вы уже в главном меню.")
	})
}
