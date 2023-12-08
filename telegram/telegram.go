package telegram

import (
	"fmt"
	"main/application"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	tele "gopkg.in/telebot.v3"
)

type Telegram struct {
	defaultMenu *tele.ReplyMarkup
	client      *tele.Bot
	app         *application.Application
	tlgUser     int64
}

func NewTelegram(app *application.Application, tlgToken, tlgUser string) (*Telegram, error) {

	poller := &tele.LongPoller{Timeout: 10 * time.Second}
	user, _ := strconv.ParseInt(tlgUser, 10, 64)

	userMiddleware := tele.NewMiddlewarePoller(poller, func(u *tele.Update) bool {
		if u.Message == nil || u.Message.Sender == nil {
			fmt.Printf("no message, ", u.Message)
			return false
		}
		if u.Message.Sender.ID == user {
			return true
		}
		fmt.Printf("invalid user, ", u.Message)
		return false
	})

	client, err := tele.NewBot(tele.Settings{
		ParseMode: tele.ModeMarkdown,
		Token:     tlgToken,
		Poller:    userMiddleware,
	})
	if err != nil {
		return nil, err
	}
	var (
		menu       = &tele.ReplyMarkup{ResizeKeyboard: true}
		btnBalance = menu.Text("Balance")
	)

	command := []tele.Command{}
	err = client.SetCommands(command)
	if err != nil {
		return nil, err
	}

	menu.Reply(
		menu.Row(btnBalance),
	)

	bot := &Telegram{
		client:      client,
		defaultMenu: menu,
		app:         app,
		tlgUser:     user,
	}

	client.Handle("/start", func(c tele.Context) error {
		return c.Send("Hello!", menu)
	})

	client.Handle(&btnBalance, func(c tele.Context) error {
		fmt.Println("Зашли")
		bufer := ""
		for _, item := range bot.getAssets() {
			bufer = bufer + item + "\n"
		}
		fmt.Println(bufer)
		return c.Send(bufer)
	})

	client.Handle(tele.OnText, bot.differentMess)

	return bot, nil
}

func (t Telegram) Start() error {
	go t.client.Start()
	_, err := t.client.Send(&tele.User{ID: t.tlgUser}, "Bot initialized.", t.defaultMenu)
	if err != nil {
		return err
	}

	return nil
}

func (t Telegram) differentMess(c tele.Context) error {

	text := ""

	fmt.Println("----------------------")
	fmt.Println(c.Text())
	// Выбор определенной пары
	asset := strings.ToUpper(c.Text()) + "USDT"
	assets := t.app.Account.Assets
	assetsKey := t.app.Account.AssetsKey
	if idx := slices.Index(assetsKey, asset); idx >= 0 {
		fullPrice := assets[asset].CommonData.FullPrice
		//ch24 := t.app.Assets[asset].Ch_24

		//text = fmt.Sprintf("----%s----\nСтоимость	%.1f\nch24	%.1f\n", asset, fullPrice, ch24)
		text = fmt.Sprintf("----%s----\nСтоимость	%.1f\n", asset, fullPrice)

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
	}

	return c.Send(text)
}

func (t Telegram) getAssets() []string {
	err := t.app.Account.UpdateAssets()
	if err != nil {
		fmt.Println("эта ошибка")
		fmt.Println(err)
	}

	var out []string
	for _, asset := range t.app.Account.Assets {
		if asset.CommonData.FullPrice >= t.app.BaseAmountAsset {
			//s := fmt.Sprintf("%s: %.1f💲  24ch: %-5.1f", asset.Name[:len(asset.Name)-len("USDT")], asset.CommonData.FullPrice, asset.Ch_24)
			s := fmt.Sprintf("%s: %.1f💲 ", asset.Name[:len(asset.Name)-len("USDT")], asset.CommonData.FullPrice)
			out = append(out, s)
		}
	}
	return out
}
