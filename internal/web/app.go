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
	cfg    *config.Web
	server *http.Server
	app    *application.Application
	sockets

	content embed.FS
	auth    auth
}

type auth struct {
	username string
	password string
}

type sockets struct {
	clients        sync.Map
	socketsMessage *notification.SocketsMessage
}

var appWebLogger = logger.AddFields(map[string]interface{}{
	"package": "web",
})

func (c *sockets) SendDataRun(ctx context.Context) {
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

func NewWeb(app *application.Application, socketsMessage *notification.SocketsMessage, cfg *config.Web, content embed.FS) *Web {
	web := &Web{
		app:     app,
		cfg:     cfg,
		content: content,
	}
	web.sockets = sockets{
		socketsMessage: socketsMessage,
	}

	auth := auth{username: cfg.UsernameAuth, password: cfg.PasswordAuth}
	web.auth = auth

	return web
}

func (w *Web) Run(ctx context.Context) error {
	w.sockets.SendDataRun(ctx)

	serverErrChan := make(chan error, 1)

	go func() {
		var err error
		if w.cfg.UseTLC {
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
		HostPolicy: autocert.HostWhitelist(w.cfg.HostProduction),
	}

	srv := &http.Server{
		Addr:         ":443",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      w.routes(),
		TLSConfig:    certManager.TLSConfig(),
	}
	w.mu.Lock()
	w.server = srv
	w.mu.Unlock()

	return srv.ListenAndServeTLS("", "")
}

func (w *Web) serve() error {

	appWebLogger.Infof("Запуск HTTP сервера port:%s ", w.cfg.ListenPort)
	srv := &http.Server{
		Addr:    ":" + w.cfg.ListenPort,
		Handler: w.routes(),
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
