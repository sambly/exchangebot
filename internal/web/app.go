package web

import (
	"context"
	"embed"
	"net/http"
	"sync"
	"time"

	"github.com/sambly/exchangebot/internal/application"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/notification"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/acme/autocert"
)

type Web struct {
	mu     sync.Mutex
	server *http.Server
	App    *application.Application
	Sockets

	listenPort   string
	hostname     string
	useTLS       bool
	contentEmbed bool
	content      embed.FS
	auth         auth

	telegramWebhookPath    string
	telegramWebhookHandler http.Handler
}

type auth struct {
	username string
	password string
}

type Sockets struct {
	clients        sync.Map
	socketsMessage *notification.SocketsMessage
}

var appWebLogger = logger.AddFields(map[string]interface{}{
	"package": "web",
})

// writeTimeout - сколько ждём медленного WS-клиента, прежде чем отключить его.
// Без дедлайна WriteMessage к залипшему клиенту блокирует рассылку всем
// остальным, а очередь сокетов начинает переполняться и терять сообщения.
const writeTimeout = 2 * time.Second

// Таймауты HTTP-сервера.
//
// ReadTimeout и WriteTimeout здесь НЕ выставляются намеренно: они превращаются
// в дедлайны на самом соединении, а после Upgrade (hijack) это соединение живёт
// как веб-сокет - и дедлайн его убьёт. Раньше в TLS-режиме стоял
// WriteTimeout: 5s, то есть каждый веб-сокет рвался через 5 секунд.
//
// От медленных клиентов защищаемся точечно: заголовки - ReadHeaderTimeout,
// простаивающие keep-alive соединения - IdleTimeout, запись в веб-сокет -
// SetWriteDeadline в SendDataRun.
const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 120 * time.Second
)

func (c *Sockets) SendDataRun(ctx context.Context) {
	go func(message chan []byte) {
		for {
			select {
			case mes := <-message:
				c.clients.Range(func(key, value interface{}) bool {
					conn := key.(*websocket.Conn)

					if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
						appWebLogger.Errorf("error set write deadline: %v", err)
						conn.Close()
						c.clients.Delete(conn)
						return true
					}

					err := conn.WriteMessage(websocket.TextMessage, mes)
					if err != nil {
						appWebLogger.Errorf("error writing message to websocket: %v", err)
						conn.Close()
						c.clients.Delete(conn)
					}
					return true
				})
			case <-ctx.Done():
				appWebLogger.Info("Shutting down socket message processing")
				return
			}
		}
	}(c.socketsMessage.Message)
}

func NewWeb(app *application.Application, socketsMessage *notification.SocketsMessage, cfg config.Web, content embed.FS) *Web {
	web := &Web{
		App: app,

		listenPort: cfg.ListenPort,
		hostname:   cfg.Host,
		useTLS:     cfg.UseTLC,

		contentEmbed: cfg.ContentEmbed,
		content:      content,
	}
	web.Sockets = Sockets{
		socketsMessage: socketsMessage,
	}

	auth := auth{username: cfg.UsernameAuth, password: cfg.PasswordAuth}
	web.auth = auth

	return web
}

// SetTelegramWebhook монтирует хендлер Telegram-вебхука на указанный путь.
// Должен вызываться до Run - роуты собираются один раз при старте сервера.
func (w *Web) SetTelegramWebhook(path string, handler http.Handler) {
	w.telegramWebhookPath = path
	w.telegramWebhookHandler = handler
}

func (w *Web) Run(ctx context.Context) error {
	w.Sockets.SendDataRun(ctx)

	serverErrChan := make(chan error, 1)

	go func() {
		var err error
		if w.useTLS {
			err = w.serveTLS()
		} else {
			err = w.serve()
		}
		serverErrChan <- err
	}()
	var err error

	select {
	case <-ctx.Done():
		// Завершение работы сервера
		stopErr := w.stop()
		if stopErr != nil {
			err = stopErr
			appWebLogger.Errorf("ошибка при завершении работы HTTP-сервера: %v", stopErr)

		}
	case serverErr := <-serverErrChan:
		if serverErr != nil {
			err = serverErr
			appWebLogger.Errorf("ошибка во время работы сервера: %v", err)
		}
	}

	appWebLogger.Info("HTTP сервер завершен")

	return err
}

func (w *Web) serveTLS() error {

	appWebLogger.Info("Запуск HTTP TLS сервера ")

	certManager := &autocert.Manager{
		Cache:      autocert.DirCache("certs"),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(w.hostname),
	}

	srv := &http.Server{
		Addr:              ":443",
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		Handler:           w.routes(),
		TLSConfig:         certManager.TLSConfig(),
	}
	w.mu.Lock()
	w.server = srv
	w.mu.Unlock()

	return srv.ListenAndServeTLS("", "")
}

func (w *Web) serve() error {

	appWebLogger.Infof("Запуск HTTP сервера port:%s ", w.listenPort)
	srv := &http.Server{
		Addr:              ":" + w.listenPort,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		Handler:           w.routes(),
	}
	w.mu.Lock()
	w.server = srv
	w.mu.Unlock()
	return srv.ListenAndServe()
}

func (w *Web) stop() error {
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	w.mu.Lock()
	srv := w.server
	w.mu.Unlock()

	if srv == nil {
		return nil
	}

	if err := w.server.Shutdown(ctxShutdown); err != nil {
		appWebLogger.Errorf("ошибка при завершении работы HTTP-сервера: %v", err)
		return err
	}
	return nil
}
