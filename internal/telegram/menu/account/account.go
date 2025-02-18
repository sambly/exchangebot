package account

import (
	"fmt"

	"github.com/sambly/exchangebot/internal/account"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/telegram/menu/base"
	"github.com/sambly/exchangebot/internal/telegram/menu/global"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"

	tele "gopkg.in/telebot.v3"
)

var (
	// Кнопка точка входа
	entryButton = global.Markup.Text("📌 Аккаунт")

	balance = global.Markup.Text("BALANCE")

	// Базовые кнопки в меню
	defaultButtons = []tele.Btn{
		balance,
		global.BtnBack,
		global.BtnMainMenu,
	}
)

// Структура меню аккаунта
type AccountMenu struct {
	*base.BaseMenu
	Account         *account.Account
	AssetsPrices    *prices.AsetsPrices
	BaseAmountAsset float64
}

func NewAccountMenu(name, id string, account *account.Account, asetsPrices *prices.AsetsPrices, baseAmountAsset float64) *AccountMenu {
	menu := &AccountMenu{
		BaseMenu:        base.NewBaseMenu(name, id),
		Account:         account,
		AssetsPrices:    asetsPrices,
		BaseAmountAsset: baseAmountAsset,
	}

	menu.BaseMenu.WithEntryButton(entryButton)
	menu.BaseMenu.AddButtons(defaultButtons...)

	return menu
}

func (m *AccountMenu) Show(c tele.Context, handler model.MenuHandler) error {

	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show)
	handler.DeleteUserMessages(c, userID)

	msg, err := c.Bot().Send(c.Chat(), "Меню аккаунта:", m.Markup)
	if err == nil {
		handler.SaveMessage(userID, msg)
	}
	return err
}

// Handle обрабатывает кнопки меню аккаунта
func (m *AccountMenu) Handle(b *tele.Bot, handler model.MenuHandler) {
	// Обработчик кнопки входа в аккаунт
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})

	b.Handle(&balance, func(c tele.Context) error {

		if err := m.Account.UpdateAssets(); err != nil {
			return err
		}
		marketStat := m.AssetsPrices.MarketsStat

		var out []string
		for _, asset := range m.Account.Assets {
			if asset.CommonData.FullPrice >= m.BaseAmountAsset {
				s := fmt.Sprintf("%s: %.1f💲  24ch: %-5.1f", asset.Name[:len(asset.Name)-len("USDT")], asset.CommonData.FullPrice, marketStat[asset.Name].Ch24)
				out = append(out, s)
			}
		}

		bufer := ""
		for _, item := range out {
			bufer = bufer + item + "\n"
		}

		msg, err := c.Bot().Send(c.Chat(), bufer, m.Markup)
		if err == nil {
			handler.SaveMessage(c.Sender().ID, msg)
		}

		return nil
	})

}
