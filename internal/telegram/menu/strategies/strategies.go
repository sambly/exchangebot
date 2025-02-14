package strategies

import (
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
	}
)

type StrategyMenu struct {
	*base.BaseMenu
}

func NewStrategyMenu(name, id string) *StrategyMenu {
	menu := &StrategyMenu{
		BaseMenu: base.NewBaseMenu(name, id),
	}

	menu.BaseMenu.WithEntryButton(entryButton)
	menu.BaseMenu.AddButtons(defaultButtons...)

	return menu
}

// Handle обрабатывает кнопки меню стратегий.
func (m *StrategyMenu) Handle(b *tele.Bot, handler model.MenuHandler) {
	// Обработчик кнопки входа в меню стратегий
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

		// Отправляем сообщение с меню стратегий
		return c.Send("Меню стратегий:", m.Markup)
	})

	// // Обработчик кнопки "Назад"
	// for _, btn := range m.ButtonsHandler.Buttons {
	// 	if btn.ID == "btnStrategiesBack" {
	// 		b.Handle(&btn.TgBtn, func(c tele.Context) error {
	// 			userID := c.Sender().ID

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
