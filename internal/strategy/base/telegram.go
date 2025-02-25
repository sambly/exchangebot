package base

import (
	"github.com/sambly/exchangebot/internal/telegram/menu/base"
	"github.com/sambly/exchangebot/internal/telegram/menu/global"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	tele "gopkg.in/telebot.v3"
)

var (
	// Кнопка точка входа
	entryButton = tele.Btn{Text: "BASE"}
	// Базовые кнопки в меню
	defaultButtons = [][]tele.Btn{
		{global.BtnBack, global.BtnMainMenu},
	}

	// Inline кнопки
	btnEnableNotifications  = tele.Btn{Text: "🔔 Включить уведомления", Unique: "enable_notif"}
	btnDisableNotifications = tele.Btn{Text: "🔕 Отключить уведомления", Unique: "disable_notif"}

	inlineButtons = [][]tele.Btn{
		{btnEnableNotifications, btnDisableNotifications},
	}
)

type StrategyBaseMenu struct {
	*base.BaseMenu
}

func NewStrategyMenu(name, id string) *StrategyBaseMenu {
	menu := &StrategyBaseMenu{
		BaseMenu: base.NewBaseMenu(name, id),
	}

	menu.AddButtons(defaultButtons...)
	menu.WithEntryButton(entryButton)
	menu.AddButtonsInline(inlineButtons...)

	return menu
}

func (m *StrategyBaseMenu) Show(c tele.Context, handler model.MenuHandler) error {

	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show)
	handler.DeleteUserMessages(c, userID)

	msg, err := c.Bot().Send(c.Chat(), "Base стратегия:", m.Markup)
	if err == nil {
		handler.SaveMessage(userID, msg)
	}

	// Кнопки inline отправляем отдельно
	if len(m.InlineButtons) > 0 {
		msg, err := c.Bot().Send(c.Chat(), "Выберите действие:", m.InlineMarkup)
		if err == nil {
			handler.SaveMessage(userID, msg)
		}
	}
	return err
}

// Handle обрабатывает кнопки меню стратегий
func (m *StrategyBaseMenu) Handle(b *tele.Bot, handler model.MenuHandler) {
	// Обработчик кнопки входа в меню стратегий
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})

	b.Handle(&btnEnableNotifications, func(c tele.Context) error {

		msg, err := c.Bot().Send(c.Chat(), "Уведомления включены ✅")
		if err != nil {
			return err
		}
		handler.SaveMessage(c.Sender().ID, msg)
		// Здесь логика отключения уведомлений
		return c.Respond(&tele.CallbackResponse{Text: "Уведомления включены ✅", ShowAlert: true})
	})

	b.Handle(&btnDisableNotifications, func(c tele.Context) error {

		msg, err := c.Bot().Send(c.Chat(), "Уведомления отключены ❌")
		if err != nil {
			return err
		}
		handler.SaveMessage(c.Sender().ID, msg)

		// Здесь логика отключения уведомлений
		return c.Respond(&tele.CallbackResponse{Text: "Уведомления отключены ❌", ShowAlert: true})
	})

}
