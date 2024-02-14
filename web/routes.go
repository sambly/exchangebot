package web

import (
	"io/fs"
	"main/fronted"
	"net/http"
)

func staticHandler() http.Handler {
	fsys := fs.FS(fronted.Content)
	contentStatic, _ := fs.Sub(fsys, "dist")
	return http.FileServer(http.FS(contentStatic))

}

func (app *Web) routes() *http.ServeMux {
	mux := http.NewServeMux()
	//mux.HandleFunc("/", app.home)
	mux.HandleFunc("/updatefull", app.updateFull)
	mux.HandleFunc("/getChangeDelta", app.getChangeDelta)
	mux.HandleFunc("/updateTop", app.updateTop)
	mux.HandleFunc("/openDeal", app.openDeal)
	mux.HandleFunc("/closeDeal", app.closeDeal)
	mux.HandleFunc("/ws", app.echo)

	mux.HandleFunc("/formingPage", app.formingPage)

	mux.Handle("/", staticHandler())

	// fileServer := http.FileServer(http.Dir("web/static/"))
	// mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	return mux
}
