package manager

import (
	"github.com/sambly/exchangebot/internal/telegram/menu/account"
	"github.com/sambly/exchangebot/internal/telegram/menu/entry"
	"github.com/sambly/exchangebot/internal/telegram/menu/strategies"
	tele "gopkg.in/telebot.v3"
)

// UserSession хранит текущее и предыдущее состояние пользователя.
type UserSession struct {
	CurrentState   string
	PreviousState  string
	CurrentMenu    string
	PreviousMenu   string
	PreviousMarkup *tele.ReplyMarkup // Предыдущая клавиатура
}

// MenuManager управляет всеми меню бота.
type MenuManager struct {
	Main      *entry.MainMenu
	Account   *account.AccountMenu
	Strategy  *strategies.StrategyMenu
	UserState map[int64]*UserSession // Хранит состояния пользователей
}

// NewMenuManager создаёт все меню.
func NewMenuManager() *MenuManager {
	mainMenu := entry.NewMainMenu("Главное меню:", "main")
	accountMenu := account.NewAccountMenu("Аккаунт:", "account")
	strategiesMenu := strategies.NewStrategyMenu("Стратегии:", "strategies")

	mainMenu.AddButtons(accountMenu.ButtonsHandler.EntryButton)
	mainMenu.AddButtons(strategiesMenu.ButtonsHandler.EntryButton)

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

// GetUserState возвращает текущее состояние пользователя.
func (m *MenuManager) GetUserState(userID int64) string {
	if session, exists := m.UserState[userID]; exists {
		return session.CurrentState
	}
	return ""
}

// GetPreviousState возвращает предыдущее состояние пользователя.
func (m *MenuManager) GetPreviousState(userID int64) string {
	if session, exists := m.UserState[userID]; exists {
		return session.PreviousState
	}
	return ""
}

// SetUserState устанавливает новое состояние пользователя и сохраняет предыдущее.
func (m *MenuManager) SetUserState(userID int64, state string) {
	if session, exists := m.UserState[userID]; exists {
		session.PreviousState = session.CurrentState
		session.CurrentState = state
	} else {
		m.UserState[userID] = &UserSession{
			CurrentState: state,
		}
	}
}

// SetUserMenu устанавливает текущее и предыдущее меню пользователя, сохраняя предыдущее состояние клавиатуры.
func (m *MenuManager) SetUserMenu(userID int64, menu string, markup *tele.ReplyMarkup) {
	if session, exists := m.UserState[userID]; exists {
		session.PreviousMenu = session.CurrentMenu
		session.CurrentMenu = menu
		session.PreviousMarkup = markup
	} else {
		m.UserState[userID] = &UserSession{
			CurrentMenu:    menu,
			PreviousMarkup: markup,
		}
	}
}

// GetCurrentMenu возвращает текущее меню пользователя.
func (m *MenuManager) GetCurrentMenu(userID int64) string {
	if session, exists := m.UserState[userID]; exists {
		return session.CurrentMenu
	}
	return ""
}

// GetPreviousMenu возвращает предыдущее меню пользователя.
func (m *MenuManager) GetPreviousMenu(userID int64) string {
	if session, exists := m.UserState[userID]; exists {
		return session.PreviousMenu
	}
	return ""
}

// GetPreviousMarkup возвращает предыдущую клавиатуру пользователя.
func (m *MenuManager) GetPreviousMarkup(userID int64) *tele.ReplyMarkup {
	if session, exists := m.UserState[userID]; exists {
		return session.PreviousMarkup
	}
	return nil
}
