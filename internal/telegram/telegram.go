package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sambly/exchangeBot/internal/application"
	"github.com/sambly/exchangeBot/internal/logger"
	"github.com/sambly/exchangeBot/internal/notification"

	"golang.org/x/exp/slices"
	tele "gopkg.in/telebot.v3"
)

type Telegram struct {
	defaultMenu        *tele.ReplyMarkup
	client             *tele.Bot
	app                *application.Application
	tlgUser            int64
	Messages           *notification.Notification
	notificationEnable bool
}

var tlgLogger = logger.AddFieldsEmpty()

func NewTelegram(app *application.Application, tlgToken, tlgUser string, notification *notification.Notification) (*Telegram, error) {

	poller := &tele.LongPoller{Timeout: 10 * time.Second}
	user, _ := strconv.ParseInt(tlgUser, 10, 64)

	userMiddleware := tele.NewMiddlewarePoller(poller, func(u *tele.Update) bool {
		if u.Message == nil || u.Message.Sender == nil {
			tlgLogger.Debug("No message")
			return false
		}
		if u.Message.Sender.ID == user {
			return true
		}
		tlgLogger.Debug("Invalid user")
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

	commandPeriods := []tele.Command{
		{Text: "/ch3m", Description: "Изменение за 3м"},
		{Text: "/ch15m", Description: "Изменение за 15м"},
		{Text: "/ch1h", Description: "Изменение за 1ч"},
		{Text: "/ch4h", Description: "Изменение за 4ч"},
	}
	err = client.SetCommands(commandPeriods)
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
		Messages:    notification,
	}

	client.Handle("/start", func(c tele.Context) error {
		return c.Send("Hello!", menu)
	})

	client.Handle(&btnBalance, func(c tele.Context) error {
		bufer := ""
		for _, item := range bot.getAssets() {
			bufer = bufer + item + "\n"
		}
		return c.Send(bufer)
	})

	client.Handle("/ch3m", bot.hanlePeriods)
	client.Handle("/ch15m", bot.hanlePeriods)
	client.Handle("/ch1h", bot.hanlePeriods)
	client.Handle("/ch4h", bot.hanlePeriods)
	client.Handle(tele.OnText, bot.differentMess)

	return bot, nil
}

func (t Telegram) Start(ctx context.Context) error {
	go t.client.Start()
	_, err := t.client.Send(&tele.User{ID: t.tlgUser}, fmt.Sprintf("Bot initialized. Server name - %s", t.app.Settings.ServerName), t.defaultMenu)
	if err != nil {
		return err
	}
	tlgLogger.Infof("Telegram started. Server name - %s", t.app.Settings.ServerName)

	go func(message chan string) {
		for {
			select {
			case mes := <-message:
				// Проверяем бит разрешения отправки уведомлений
				if t.notificationEnable {
					_, err := t.client.Send(&tele.User{ID: t.tlgUser}, mes, t.defaultMenu)
					if err != nil {
						tlgLogger.Errorf("error send message tlg: %v", err)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}(t.Messages.Message)

	<-ctx.Done()
	_, err = t.client.Send(&tele.User{ID: t.tlgUser}, fmt.Sprintf("Telegram stopped gracefully. Server name - %s", t.app.Settings.ServerName), t.defaultMenu)
	if err != nil {
		return err
	}
	t.client.Stop()

	tlgLogger.Infof("Telegram stopped gracefully. Server name - %s", t.app.Settings.ServerName)
	return nil
}

func (t Telegram) differentMess(c tele.Context) error {

	text := ""

	// Выбор определенной пары
	asset := strings.ToUpper(c.Text()) + "USDT"
	assets := t.app.Account.Assets
	assetsKey := t.app.Account.AssetsKey
	marketStat := t.app.AssetsPrices.MarketsStat
	change := t.app.AssetsPrices.ChangePrices
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
			text = text + fmt.Sprintf("%s		%.2f\n", key, value.СhangePercent)
		}
	}

	return c.Send(text)
}

func (t Telegram) hanlePeriods(c tele.Context) error {
	text := ""
	period := c.Text()[1:]
	assets := t.getPeriods(period)
	for _, item := range assets {
		text = text + fmt.Sprintf("%s\n", item)
	}
	return c.Send(text)
}

func (t Telegram) getPeriods(period string) []string {
	change := t.app.AssetsPrices.ChangePrices
	var out []string
	for _, asset := range t.app.Account.Assets {
		if _, ok := change[asset.Name][period]; ok {
			if asset.CommonData.FullPrice >= t.app.BaseAmountAsset {
				s := fmt.Sprintf("%s:		%.2f", asset.Name[:len(asset.Name)-len("USDT")], change[asset.Name][period].СhangePercent)
				out = append(out, s)
			}
		}
	}
	return out
}

func (t Telegram) getAssets() []string {
	err := t.app.Account.UpdateAssets()
	if err != nil {
		tlgLogger.Errorf("error tlg getAssets: %v", err)
	}
	marketStat := t.app.AssetsPrices.MarketsStat
	var out []string
	for _, asset := range t.app.Account.Assets {
		if asset.CommonData.FullPrice >= t.app.BaseAmountAsset {
			s := fmt.Sprintf("%s: %.1f💲  24ch: %-5.1f", asset.Name[:len(asset.Name)-len("USDT")], asset.CommonData.FullPrice, marketStat[asset.Name].Ch24)
			out = append(out, s)
		}
	}
	return out
}
