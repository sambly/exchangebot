package web

import (
	"encoding/json"
	"log"
	"net/http"
)

type Response map[string]interface{}

func (app *Web) renderRepsonseJson(w http.ResponseWriter, message string, err error, statusCode int) {

	response := Response{"message": message, "err": ""}
	if err != nil {
		app.logError(err)
		response["err"] = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func (app *Web) serverError(w http.ResponseWriter, err error, status int) {

	app.logError(err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (app *Web) logError(err error) {
	//trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	log.Println(err.Error())
	//app.errorLog.Output(2, trace)
}

func (app *Web) renderRepsonseJsonToClient(w http.ResponseWriter, status string, code int, desc string, message, data string) {

	resp := make(map[string]interface{})
	resp["status"] = status
	resp["response_code"] = map[string]interface{}{"code": code, "desc": desc}
	resp["message"] = message
	resp["data"] = data

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(resp)
}
