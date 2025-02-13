package manager

import (
	"github.com/sambly/exchangebot/internal/telegram/menu/account"
	"github.com/sambly/exchangebot/internal/telegram/menu/entry"
	"github.com/sambly/exchangebot/internal/telegram/menu/strategies"
	"gopkg.in/telebot.v3"
)

// MenuManager управляет всеми меню бота.
type MenuManager struct {
	Main      *entry.MainMenu
	Account   *account.AccountMenu
	Strategy  *strategies.StrategyMenu
	UserState map[int64]string // Хранит текущее состояние пользователя
}

// NewMenuManager создаёт все меню.
func NewMenuManager() *MenuManager {
	return &MenuManager{
		Main:      entry.NewMainMenu(),
		Account:   account.NewAccountMenu(),
		Strategy:  strategies.NewStrategyMenu(),
		UserState: make(map[int64]string),
	}
}

// InitHandlers инициализирует обработчики всех меню.
func (m *MenuManager) InitHandlers(b *telebot.Bot) {
	m.Main.Handle(b, m)
	m.Account.Handle(b, m)
	m.Strategy.Handle(b, m)
}
