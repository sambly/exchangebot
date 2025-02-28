package account

import (
	"fmt"

	"github.com/sambly/exchangebot/internal/telegram/menu/base"
	"github.com/sambly/exchangebot/internal/telegram/menu/global"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	tele "gopkg.in/telebot.v3"
)

var (
	// Кнопка точка входа
	entryButtonChangePeriods = tele.Btn{Text: "Периоды"}

	period_1m  = tele.Btn{Text: "1m"}
	period_3m  = tele.Btn{Text: "3m"}
	period_15m = tele.Btn{Text: "15m"}
	period_1h  = tele.Btn{Text: "1h"}
	period_4h  = tele.Btn{Text: "4h"}
	period_1d  = tele.Btn{Text: "1d"}

	defaultButtonsChangePeriods = [][]tele.Btn{
		{period_1m},
		{period_3m},
		{period_15m},
		{period_1h},
		{period_4h},
		{period_1d},
		{global.BtnBack, global.BtnMainMenu},
	}
)

type ChangePeriodsMenu struct {
	*base.BaseMenu
	Account *AccountMenu
}

func NewChangePeriodsMenu(account *AccountMenu) *ChangePeriodsMenu {
	menu := &ChangePeriodsMenu{
		BaseMenu: base.NewBaseMenu("Периоды", "periods"),
		Account:  account,
	}

	menu.WithEntryButton(entryButtonChangePeriods)
	menu.AddButtons(defaultButtonsChangePeriods...)

	return menu
}

func (m *ChangePeriodsMenu) Show(c tele.Context, handler model.MenuHandler) error {

	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show)
	handler.DeleteUserMessages(c, userID)

	msg, err := c.Bot().Send(c.Chat(), "Выбери период:", m.Markup)
	if err == nil {
		handler.SaveMessage(userID, msg)
	}
	return err
}

func (m *ChangePeriodsMenu) Handle(b *tele.Bot, handler model.MenuHandler) {
	// Обработчик кнопки входа в аккаунт
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})

	b.Handle(&period_1m, func(c tele.Context) error {
		return m.handlePeriod(c, handler)
	})

	b.Handle(&period_3m, func(c tele.Context) error {
		return m.handlePeriod(c, handler)
	})

	b.Handle(&period_15m, func(c tele.Context) error {
		return m.handlePeriod(c, handler)
	})

	b.Handle(&period_4h, func(c tele.Context) error {
		return m.handlePeriod(c, handler)
	})

	b.Handle(&period_1d, func(c tele.Context) error {
		return m.handlePeriod(c, handler)
	})
}

func (m *ChangePeriodsMenu) handlePeriod(c tele.Context, handler model.MenuHandler) error {
	userID := c.Sender().ID
	handler.DeleteUserMessages(c, userID)

	var bufer string
	var message string
	period := c.Text()[1:]
	assets := m.getPeriods(period)
	for _, item := range assets {
		bufer = bufer + fmt.Sprintf("%s\n", item)
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
}

func (m *ChangePeriodsMenu) getPeriods(period string) []string {
	change := m.Account.AssetsPrices.ChangePrices
	var out []string
	for _, asset := range m.Account.Account.Assets {
		if _, ok := change[asset.Name][period]; ok {
			if asset.CommonData.FullPrice >= m.Account.BaseAmountAsset {
				s := fmt.Sprintf("%s:		%.2f", asset.Name[:len(asset.Name)-len("USDT")], change[asset.Name][period].ChangePercent)
				out = append(out, s)
			}
		}
	}
	return out
}
