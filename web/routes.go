package web

import (
	"fmt"
	"io/fs"
	"main/fronted"
	"net/http"
	"os"
)

func getFrontendAssets(production bool) fs.FS {

	if production {
		fsys := fs.FS(fronted.Content)
		f, err := fs.Sub(fsys, "dist")
		if err != nil {
			fmt.Println(err)
		}

		return f
	} else {

		return os.DirFS("fronted/dist")
	}

}

func (app *Web) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/formingPage", app.formingPage)
	mux.HandleFunc("/updatefull", app.updateFull)
	mux.HandleFunc("/getChangeDelta", app.getChangeDelta)
	mux.HandleFunc("/updateTop", app.updateTop)
	mux.HandleFunc("/openDeal", app.openDeal)
	mux.HandleFunc("/closeDeal", app.closeDeal)
	mux.HandleFunc("/ws", app.echo)

	mux.Handle("/", http.FileServer(http.FS(getFrontendAssets(false))))

	return mux
}
