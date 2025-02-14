package model

import tele "gopkg.in/telebot.v3"

type MenuHandler interface {
	InitHandlers(b *tele.Bot)
	GetUserState(userID int64) string
	GetPreviousState(userID int64) string
	SetUserState(userID int64, state string)
	GetCurrentMenu(userID int64) string
	GetPreviousMenu(userID int64) string
	SetUserMenu(userID int64, menu string, markup *tele.ReplyMarkup)
	GetPreviousMarkup(userID int64) *tele.ReplyMarkup
}

// // универсальная структура для кнопок
// type Button struct {
// 	Name  string
// 	ID    string
// 	TgBtn tele.Btn // Кнопка telebot
// }

// // TODO, здесь нет смыссла в id ,обычные кнопки хранят только текст
// func NewButton(name, id string) Button {
// 	return Button{
// 		Name:  name,
// 		ID:    id,
// 		TgBtn: tele.Btn{Text: name},
// 	}
// }
