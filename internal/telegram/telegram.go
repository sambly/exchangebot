package telegram

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sambly/exchangebot/internal/application"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/telegram/menu/manager"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	"github.com/sambly/exchangebot/internal/telegram/utils"

	tele "gopkg.in/telebot.v3"
)

type Telegram struct {
	bot             *tele.Bot
	menu            *manager.MenuManager
	callbakRegistry *utils.CallbackRegistry
	config          *config.Telegram
	user            int64

	app *application.Application
}

var tlgLogger = logger.AddFieldsEmpty()

func NewTelegram(app *application.Application, cfg config.Telegram) (*Telegram, error) {

	if cfg.User == "" || cfg.Token == "" {
		return nil, errors.New("telegram configuration is missing: user or token not provided")
	}

	user, _ := strconv.ParseInt(cfg.User, 10, 64)
	poller := &tele.LongPoller{Timeout: 10 * time.Second}
	callbakRegistry := utils.NewCallbackRegistry()
	menu := manager.NewMenuManager(app, user, callbakRegistry)

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
		bot:             bot,
		menu:            menu,
		app:             app,
		config:          &cfg,
		user:            user,
		callbakRegistry: callbakRegistry,
	}

	return tlg, nil
}

func (t *Telegram) Start(ctx context.Context) error {

	if t == nil {
		return errors.New("telegram not initialized")
	}
	menu := t.menu
	// Запускаем обработчики всех кнопок
	menu.InitHandlers(t.bot)

	// Универсальный обработчик  OnCallback по Unique
	t.bot.Handle(tele.OnCallback, func(c tele.Context) error {
		data := c.Callback().Data
		if !strings.HasPrefix(data, "\f") {
			return nil
		}
		parts := strings.Split(data, "|")
		unique := strings.TrimPrefix(parts[0], "\f")
		c.Callback().Data = strings.Join(parts[1:], "|")

		if handler, ok := t.callbakRegistry.GetHandler(unique); ok {
			return handler(c)
		}
		return nil
	})

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
	return ctx.Err()
}

func (t *Telegram) Send(message string) {
	if t.config.NotificationEnable {
		_, err := t.bot.Send(&tele.User{ID: t.user}, message)
		if err != nil {
			tlgLogger.Errorf("error sending message via Telegram: %v", err)
		}
	}
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
