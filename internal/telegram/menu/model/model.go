package model

import tele "gopkg.in/telebot.v3"

// MenuHandler определяет интерфейс для всех меню.
type MenuHandler interface {
	InitHandlers(b *tele.Bot)
	GetMainMenu() func(c tele.Context, handler MenuHandler) error
	GetUser() int64
	SetCurrentMenu(userID int64, newFunc func(c tele.Context, handler MenuHandler) error, handleTextFunc func(c tele.Context) error)
	GetPreviousMenu(userID int64) func(c tele.Context, handler MenuHandler) error
	ResetPreviousMenu(userID int64)
	SaveMessage(userID int64, msg *tele.Message)
	DeleteUserMessages(c tele.Context, userID int64)
	ActivateBntBack(userID int64) MenuHandler
}

type WindowHandler interface {
	AddButtons(prepend bool, buttons ...tele.Btn)
	AddButtonRows(buttonRows ...[]tele.Btn)
	AddButtonsInline(prepend bool, buttons ...tele.Btn)
	AddButtonRowsInline(buttonRows ...[]tele.Btn)
	WithEntryButton(button tele.Btn)
	GetEntryButton() tele.Btn
	Show(c tele.Context, handler MenuHandler) error
	Handle(b *tele.Bot, handler MenuHandler)
}
