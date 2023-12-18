package web

import (
	"net/http"
)

func (app *Web) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", app.home)

	fileServer := http.FileServer(http.Dir("web/static/"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	return mux
}
