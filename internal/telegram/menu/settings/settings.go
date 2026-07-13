package settings

import (
	"github.com/sambly/exchangebot/internal/telegram/menu/base"
	"github.com/sambly/exchangebot/internal/telegram/menu/global"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	"github.com/sambly/exchangebot/internal/toggle"
	tele "gopkg.in/telebot.v3"
)

var (
	// Кнопка точка входа
	entryButton = tele.Btn{Text: "⚙️ Настройки"}
	// Базовые кнопки в меню
	replyButtons = [][]tele.Btn{
		{global.BtnBack, global.BtnMainMenu},
	}

	// Inline кнопки
	btnEnableNotifications  = tele.Btn{Text: "🔔 Включить уведомления", Unique: "enable_notif_tlg"}
	btnDisableNotifications = tele.Btn{Text: "🔕 Отключить уведомления", Unique: "disable_notif_tlg"}

	inlineButtons = [][]tele.Btn{
		{btnEnableNotifications, btnDisableNotifications},
	}
)

type SettingsMenu struct {
	*base.BaseMenu
	// notificationEnable читает горутина шины уведомлений, а переключают его
	// конкурентные обработчики telebot - отсюда потокобезопасный флаг вместо
	// поля конфига.
	notificationEnable *toggle.Bool
}

func NewSettingsMenu(name, id string, notificationEnable *toggle.Bool) *SettingsMenu {
	menu := &SettingsMenu{
		BaseMenu:           base.NewBaseMenu(name, id),
		notificationEnable: notificationEnable,
	}

	menu.AddButtonRows(replyButtons...)
	menu.WithEntryButton(entryButton)
	menu.AddButtonRowsInline(inlineButtons...)

	return menu
}

func (m *SettingsMenu) Show(c tele.Context, handler model.MenuHandler) error {

	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show, nil)
	handler.DeleteUserMessages(c, userID)

	text := "Настройки бота:\n"
	if m.notificationEnable.Get() {
		text += "Уведомления: включены"
	} else {
		text += "Уведомления: отключены"
	}

	if err := c.Send(text, m.Markup); err != nil {
		return err
	}

	// Кнопки inline отправляем отдельно
	if len(m.InlineButtons) > 0 {
		if err := c.Send("Уведомления:", m.InlineMarkup); err != nil {
			return err
		}
	}
	return nil
}

// Handle обрабатывает кнопки меню стратегий
func (m *SettingsMenu) Handle(b *tele.Bot, handler model.MenuHandler) {
	// Обработчик кнопки входа в меню стратегий
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})

	b.Handle(&btnEnableNotifications, func(c tele.Context) error {
		m.notificationEnable.Set(true)
		return c.Respond(&tele.CallbackResponse{Text: "Уведомления включены ✅", ShowAlert: true})
	})

	b.Handle(&btnDisableNotifications, func(c tele.Context) error {
		m.notificationEnable.Set(false)
		return c.Respond(&tele.CallbackResponse{Text: "Уведомления отключены ❌", ShowAlert: true})
	})

}
