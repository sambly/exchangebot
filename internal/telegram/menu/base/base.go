package base

import (
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	tele "gopkg.in/telebot.v3"
)

// Базовое меню
type BaseMenu struct {
	Name           string
	ID             string
	Markup         *tele.ReplyMarkup
	InlineMarkup   *tele.ReplyMarkup
	ButtonsHandler ButtonsHandler // Обработчики кнопок
	InlineButtons  [][]tele.Btn
	SubMenus       []model.WindowHandler
}

// Структура для хранения кнопок обработки событий
type ButtonsHandler struct {
	EntryButton tele.Btn
	Buttons     [][]tele.Btn
}

// Создает новое базовое меню
func NewBaseMenu(name, id string) *BaseMenu {
	menu := &BaseMenu{
		Name:         name,
		ID:           id,
		Markup:       &tele.ReplyMarkup{ResizeKeyboard: true},
		InlineMarkup: &tele.ReplyMarkup{ResizeKeyboard: true},
		ButtonsHandler: ButtonsHandler{
			Buttons: make([][]tele.Btn, 0),
		},
		InlineButtons: make([][]tele.Btn, 0),
	}
	menu.updateMarkup()
	return menu
}

// Добавляет кнопку или группу кнопок в меню
func (m *BaseMenu) AddButtons(prepend bool, buttons ...tele.Btn) {
	if len(buttons) == 0 {
		return
	}
	newRow := buttons
	if prepend {
		m.ButtonsHandler.Buttons = append([][]tele.Btn{newRow}, m.ButtonsHandler.Buttons...)
	} else {
		m.ButtonsHandler.Buttons = append(m.ButtonsHandler.Buttons, newRow)
	}
	m.updateMarkup()
}

// Добавляет несколько строк кнопок
func (m *BaseMenu) AddButtonRows(buttonRows ...[]tele.Btn) {
	m.ButtonsHandler.Buttons = append(m.ButtonsHandler.Buttons, buttonRows...)
	m.updateMarkup()
}

// Добавляет кнопку или группу кнопок в inline-меню
func (m *BaseMenu) AddButtonsInline(prepend bool, buttons ...tele.Btn) {
	if len(buttons) == 0 {
		return
	}
	newRow := buttons
	if prepend {
		m.InlineButtons = append([][]tele.Btn{newRow}, m.InlineButtons...)
	} else {
		m.InlineButtons = append(m.InlineButtons, newRow)
	}
	m.updateInlineMarkup()
}

// Добавляет несколько строк inline-кнопок
func (m *BaseMenu) AddButtonRowsInline(buttonRows ...[]tele.Btn) {
	m.InlineButtons = append(m.InlineButtons, buttonRows...)
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
	var rows []tele.Row
	for _, btnRow := range m.ButtonsHandler.Buttons {
		rows = append(rows, m.Markup.Row(btnRow...)) // Используем встроенную Row
	}
	m.Markup.Reply(rows...)
}

// Обновляет отображение inline-кнопок
func (m *BaseMenu) updateInlineMarkup() {
	var rows []tele.Row
	for _, btnRow := range m.InlineButtons {
		rows = append(rows, m.InlineMarkup.Row(btnRow...)) // Аналогично для инлайн-кнопок
	}
	m.InlineMarkup.Inline(rows...)
}

// Добавляет подменю
func (m *BaseMenu) AddSubMenu(subMenu model.WindowHandler) {
	m.SubMenus = append(m.SubMenus, subMenu)
}
