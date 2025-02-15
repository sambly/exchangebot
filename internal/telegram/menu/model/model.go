package model

import tele "gopkg.in/telebot.v3"

// MenuHandler определяет интерфейс для всех меню.
type MenuHandler interface {
	InitHandlers(b *tele.Bot)
	GetMainMenu() func(c tele.Context, handler MenuHandler) error
	SetCurrentMenu(userID int64, newFunc func(c tele.Context, handler MenuHandler) error)
	GetPreviousMenu(userID int64) func(c tele.Context, handler MenuHandler) error
}
