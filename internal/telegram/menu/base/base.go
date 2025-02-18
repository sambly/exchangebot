package base

import (
	tele "gopkg.in/telebot.v3"
)

// Базовое меню
type BaseMenu struct {
	Name           string
	ID             string
	Markup         *tele.ReplyMarkup
	ButtonsMarkup  []tele.Btn     // Отображаемые кнопки в меню
	ButtonsHandler ButtonsHandler // Обработчики кнопок

	InlineMarkup  *tele.ReplyMarkup
	InlineButtons []tele.Btn
}

// Структура для хранения кнопок обработки событий
type ButtonsHandler struct {
	EntryButton tele.Btn
	Buttons     []tele.Btn
}

// Создает новое базовое меню
func NewBaseMenu(name, id string) *BaseMenu {
	menu := &BaseMenu{
		Name:          name,
		ID:            id,
		Markup:        &tele.ReplyMarkup{ResizeKeyboard: true},
		InlineMarkup:  &tele.ReplyMarkup{},
		ButtonsMarkup: []tele.Btn{},
		InlineButtons: []tele.Btn{},
		ButtonsHandler: ButtonsHandler{
			Buttons: []tele.Btn{},
		},
	}
	menu.updateMarkup()
	return menu
}

// Добавляет кнопки в меню
func (m *BaseMenu) AddButtons(buttons ...tele.Btn) {
	m.ButtonsHandler.Buttons = append(buttons, m.ButtonsHandler.Buttons...)
	m.updateMarkup()
}

func (m *BaseMenu) AddButtonsInline(buttons ...tele.Btn) {
	m.InlineButtons = append(buttons, m.InlineButtons...)
	m.updateInlineMarkup()
}

// Устанавливает кнопку входа
func (m *BaseMenu) WithEntryButton(button tele.Btn) {
	m.ButtonsHandler.EntryButton = button
}

func (m *BaseMenu) GetEntryButton() tele.Btn {
	return m.ButtonsHandler.EntryButton
}

// Обновляет отображение кнопок в меню
func (m *BaseMenu) updateMarkup() {
	m.ButtonsMarkup = append([]tele.Btn{}, m.ButtonsHandler.Buttons...)
	m.Markup.Reply(m.ButtonsMarkup)
}

func (m *BaseMenu) updateInlineMarkup() {
	m.InlineButtons = append([]tele.Btn{}, m.InlineButtons...)
	m.InlineMarkup.Inline(m.InlineButtons)
}
