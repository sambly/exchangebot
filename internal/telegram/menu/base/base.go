package base

import (
	tele "gopkg.in/telebot.v3"
)

// Базовое меню
type BaseMenu struct {
	Name           string
	ID             string
	Markup         *tele.ReplyMarkup
	ButtonsMarkup  [][]tele.Btn   // Все кнопки отоборажаемые в меню, вместе с EntryButton другого меню
	ButtonsHandler ButtonsHandler // Обработчики кнопок

	InlineMarkup  *tele.ReplyMarkup
	InlineButtons [][]tele.Btn
}

// Структура для хранения кнопок обработки событий
type ButtonsHandler struct {
	EntryButton tele.Btn
	Buttons     [][]tele.Btn
}

// Создает новое базовое меню
func NewBaseMenu(name, id string) *BaseMenu {
	menu := &BaseMenu{
		Name:          name,
		ID:            id,
		Markup:        &tele.ReplyMarkup{ResizeKeyboard: true},
		InlineMarkup:  &tele.ReplyMarkup{},
		ButtonsMarkup: [][]tele.Btn{},
		InlineButtons: [][]tele.Btn{},
		ButtonsHandler: ButtonsHandler{
			Buttons: [][]tele.Btn{},
		},
	}
	menu.updateMarkup()
	return menu
}

// Добавляет кнопку в меню
func (m *BaseMenu) AddButton(button tele.Btn, prepend bool) {
	newRow := []tele.Btn{button}
	if prepend {
		// Вставляем кнопку в начало списка
		m.ButtonsHandler.Buttons = append([][]tele.Btn{newRow}, m.ButtonsHandler.Buttons...)
	} else {
		// Добавляем кнопку в конец списка
		m.ButtonsHandler.Buttons = append(m.ButtonsHandler.Buttons, newRow)
	}
	m.updateMarkup()
}

// Добавляет кнопки в меню
func (m *BaseMenu) AddButtons(buttons ...[]tele.Btn) {
	m.ButtonsHandler.Buttons = append(m.ButtonsHandler.Buttons, buttons...)
	m.updateMarkup()
}
func (m *BaseMenu) AddButtonInline(button tele.Btn, prepend bool) {
	newRow := []tele.Btn{button}
	if prepend {
		// Вставляем кнопку в начало списка
		m.InlineButtons = append([][]tele.Btn{newRow}, m.InlineButtons...)
	} else {
		// Добавляем кнопку в конец списка
		m.InlineButtons = append(m.InlineButtons, newRow)
	}
	m.updateInlineMarkup()
}

func (m *BaseMenu) AddButtonsInline(buttons ...[]tele.Btn) {
	m.InlineButtons = append(m.InlineButtons, buttons...)
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

	m.ButtonsMarkup = append([][]tele.Btn{}, m.ButtonsHandler.Buttons...)

	var rows []tele.Row
	for _, btnRow := range m.ButtonsMarkup {
		rows = append(rows, tele.Row(btnRow))
	}
	m.Markup.Reply(rows...)
}
func (m *BaseMenu) updateInlineMarkup() {

	m.InlineButtons = append([][]tele.Btn{}, m.InlineButtons...)

	var rows []tele.Row
	for _, btnRow := range m.InlineButtons {
		rows = append(rows, tele.Row(btnRow))
	}

	m.InlineMarkup.Inline(rows...)
}
