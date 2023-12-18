package web

import (
	"fmt"
	"html/template"
	"main/prices"
	"net/http"
)

type Menu struct {
	Name string
	Url  string
}

type ViewData struct {
	Menu         []Menu
	Pairs        []string
	ChangePrices map[string]map[string]*prices.ChangeData
}

func (web *Web) home(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := ViewData{
		Menu:         []Menu{{Name: "Главная", Url: "/"}},
		Pairs:        web.App.AssetsPrices.Pairs,
		ChangePrices: web.App.AssetsPrices.ChangePrices,
	}

	ts, err := template.ParseFiles(web.Files...)
	if err != nil {
		web.serverError(w, fmt.Errorf("error parseFiles %v", err), http.StatusInternalServerError)
		return
	}

	err = ts.Execute(w, data)
	if err != nil {
		web.logError(err)
	}
}
