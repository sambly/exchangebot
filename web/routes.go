package web

import (
	"crypto/sha256"
	"crypto/subtle"
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

	production := false
	mux := http.NewServeMux()

	mux.HandleFunc("/trade/formingPage", app.basicAuth(app.formingPage))
	mux.HandleFunc("/trade/updatefull", app.basicAuth(app.updateFull))
	mux.HandleFunc("/trade/getChangeDelta", app.basicAuth(app.getChangeDelta))
	mux.HandleFunc("/trade/updateTop", app.basicAuth(app.updateTop))
	mux.HandleFunc("/trade/openDeal", app.basicAuth(app.openDeal))
	mux.HandleFunc("/trade/closeDeal", app.basicAuth(app.closeDeal))
	mux.HandleFunc("/trade/ws", app.basicAuth(app.echo))

	//mux.Handle("/", http.FileServer(http.FS(getFrontendAssets(production))))

	// fileServer := http.FileServer(http.Dir("./static/"))
	// mux.Handle("/get-pays/static/", http.StripPrefix("/get-pays/static", fileServer))

	// fileServer := http.FileServer(http.Dir("./static/"))
	// mux.HandleFunc("/static/", app.basicAuth(http.StripPrefix("/static", fileServer).ServeHTTP))

	fileServer := http.FileServer(http.FS(getFrontendAssets(production)))
	mux.HandleFunc("/trade/", app.basicAuth(http.StripPrefix("/trade", fileServer).ServeHTTP))

	//mux.HandleFunc("/trade", app.basicAuth.ServeHTTP))

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

func (app *Web) middle(next http.HandlerFunc, authBase bool) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if authBase {
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
		} else {
			next.ServeHTTP(w, r)
		}

	})
}
