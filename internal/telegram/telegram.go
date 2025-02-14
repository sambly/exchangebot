package telegram

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/sambly/exchangebot/internal/application"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/telegram/menu/manager"

	tele "gopkg.in/telebot.v3"
)

type Telegram struct {
	client *tele.Bot
	menu   *manager.MenuManager

	app                *application.Application
	tlgUser            int64
	Messages           *notification.Notification
	notificationEnable bool
}

var tlgLogger = logger.AddFieldsEmpty()

// UserState хранит состояние каждого пользователя (в каком меню он находится)
var UserState = make(map[int64]string)

func NewTelegram(app *application.Application, cfg config.Telegram, notification *notification.Notification) (*Telegram, error) {
	user, _ := strconv.ParseInt(cfg.User, 10, 64)

	pref := tele.Settings{
		Token:  cfg.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}

	tlg := &Telegram{
		client:             bot,
		menu:               manager.NewMenuManager(),
		app:                app,
		tlgUser:            user,
		Messages:           notification,
		notificationEnable: cfg.NotificationEnable,
	}

	return tlg, nil
}

func (t Telegram) Start(ctx context.Context) error {
	menu := t.menu
	// Запускаем обработчики всех кнопок
	menu.InitHandlers(t.client)

	menu.SetUserState(t.tlgUser, menu.Main.ID)
	menu.SetUserMenu(t.tlgUser, menu.Main.ID, menu.Main.BaseMenu.Markup)

	go t.client.Start()
	_, err := t.client.Send(&tele.User{ID: t.tlgUser}, fmt.Sprintf("Bot initialized. Server name - %s", t.app.Settings.ServerName), menu.Main.BaseMenu.Markup)
	if err != nil {
		return err
	}
	tlgLogger.Infof("Telegram started. Server name - %s", t.app.Settings.ServerName)

	// Горутина для обработки входящих сообщений
	go func() {
		for {
			select {
			case mes, ok := <-t.Messages.Message:
				if !ok {
					// Канал закрыт, выходим из горутины
					return
				}

				if t.notificationEnable {
					_, err := t.client.Send(&tele.User{ID: t.tlgUser}, mes, menu.Main.BaseMenu.Markup)
					if err != nil {
						tlgLogger.Errorf("error send message tlg: %v", err)
					}
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
	_, err = t.client.Send(&tele.User{ID: t.tlgUser}, fmt.Sprintf("Telegram stopped gracefully. Server name - %s", t.app.Settings.ServerName), menu.Main.BaseMenu.Markup)
	if err != nil {
		return err
	}

	t.client.Stop()

	tlgLogger.Infof("Telegram stopped gracefully. Server name - %s", t.app.Settings.ServerName)
	return nil
}
