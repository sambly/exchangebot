package web

import (
	"encoding/json"
	"log"
	"net/http"
)

func (app *Web) logError(err error) {
	log.Println(err.Error())
}

func (app *Web) jsonEncode(w http.ResponseWriter, data interface{}) {
	if err := json.NewEncoder(w).Encode(data); err != nil {
		app.logError(err)
	}
}
