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
	"github.com/sambly/exchangebot/internal/telegram/menu/model"

	tele "gopkg.in/telebot.v3"
)

type Settings struct {
	NotificationEnable bool
}

type Telegram struct {
	bot  *tele.Bot
	menu *manager.MenuManager
	*config.Telegram
	user int64

	app      *application.Application
	messages *notification.Notification
}

var tlgLogger = logger.AddFieldsEmpty()

func NewTelegram(app *application.Application, cfg config.Telegram, notification *notification.Notification) (*Telegram, error) {

	user, _ := strconv.ParseInt(cfg.User, 10, 64)
	poller := &tele.LongPoller{Timeout: 10 * time.Second}
	menu := manager.NewMenuManager(app)

	pref := tele.Settings{
		Token:  cfg.Token,
		Poller: poller,
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}

	bot.Use(userMiddleware(user))
	bot.Use(saveMessageMiddleware(menu))
	bot.Use(handleErrorMiddleware())

	command := []tele.Command{
		{Text: "/start", Description: "Стартовая страница"},
	}
	err = bot.SetCommands(command)
	if err != nil {
		return nil, err
	}

	tlg := &Telegram{
		bot:      bot,
		menu:     menu,
		app:      app,
		messages: notification,
		Telegram: &cfg,
		user:     user,
	}

	return tlg, nil
}

func (t Telegram) Start(ctx context.Context) error {
	menu := t.menu
	// Запускаем обработчики всех кнопок
	menu.InitHandlers(t.bot)

	go t.bot.Start()
	_, err := t.bot.Send(
		&tele.User{ID: t.user},
		fmt.Sprintf("🚀 *Бот успешно запущен!*\n🔹*Сервер:* %s\n", t.app.Settings.ServerName),
		menu.Main.Markup,
	)
	if err != nil {
		return err
	}
	tlgLogger.Infof("Telegram started. Server name - %s", t.app.Settings.ServerName)

	// Горутина для обработки входящих сообщений
	go func() {
		for {
			select {
			case mes, ok := <-t.messages.Message:
				if !ok {
					// Канал закрыт, выходим из горутины
					return
				}

				if t.NotificationEnable {
					_, err := t.bot.Send(&tele.User{ID: t.user}, mes, menu.Main.Markup)
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

	t.menu.DeleteAllUserMessages(t.bot)
	_, err = t.bot.Send(
		&tele.User{ID: t.user},
		fmt.Sprintf("⚠️ *Бот остановлен.*\n🔹 *Сервер:* %s\n", t.app.Settings.ServerName),
		menu.Main.Markup,
	)

	if err != nil {
		return err
	}

	t.bot.Stop()

	tlgLogger.Infof("Telegram stopped gracefully. Server name - %s", t.app.Settings.ServerName)
	return nil
}

// Middleware проверки пользователя
func userMiddleware(userID int64) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			user := c.Sender()
			if user == nil || user.ID != userID {
				tlgLogger.Debugf("Доступ запрещен: userID=%d", user.ID)
				return nil
			}
			return next(c)
		}
	}
}

type wrappedContext struct {
	tele.Context
	menuHandler model.MenuHandler
}

func (wc *wrappedContext) Send(what interface{}, opts ...interface{}) error {
	// Используем Bot().Send() вместо Context.Send(), чтобы получить MessageID
	msg, err := wc.Bot().Send(wc.Recipient(), what, opts...)
	if err == nil && msg != nil {
		wc.menuHandler.SaveMessage(wc.Sender().ID, msg)
	}
	return err
}

// Middleware для сохранения сообщений
func saveMessageMiddleware(menuHandler model.MenuHandler) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			// Оборачиваем context
			wc := &wrappedContext{Context: c, menuHandler: menuHandler}

			// Вызываем следующий обработчик
			err := next(wc)

			// Сохраняем входящее сообщение от пользователя
			if c.Message() != nil {
				menuHandler.SaveMessage(c.Sender().ID, c.Message())
			}

			return err
		}
	}
}

// Middleware для сохранения сообщений
func handleErrorMiddleware() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			err := next(c)
			if err != nil {
				tlgLogger.Errorf("Ошибка в обработчике: %v", err)

				if err := c.Send("❌ Произошла ошибка, попробуйте позже."); err != nil {
					return err
				}
			}
			return nil
		}
	}
}
