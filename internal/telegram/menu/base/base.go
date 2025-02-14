package base

import (
	tele "gopkg.in/telebot.v3"
)

// Базовое меню
type BaseMenu struct {
	Name           string
	ID             string
	Markup         *tele.ReplyMarkup
	ButtonsMarkup  []tele.Btn     // Отображаемые кнопки
	ButtonsHandler ButtonsHandler // Обработчики кнопок
}

// Структура для хранения кнопок обработки событий
type ButtonsHandler struct {
	EntryButton tele.Btn
	Buttons     []tele.Btn
}

// Создает новое базовое меню
func NewBaseMenu(name, id string) *BaseMenu {
	menu := &BaseMenu{
		Name:   name,
		ID:     id,
		Markup: &tele.ReplyMarkup{ResizeKeyboard: true},
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

// Устанавливает кнопку входа
func (m *BaseMenu) WithEntryButton(button tele.Btn) {
	m.ButtonsHandler.EntryButton = button
}

// Обновляет отображение кнопок в меню
func (m *BaseMenu) updateMarkup() {
	m.ButtonsMarkup = []tele.Btn{}
	for _, btn := range m.ButtonsHandler.Buttons {
		m.ButtonsMarkup = append(m.ButtonsMarkup, btn)
	}
	m.Markup.Reply(m.ButtonsMarkup)
}
