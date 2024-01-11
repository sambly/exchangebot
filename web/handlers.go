package web

import (
	"encoding/json"
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
	ChangeDelta  map[string]map[string][]prices.ChangeDelta
	DeltaFast    map[string]map[string]*prices.DeltaFast
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
		ChangeDelta:  web.App.AssetsPrices.ChangeDelta,
		DeltaFast:    web.App.AssetsPrices.DeltaFast,
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

func (web *Web) updateFull(w http.ResponseWriter, r *http.Request) {

	err := web.App.AssetsPrices.UpdateDelta()
	if err != nil {
		web.logError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(web.App.AssetsPrices.DeltaFast)
}

func (web *Web) updateFrame(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(web.App.AssetsPrices.DeltaFast)

}
