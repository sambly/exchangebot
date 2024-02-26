package web

import (
	"main/application"
	"main/notification"
	"net/http"

	"github.com/gorilla/websocket"
)

type Web struct {
	App   *application.Application
	Files []string
	Sockets
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

func NewWeb(app *application.Application, socketsMessage *notification.SocketsMessage) *Web {
	web := &Web{
		App: app,
	}
	web.Sockets = Sockets{
		clients:        make(map[*websocket.Conn]bool),
		socketsMessage: socketsMessage,
	}

	return web
}

func (w *Web) Run() {
	w.Sockets.SendDataRun()
	go http.ListenAndServe(":80", w.routes())
}
