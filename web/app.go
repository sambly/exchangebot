package web

import (
	"main/application"
	"net/http"
)

type Web struct {
	App   *application.Application
	Files []string
}

func NewWeb(app *application.Application) *Web {
	web := &Web{
		App: app,
		Files: []string{
			"web/template/home.page.html",
			"web/template/base.layout.html",
		},
	}
	return web
}

func (w *Web) Run() {
	go http.ListenAndServe(":80", w.routes())
}
