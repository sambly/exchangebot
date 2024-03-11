package web

import (
	"main/application"
	"main/notification"
	"net/http"

	"github.com/gorilla/websocket"
)

type Web struct {
	App *application.Application
	Sockets
	production     bool
	productionPort string
	auth           auth
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

func NewWeb(app *application.Application, socketsMessage *notification.SocketsMessage, production bool, productionPort, username, password string) *Web {
	web := &Web{
		App:            app,
		production:     production,
		productionPort: productionPort,
	}
	web.Sockets = Sockets{
		clients:        make(map[*websocket.Conn]bool),
		socketsMessage: socketsMessage,
	}

	auth := auth{username: username, password: password}
	web.auth = auth

	return web
}

func (w *Web) Run() {
	w.Sockets.SendDataRun()

	if w.production {
		go http.ListenAndServe(":"+w.productionPort, w.routes())
	} else {
		go http.ListenAndServe(":80", w.routes())
	}

}
