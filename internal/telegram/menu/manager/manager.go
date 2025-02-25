package manager

import (
	"fmt"

	"github.com/sambly/exchangebot/internal/application"
	"github.com/sambly/exchangebot/internal/telegram/menu/account"
	"github.com/sambly/exchangebot/internal/telegram/menu/entry"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	"github.com/sambly/exchangebot/internal/telegram/menu/strategies"
	tele "gopkg.in/telebot.v3"
)

type UserSession struct {
	CurrentMenuFunc func(c tele.Context, handler model.MenuHandler) error
	PreviousMenus   []func(c tele.Context, handler model.MenuHandler) error
	ActiveBntBack   bool
	Messages        []tele.Message
}

// MenuManager управляет всеми меню бота.
type MenuManager struct {
	Main     *entry.MainMenu
	Account  *account.AccountMenu
	Strategy *strategies.StrategyMenu
	// TODO Здесь возможно надо делать mutex
	UserState map[int64]*UserSession // Хранит состояния пользователей

}

// NewMenuManager создаёт все меню.
func NewMenuManager(app *application.Application) *MenuManager {
	mainMenu := entry.NewMainMenu("Главное меню:", "main")
	accountMenu := account.NewAccountMenu("Аккаунт:", "account", app.Account, app.AssetsPrices, app.BaseAmountAsset)
	strategiesMenu := strategies.NewStrategyMenu("Стратегии:", "strategies", app.ControllerStrategy)

	mainMenu.AddButton(accountMenu.ButtonsHandler.EntryButton, false)
	mainMenu.AddButton(strategiesMenu.ButtonsHandler.EntryButton, false)

	return &MenuManager{
		Main:      mainMenu,
		Account:   accountMenu,
		Strategy:  strategiesMenu,
		UserState: make(map[int64]*UserSession),
	}
}

// InitHandlers инициализирует обработчики всех меню.
func (m *MenuManager) InitHandlers(b *tele.Bot) {
	m.Main.Handle(b, m)
	m.Account.Handle(b, m)
	m.Strategy.Handle(b, m)
}

func (m *MenuManager) GetMainMenu() func(c tele.Context, handler model.MenuHandler) error {
	return m.Main.Show
}

// SetCurrentMenu устанавливает текущее и предыдущее меню.
func (m *MenuManager) SetCurrentMenu(userID int64, newMenuFunc func(c tele.Context, handler model.MenuHandler) error) {
	session, exists := m.UserState[userID]
	if !exists {
		session = &UserSession{}
		m.UserState[userID] = session
	}

	// Добавляем текущее меню в историю, если оно есть и не была нажат кнопка назад
	if session.CurrentMenuFunc != nil && !session.ActiveBntBack {
		session.PreviousMenus = append(session.PreviousMenus, session.CurrentMenuFunc)
	}
	session.ActiveBntBack = false
	session.CurrentMenuFunc = newMenuFunc
}

// GetPreviousMenu вызывает сохраненное предыдущее меню.
func (m *MenuManager) GetPreviousMenu(userID int64) func(c tele.Context, handler model.MenuHandler) error {
	session, exists := m.UserState[userID]
	if !exists || len(session.PreviousMenus) == 0 {
		return nil // Нет истории — некуда возвращаться
	}

	// Получаем последнее меню
	lastIndex := len(session.PreviousMenus) - 1
	prevMenu := session.PreviousMenus[lastIndex]

	// Удаляем его из истории
	session.PreviousMenus = session.PreviousMenus[:lastIndex]

	// Делаем его текущим меню
	session.CurrentMenuFunc = prevMenu

	return prevMenu
}

func (m *MenuManager) ResetPreviousMenu(userID int64) {
	if session, exists := m.UserState[userID]; exists {
		session.PreviousMenus = nil
	}
}

func (m *MenuManager) ActivateBntBack(userID int64) model.MenuHandler {
	if session, exists := m.UserState[userID]; exists {
		session.ActiveBntBack = true
	}
	return m
}

func (m *MenuManager) SaveMessage(userID int64, msg *tele.Message) {
	if msg == nil {
		return
	}
	session, exists := m.UserState[userID]
	if !exists {
		session = &UserSession{}
		m.UserState[userID] = session
	}

	session.Messages = append(session.Messages, *msg)
}

func (m *MenuManager) DeleteUserMessages(c tele.Context, userID int64) {

	var messages []tele.Editable
	session, exists := m.UserState[userID]
	if !exists {
		// TODO не знаю, надо ли здесь exists оставить или нет
		fmt.Println("Был вызван exists")
		return
	}
	// Удаление текущего сообщения
	if c.Message() != nil {
		messages = append(messages, c.Message())
	} else if c.Callback() != nil {
		messages = append(messages, c.Callback().Message)
	}
	// Удаление накопленных сообщений
	for i := range session.Messages {
		messages = append(messages, &session.Messages[i])
	}
	_ = c.Bot().DeleteMany(messages)
	session.Messages = nil
}

func (m *MenuManager) DeleteAllUserMessages(b *tele.Bot) {
	var messages []tele.Editable

	for _, session := range m.UserState {
		if len(session.Messages) == 0 {
			continue // Пропускаем, если сообщений нет
		}
		for i := range session.Messages {
			messages = append(messages, &session.Messages[i])
		}
		// Очищаем сохраненные сообщения
		session.Messages = nil
	}

	// Удаляем только если есть сообщения
	if len(messages) > 0 {
		_ = b.DeleteMany(messages)
	}
}
