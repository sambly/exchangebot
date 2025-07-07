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
	server *http.Server
	App    *application.Application
	Sockets

	listenPort   string
	hostname     string
	useTLS       bool
	contentEmbed bool
	content      embed.FS
	auth         auth
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

func (c *Sockets) SendDataRun(ctx context.Context) {
	go func(message chan []byte) {
		for {
			select {
			case mes := <-message:
				c.clients.Range(func(key, value interface{}) bool {
					conn := key.(*websocket.Conn)
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
		Addr:         ":443",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      w.routes(),
		TLSConfig:    certManager.TLSConfig(),
	}
	w.server = srv

	return srv.ListenAndServeTLS("", "")
}

func (w *Web) serve() error {

	appWebLogger.Infof("Запуск HTTP сервера port:%s ", w.listenPort)
	srv := &http.Server{
		Addr:    ":" + w.listenPort,
		Handler: w.routes(),
	}
	w.server = srv
	return srv.ListenAndServe()
}

func (w *Web) stop() error {
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := w.server.Shutdown(ctxShutdown); err != nil {
		appWebLogger.Errorf("ошибка при завершении работы HTTP-сервера: %v", err)
		return err
	}
	return nil
}
