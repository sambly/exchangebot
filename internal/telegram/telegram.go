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

func NewTelegram(app *application.Application, cfg config.Telegram, notification *notification.Notification) (*Telegram, error) {

	user, _ := strconv.ParseInt(cfg.User, 10, 64)
	poller := &tele.LongPoller{Timeout: 10 * time.Second}
	menu := manager.NewMenuManager(app)

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

	pref := tele.Settings{
		Token:  cfg.Token,
		Poller: userMiddleware,
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}

	// Middleware для обработки ошибок
	bot.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			err := next(c)
			if err != nil {
				tlgLogger.Errorf("Ошибка в обработчике: %v", err)

				msg, err := c.Bot().Send(c.Chat(), "❌ Произошла ошибка, попробуйте позже.", menu.Main.Markup)
				if err == nil {
					menu.SaveMessage(c.Sender().ID, msg)
				}

				return err
			}
			return nil
		}
	})

	command := []tele.Command{
		{Text: "/start", Description: "Стартовая страница"},
	}
	err = bot.SetCommands(command)
	if err != nil {
		return nil, err
	}

	tlg := &Telegram{
		client:             bot,
		menu:               menu,
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

	go t.client.Start()
	_, err := t.client.Send(
		&tele.User{ID: t.tlgUser},
		fmt.Sprintf("🚀 *Бот успешно запущен!*\n🔹*Сервер:* %s\n", t.app.Settings.ServerName),
		menu.Main.BaseMenu.Markup,
	)

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

	t.menu.DeleteAllUserMessages(t.client)
	_, err = t.client.Send(
		&tele.User{ID: t.tlgUser},
		fmt.Sprintf("⚠️ *Бот остановлен.*\n🔹 *Сервер:* %s\n", t.app.Settings.ServerName),
		menu.Main.BaseMenu.Markup,
	)

	if err != nil {
		return err
	}

	t.client.Stop()

	tlgLogger.Infof("Telegram stopped gracefully. Server name - %s", t.app.Settings.ServerName)
	return nil
}
