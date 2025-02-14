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

// Handle обрабатывает кнопки меню аккаунта
func (m *AccountMenu) Handle(b *tele.Bot, handler model.MenuHandler) {
	// Обработчик кнопки входа (Account)
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		userID := c.Sender().ID

		// Сохраняем текущее меню и клавиатуру перед переходом
		handler.SetUserMenu(userID, m.ID, m.Markup)

		// Устанавливаем состояние пользователя
		handler.SetUserState(userID, m.ID)

		// Удаляем сообщение пользователя, если оно есть
		if c.Message() != nil {
			_ = c.Delete()
		}

		// Отправляем сообщение с меню аккаунта
		return c.Send("Меню аккаунта:", m.Markup)
	})

	// // Обработчик кнопки "Назад"
	// for _, btn := range m.ButtonsHandler.Buttons {
	// 	btn := btn /
	// 	// fmt.Printf("ID: %s, Text: %s\n", btn.ID, btn.TgBtn.Text)
	// 	if btn.ID == "" {
	// 		b.Handle(&btn.TgBtn, func(c tele.Context) error {
	// 			userID := c.Sender().ID

	// 			fmt.Println("Кнопка назад нажата")
	// 			// Получаем предыдущее меню и клавиатуру
	// 			previousMenu := handler.GetPreviousMenu(userID)
	// 			previousMarkup := handler.GetPreviousMarkup(userID)

	// 			// Устанавливаем состояние пользователя на предыдущее
	// 			handler.SetUserState(userID, previousMenu)

	// 			// Удаляем сообщение пользователя, если оно есть
	// 			if c.Message() != nil {
	// 				_ = c.Delete()
	// 			}

	// 			// Отправляем предыдущее меню с соответствующей клавиатурой
	// 			return c.Send("Возврат в предыдущее меню:", previousMarkup)
	// 		})
	// 	}
	// }
}
