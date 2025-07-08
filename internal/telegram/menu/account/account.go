package account

import (
	"fmt"
	"slices"
	"strings"

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
	selectAsset = tele.Btn{Text: "Обзор пары"}

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
	Account      *account.Account
	AssetsPrices *prices.AssetsPrices
}

func NewAccountMenu(name, id string, account *account.Account, assetsPrices *prices.AssetsPrices) *AccountMenu {
	menu := &AccountMenu{
		BaseMenu:     base.NewBaseMenu(name, id),
		Account:      account,
		AssetsPrices: assetsPrices,
	}

	menu.WithEntryButton(entryButton)
	menu.AddButtonRows(defaultButtons...)
	menu.AddSubMenu(NewChangePeriodsMenu(menu))

	return menu
}

func (m *AccountMenu) Show(c tele.Context, handler model.MenuHandler) error {

	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show, m.HandleText)
	handler.DeleteUserMessages(c, userID)
	return c.Send("Меню аккаунта:", m.Markup)
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

		err := m.Account.UpdateAssets()
		if err != nil {
			return fmt.Errorf("error tlg getAssets: %v", err)
		}
		return c.Send("Данные обновлены", m.Markup)
	})

	b.Handle(&balance, func(c tele.Context) error {

		userID := c.Sender().ID
		handler.DeleteUserMessages(c, userID)

		marketStat := m.AssetsPrices.MarketsStat

		var out []string
		for _, asset := range m.Account.Assets {
			s := fmt.Sprintf("%s: %.1f💲  24ch: %-5.1f", asset.Name[:len(asset.Name)-len("USDT")], asset.CommonData.FullPrice, marketStat[asset.Name].Ch24)
			out = append(out, s)
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
		return c.Send(message, m.Markup)
	})

	b.Handle(&selectAsset, func(c tele.Context) error {

		userID := c.Sender().ID
		handler.DeleteUserMessages(c, userID)

		pairsText := "Выбери пару!!!!\n\nТекущие пары:\n"
		for _, pair := range m.Account.AssetsKey {
			pairsText += fmt.Sprintf("- %s\n", pair)
		}
		return c.Send(pairsText, m.Markup)
	})

}

func (m *AccountMenu) HandleText(c tele.Context) error {

	// Выбор определенной пары

	text := ""

	// Выбор определенной пары
	asset := strings.ToUpper(c.Text()) + "USDT"
	assets := m.Account.Assets
	assetsKey := m.Account.AssetsKey
	marketStat := m.AssetsPrices.MarketsStat
	change := m.AssetsPrices.ChangePrices
	if idx := slices.Index(assetsKey, asset); idx >= 0 {
		fullPrice := assets[asset].CommonData.FullPrice
		ch24 := marketStat[asset].Ch24

		text = fmt.Sprintf("----%s----\nСтоимость	%.1f\nch24	%.1f\n", asset, fullPrice, ch24)

		var fullPriceSpot, fullPriceFlexible, AssetStaking float64

		if assets[asset].SpotData != nil {
			fullPriceSpot = assets[asset].SpotData.FullPrice
		}
		if assets[asset].FlexibleData != nil {
			fullPriceFlexible = assets[asset].FlexibleData.FullPrice
		}
		if assets[asset].StakingData != nil {
			AssetStaking = assets[asset].StakingData.FullPrice
		}

		if fullPriceFlexible != 0 || AssetStaking != 0 {
			if fullPriceSpot > 0 {
				text = text + fmt.Sprintf("spot:	%.1f\n", fullPriceSpot)
			}
			if fullPriceFlexible > 0 {
				text = text + fmt.Sprintf("earn:	%.1f\n", fullPriceFlexible)
			}
			if AssetStaking > 0 {
				text = text + fmt.Sprintf("staking:	%.1f\n", AssetStaking)
			}

		}
		text = text + "----------------\n"
		for key, value := range change[asset] {
			text = text + fmt.Sprintf("%s		%.2f\n", key, value.ChangePercent)
		}
	} else {
		text = "Информации не найдено"
	}

	return c.Send(text)
}
