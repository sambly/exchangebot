package web

import (
	"context"
	"embed"
	"fmt"
	"net/http"
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
	production   bool
	proxy        bool
	proxyPort    string
	contentEmbed bool
	content      embed.FS
	hostWeb      string
	auth         auth
}

type auth struct {
	username string
	password string
}

type Sockets struct {
	clients        map[*websocket.Conn]bool
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
				for conn := range c.clients {
					err := conn.WriteMessage(websocket.TextMessage, mes)
					if err != nil {
						appWebLogger.Errorf("error writing message to websocket: %v", err)
						conn.Close()
						delete(c.clients, conn)
					}
				}
			case <-ctx.Done():
				appWebLogger.Info("Shutting down socket message processing")
				return
			}
		}
	}(c.socketsMessage.Message)
}

func NewWeb(app *application.Application, socketsMessage *notification.SocketsMessage, cfg config.Web, content embed.FS) *Web {
	web := &Web{
		App:          app,
		production:   cfg.Production,
		proxy:        cfg.ProxyServer,
		proxyPort:    cfg.ProxyPort,
		contentEmbed: cfg.ContentEmbed,
		content:      content,
		hostWeb:      cfg.Host,
	}
	web.Sockets = Sockets{
		clients:        make(map[*websocket.Conn]bool),
		socketsMessage: socketsMessage,
	}

	auth := auth{username: cfg.UsernameAuth, password: cfg.PasswordAuth}
	web.auth = auth

	return web
}

func (w *Web) Run(ctx context.Context) error {
	w.Sockets.SendDataRun(ctx)

	// Создаем канал для передачи ошибок сервера
	serverErrChan := make(chan error, 1)

	go func() {
		var err error
		if w.production {
			if w.proxy {
				err = w.runProductionServerProxy()
			} else {
				err = w.runProductionServer()
			}
		} else {
			err = w.runLocalServer()
		}
		serverErrChan <- err
	}()
	var err error
	// Ожидание завершения контекста или ошибки сервера
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
			appWebLogger.Errorf("произошла ошибка во время работы сервера: %v", err)
		}
	}

	appWebLogger.Info("HTTP сервер завершен")

	return err
}

func (w *Web) runProductionServer() error {

	appWebLogger.Info("Запуск сервера: production")
	fmt.Println("Запуск сервера: production")

	certManager := &autocert.Manager{
		Cache:      autocert.DirCache("certs"),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(w.hostWeb),
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

func (w *Web) runProductionServerProxy() error {

	appWebLogger.Infof("Запуск сервера: через proxy server, port:%s ", w.proxyPort)
	fmt.Printf("Запуск сервера: через proxy server, port:%s\n", w.proxyPort)
	srv := &http.Server{
		Addr:    ":" + w.proxyPort,
		Handler: w.routes(),
	}
	w.server = srv
	return srv.ListenAndServe()
}

func (w *Web) runLocalServer() error {

	appWebLogger.Info("Запуск сервера: local")
	fmt.Println("Запуск сервера: local")

	srv := &http.Server{
		Addr:    ":80",
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
