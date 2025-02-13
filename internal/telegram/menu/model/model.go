package model

import tele "gopkg.in/telebot.v3"

// EntryButton структура кнопки Меню
type EntryButton struct {
	Name   string   // Отображаемое имя кнопки
	ID     string   // Уникальный идентификатор кнопки
	Button tele.Btn // Кнопка telebot
}

func NewEntryButton(name, id string) *EntryButton {
	markup := &tele.ReplyMarkup{}

	btn := &EntryButton{
		Name:   name,
		ID:     id,
		Button: markup.Text(name),
	}

	return btn
}
