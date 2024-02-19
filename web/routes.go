package web

import (
	"fmt"
	"io/fs"
	"main/fronted"
	"net/http"
	"os"
)

func staticHandler() http.Handler {
	fsys := fs.FS(fronted.Content)
	contentStatic, _ := fs.Sub(fsys, "dist")
	return http.FileServer(http.FS(contentStatic))

}

func getFrontendAssets(production bool) fs.FS {

	fmt.Println("Hi")
	if production {
		fsys := fs.FS(fronted.Content)
		f, err := fs.Sub(fsys, "dist")
		if err != nil {
			fmt.Println(err)
		}

		return f
	} else {

		return os.DirFS("dist")
	}

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

	//frontend := getFrontendAssets(false)
	//mux.Handle("/", http.FileServer(http.FS(frontend)))

	mux.Handle("/", http.FileServer(http.FS(getFrontendAssets(true))))

	// fileServer := http.FileServer(http.Dir("/dist/index.html"))
	// mux.Handle("/", fileServer)

	//mux.Handle("/", staticHandler())

	// fileServer := http.FileServer(http.Dir("web/static/"))
	// mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	return mux
}
