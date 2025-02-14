package global

import tele "gopkg.in/telebot.v3"

// Глобальные кнопки
var (
	Markup      = &tele.ReplyMarkup{}
	BtnBack     = Markup.Text("🔙 Назад")
	BtnMainMenu = Markup.Text("🏠 Главное меню")
)
