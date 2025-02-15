package manager

import (
	"github.com/sambly/exchangebot/internal/application"
	"github.com/sambly/exchangebot/internal/telegram/menu/account"
	"github.com/sambly/exchangebot/internal/telegram/menu/entry"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	"github.com/sambly/exchangebot/internal/telegram/menu/strategies"
	tele "gopkg.in/telebot.v3"
)

type UserSession struct {
	CurrentMenuFunc  func(c tele.Context, handler model.MenuHandler) error
	PreviousMenuFunc func(c tele.Context, handler model.MenuHandler) error
}

// MenuManager управляет всеми меню бота.
type MenuManager struct {
	Main      *entry.MainMenu
	Account   *account.AccountMenu
	Strategy  *strategies.StrategyMenu
	UserState map[int64]*UserSession // Хранит состояния пользователей

}

// NewMenuManager создаёт все меню.
func NewMenuManager(app *application.Application) *MenuManager {
	mainMenu := entry.NewMainMenu("Главное меню:", "main")
	accountMenu := account.NewAccountMenu("Аккаунт:", "account")
	strategiesMenu := strategies.NewStrategyMenu("Стратегии:", "strategies", app.ControllerStrategy)

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

	// Сохраняем текущее меню как предыдущее перед обновлением
	session.PreviousMenuFunc = session.CurrentMenuFunc
	session.CurrentMenuFunc = newMenuFunc
}

// GetPreviousMenu вызывает сохраненное предыдущее меню.
func (m *MenuManager) GetPreviousMenu(userID int64) func(c tele.Context, handler model.MenuHandler) error {
	if session, exists := m.UserState[userID]; exists {
		return session.PreviousMenuFunc
	}
	return nil
}
