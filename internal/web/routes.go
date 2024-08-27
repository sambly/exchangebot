package web

import (
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
)

func getFrontendAssets(production bool, content embed.FS) fs.FS {

	if production {
		fsys := fs.FS(content)
		f, err := fs.Sub(fsys, "frontend/dist")
		if err != nil {
			fmt.Println(err)
		}
		return f
	} else {

		return os.DirFS("../../frontend/dist")
	}
}

func (app *Web) routes() *http.ServeMux {

	mux := http.NewServeMux()

	mux.HandleFunc("/trade/formingPage", app.basicAuth(app.formingPage))
	mux.HandleFunc("/trade/updatefull", app.basicAuth(app.updateFull))
	mux.HandleFunc("/trade/getChangeDelta", app.basicAuth(app.getDeltaFast))
	mux.HandleFunc("/trade/updateTop", app.basicAuth(app.updateTop))
	mux.HandleFunc("/trade/openDeal", app.basicAuth(app.openDeal))
	mux.HandleFunc("/trade/closeDeal", app.basicAuth(app.closeDeal))
	mux.HandleFunc("/trade/ws", app.basicAuth(app.echo))

	mux.HandleFunc("/trade/getChPrice", app.basicAuth(app.getChPrice))
	mux.HandleFunc("/trade/getChDelta", app.basicAuth(app.getChDelta))

	mux.HandleFunc("/trade/closeAllDeal", app.basicAuth(app.closeAllDeal))

	mux.HandleFunc("/trade/grafana/", app.basicAuth(app.grafana))

	fileServer := http.FileServer(http.FS(getFrontendAssets(app.contentEmbed, app.content)))

	mux.HandleFunc("/trade/", app.basicAuth(http.StripPrefix("/trade", fileServer).ServeHTTP))

	return mux
}

func (app *Web) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		username, password, ok := r.BasicAuth()

		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))
			expectedUsernameHash := sha256.Sum256([]byte(app.auth.username))
			expectedPasswordHash := sha256.Sum256([]byte(app.auth.password))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {

				next.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}
