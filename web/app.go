package web

import (
	"log"
	"main/application"
	"main/config"
	"main/notification"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/acme/autocert"
)

type Web struct {
	App *application.Application
	Sockets
	production                    bool
	inProductionOnlyApp           bool
	inProductionWithFrontedNgingx bool
	productionPort                string
	hostWeb                       string
	auth                          auth
}

type auth struct {
	username string
	password string
}

type Sockets struct {
	clients        map[*websocket.Conn]bool
	socketsMessage *notification.SocketsMessage
}

func (c *Sockets) SendDataRun() {

	go func(message chan []byte) {
		for mes := range message {
			for conn := range c.clients {
				conn.WriteMessage(websocket.TextMessage, mes)
			}
		}
	}(c.socketsMessage.Message)
}

func NewWeb(app *application.Application, socketsMessage *notification.SocketsMessage, config *config.Config) *Web {
	web := &Web{
		App:                           app,
		production:                    config.Production,
		inProductionOnlyApp:           config.InProductionOnlyApp,
		inProductionWithFrontedNgingx: config.InProductionWithFrontedNgingx,
		productionPort:                config.HttpPortProduction,
		hostWeb:                       config.HostWeb,
	}
	web.Sockets = Sockets{
		clients:        make(map[*websocket.Conn]bool),
		socketsMessage: socketsMessage,
	}

	auth := auth{username: config.UsernameAuth, password: config.PasswordAuth}
	web.auth = auth

	return web
}

func (w *Web) Run() {
	w.Sockets.SendDataRun()

	if w.inProductionOnlyApp {

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

		log.Println("Запуск сервера: production")
		log.Fatal(srv.ListenAndServeTLS("", ""))
	}

	if w.inProductionWithFrontedNgingx {
		srvHttps := &http.Server{
			Addr:    ":" + w.productionPort,
			Handler: w.routes(),
		}

		log.Println("Запуск сервера: inProductionWithFrontedNgingx")
		log.Fatal(srvHttps.ListenAndServe())
	}

	if !w.inProductionOnlyApp && !w.inProductionWithFrontedNgingx {
		srv := &http.Server{
			Addr:    ":80",
			Handler: w.routes(),
		}
		log.Println("Запуск сервера: local")
		log.Fatal(srv.ListenAndServe())
	}

}
