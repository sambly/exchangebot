package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sambly/exchangebot/internal/application"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/telegram/menu/manager"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	"github.com/sambly/exchangebot/internal/telegram/utils"
	"github.com/sambly/exchangebot/internal/toggle"

	tele "gopkg.in/telebot.v3"
)

type Telegram struct {
	bot             *tele.Bot
	menu            *manager.MenuManager
	callbakRegistry *utils.CallbackRegistry
	config          *config.Telegram
	user            int64
	webhook         *tele.Webhook

	// notificationEnable переключается из меню настроек (конкурентные
	// обработчики telebot), а читается из горутины шины уведомлений.
	notificationEnable *toggle.Bool

	app *application.Application
}

var tlgLogger = logger.AddFieldsEmpty()

const (
	retryAttempts  = 3
	retryBaseDelay = 2 * time.Second
)

// withRetry вызывает fn до attempts раз с экспоненциальной паузой между
// попытками (2s, 4s, 8s...), пока fn не вернёт nil или попытки не кончатся.
func withRetry(attempts int, baseDelay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i < attempts-1 {
			delay := baseDelay * time.Duration(int64(1)<<uint(i))
			tlgLogger.Warnf("попытка %d/%d не удалась: %v, повтор через %s", i+1, attempts, err, delay)
			time.Sleep(delay)
		}
	}
	return err
}

// redact вырезает токен бота из текста ошибки
func redact(err error, token string) error {
	if err == nil || token == "" {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), token, "***"))
}

func NewTelegram(app *application.Application, cfg config.Telegram) (*Telegram, error) {

	if cfg.User == "" || cfg.Token == "" {
		return nil, errors.New("telegram configuration is missing: user or token not provided")
	}

	user, _ := strconv.ParseInt(cfg.User, 10, 64)

	var poller tele.Poller
	var webhook *tele.Webhook
	if cfg.WebhookEnable {
		if cfg.WebhookURL == "" || cfg.WebhookPath == "" {
			return nil, errors.New("webhook-enable is true but webhook-url or webhook-path not provided")
		}
		// webhook-path идёт напрямую в http.ServeMux.Handle - без ведущего "/"
		// Go (1.22+) паникует при регистрации маршрута, а паника в горутине
		// web.Run() валит весь процесс в обход любой изоляции ошибок.
		if !strings.HasPrefix(cfg.WebhookPath, "/") {
			return nil, fmt.Errorf("webhook-path must start with \"/\", got %q", cfg.WebhookPath)
		}
		webhook = &tele.Webhook{
			Endpoint:    &tele.WebhookEndpoint{PublicURL: cfg.WebhookURL + cfg.WebhookPath},
			SecretToken: cfg.WebhookSecret,
		}
		poller = webhook
	} else {
		poller = &tele.LongPoller{Timeout: 10 * time.Second}
	}

	callbakRegistry := utils.NewCallbackRegistry()
	notificationEnable := toggle.New(cfg.NotificationEnable)
	menu := manager.NewMenuManager(app, user, callbakRegistry, notificationEnable)

	pref := tele.Settings{
		Token:  cfg.Token,
		Poller: poller,
		OnError: func(err error, c tele.Context) {
			tlgLogger.Errorf("telebot: %v", redact(err, cfg.Token))
		},
	}

	if cfg.UseCustomAPI {
		if cfg.CustomAPIURL == "" {
			return nil, errors.New("use-custom-api is enabled but custom-api-url is not provided")
		}
		pref.URL = cfg.CustomAPIURL
	}

	if cfg.UseProxy && cfg.ProxyURL == "" {
		return nil, errors.New("use_proxy is enabled but proxy_url is not provided")
	}

	proxyURL := ""
	if cfg.UseProxy {
		proxyURL = cfg.ProxyURL
	}
	client, err := newHTTPClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to configure http client: %w", err)
	}
	pref.Client = client

	var bot *tele.Bot
	err = withRetry(retryAttempts, retryBaseDelay, func() error {
		var err error
		bot, err = tele.NewBot(pref)
		return redact(err, cfg.Token)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	if webhook == nil {
		// На случай, если раньше был включен режим webhook: снимаем его,
		// иначе getUpdates (long-polling) будет падать с конфликтом на стороне Telegram.
		if err := bot.RemoveWebhook(); err != nil {
			tlgLogger.Warnf("не удалось снять webhook перед запуском long-polling: %v", redact(err, cfg.Token))
		}
	}

	bot.Use(userMiddleware(user))
	bot.Use(saveMessageMiddleware(menu))
	bot.Use(handleErrorMiddleware(cfg.Token))

	command := []tele.Command{
		{Text: "/start", Description: "Стартовая страница"},
	}
	err = withRetry(retryAttempts, retryBaseDelay, func() error {
		return redact(bot.SetCommands(command), cfg.Token)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set telegram commands: %w", err)
	}

	tlg := &Telegram{
		bot:                bot,
		menu:               menu,
		app:                app,
		config:             &cfg,
		user:               user,
		webhook:            webhook,
		callbakRegistry:    callbakRegistry,
		notificationEnable: notificationEnable,
	}

	return tlg, nil
}

// WebhookHandler возвращает http.Handler для приёма апдейтов от Telegram,
// если бот сконфигурирован в режиме webhook (WebhookEnable=true). В режиме
// long-polling возвращает nil - вызывающий код не должен монтировать роут.
func (t *Telegram) WebhookHandler() http.Handler {
	if t == nil || t.webhook == nil {
		return nil
	}
	return t.webhook
}

// WebhookPath - путь, на который нужно смонтировать WebhookHandler в HTTP-мультиплексоре.
func (t *Telegram) WebhookPath() string {
	if t == nil || t.config == nil {
		return ""
	}
	return t.config.WebhookPath
}

// newHTTPClient строит клиент для вызовов Bot API.
func newHTTPClient(rawProxyURL string) (*http.Client, error) {
	transport := &http.Transport{}

	if rawProxyURL != "" {
		proxyURL, err := url.Parse(rawProxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy url: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
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

	// Сбой уведомления (сеть/прокси/воркер разово недоступны) - не повод
	// ронить весь errgroup и вместе с ним торговлю и веб
	err := withRetry(retryAttempts, retryBaseDelay, func() error {
		_, err := t.bot.Send(
			&tele.User{ID: t.user},
			fmt.Sprintf("🚀 *Бот успешно запущен!*\n🔹*Сервер:* %s\n", t.app.Settings.ServerName),
			menu.Main.Markup,
		)
		return redact(err, t.config.Token)
	})
	if err != nil {
		tlgLogger.Errorf("не удалось отправить стартовое уведомление в Telegram: %v", err)
	} else {
		tlgLogger.Infof("Telegram started. Server name - %s", t.app.Settings.ServerName)
	}

	<-ctx.Done()

	t.menu.DeleteAllUserMessages(t.bot)
	err = withRetry(retryAttempts, retryBaseDelay, func() error {
		_, err := t.bot.Send(
			&tele.User{ID: t.user},
			fmt.Sprintf("⚠️ *Бот остановлен.*\n🔹 *Сервер:* %s\n", t.app.Settings.ServerName),
			menu.Main.Markup,
		)
		return redact(err, t.config.Token)
	})
	if err != nil {
		tlgLogger.Errorf("не удалось отправить уведомление об остановке в Telegram: %v", err)
	}

	// telebot v3.3.8 паникует в bot.Stop() для webhook-режима с пустым Listen:
	// Webhook.waitForStop закрывает stop-канал повторно (уже закрыт внутри
	// самого Bot.Start()) - "close of closed channel". В этом режиме своего
	// листенера, который нужно останавливать, у нас и нет - апдейты принимает
	// наш собственный HTTP-сервер со своим Shutdown в internal/web, поэтому
	// вызов просто пропускаем.
	if t.webhook == nil {
		t.bot.Stop()
	}

	tlgLogger.Infof("Telegram stopped gracefully. Server name - %s", t.app.Settings.ServerName)
	return ctx.Err()
}

func (t *Telegram) Send(message string) {
	if t.notificationEnable.Get() {
		_, err := t.bot.Send(&tele.User{ID: t.user}, message)
		if err != nil {
			tlgLogger.Errorf("error sending message via Telegram: %v", redact(err, t.config.Token))
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
func handleErrorMiddleware(token string) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			err := next(c)
			if err != nil {
				tlgLogger.Errorf("Ошибка в обработчике: %v", redact(err, token))

				if err := c.Send("❌ Произошла ошибка, попробуйте позже."); err != nil {
					return redact(err, token)
				}
			}
			return nil
		}
	}
}
