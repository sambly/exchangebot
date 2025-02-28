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
	entryButton = tele.Btn{Text: "📌 Аккаунт"}

	updateData  = tele.Btn{Text: "Обновить данные"}
	balance     = tele.Btn{Text: "BALANCE"}
	selectAsset = tele.Btn{Text: "Выбрать пару"}

	defaultButtons = [][]tele.Btn{
		{updateData},
		{balance},
		{entryButtonChangePeriods},
		{selectAsset},
		{global.BtnBack, global.BtnMainMenu},
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

	menu.WithEntryButton(entryButton)
	menu.AddButtons(defaultButtons...)
	menu.AddSubMenu(NewChangePeriodsMenu(menu))

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

	// Подключаем обработчики подменю
	for _, subMenu := range m.SubMenus {
		subMenu.Handle(b, handler)
	}

	// Обработчик кнопки входа в аккаунт
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})

	b.Handle(&updateData, func(c tele.Context) error {

		userID := c.Sender().ID
		handler.DeleteUserMessages(c, userID)

		msg, err := c.Bot().Send(c.Chat(), "Данные обновлены", m.Markup)
		if err == nil {
			handler.SaveMessage(c.Sender().ID, msg)
		}

		return nil
	})

	b.Handle(&balance, func(c tele.Context) error {

		userID := c.Sender().ID
		handler.DeleteUserMessages(c, userID)

		marketStat := m.AssetsPrices.MarketsStat

		var out []string
		for _, asset := range m.Account.Assets {
			if asset.CommonData.FullPrice >= m.BaseAmountAsset {
				s := fmt.Sprintf("%s: %.1f💲  24ch: %-5.1f", asset.Name[:len(asset.Name)-len("USDT")], asset.CommonData.FullPrice, marketStat[asset.Name].Ch24)
				out = append(out, s)
			}
		}

		var bufer string
		var message string
		for _, item := range out {
			bufer = bufer + item + "\n"
		}

		if bufer == "" {
			message = "Данные отсутствуют"
		} else {
			message = bufer
		}

		msg, err := c.Bot().Send(c.Chat(), message, m.Markup)
		if err == nil {
			handler.SaveMessage(c.Sender().ID, msg)
		}

		return nil
	})

	b.Handle(&selectAsset, func(c tele.Context) error {

		userID := c.Sender().ID
		handler.DeleteUserMessages(c, userID)

		msg, err := c.Bot().Send(c.Chat(), "Выбрать пару!!!!", m.Markup)
		if err == nil {
			handler.SaveMessage(c.Sender().ID, msg)
		}

		return nil
	})

}
