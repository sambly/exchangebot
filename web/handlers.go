package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"main/model"
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
	MarketsStat  map[string]*model.MarketsStat
	ChangePrices map[string]map[string]*prices.ChangeData
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
		MarketsStat:  web.App.AssetsPrices.MarketsStat,
		ChangePrices: web.App.AssetsPrices.ChangePrices,
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

	maps := map[string]interface{}{
		"MarketsStat":  web.App.AssetsPrices.MarketsStat,
		"ChangePrices": web.App.AssetsPrices.ChangePrices,
		"DeltaFast":    web.App.AssetsPrices.DeltaFast,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(maps)
}

// func (web *Web) updateFrame(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(web.App.AssetsPrices.DeltaFast)

// }

func (web *Web) getChangeDelta(w http.ResponseWriter, r *http.Request) {

	data := map[string]string{}

	bodyByte, _ := io.ReadAll(r.Body)

	if err := json.Unmarshal(bodyByte, &data); err != nil {
		web.logError(err)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(web.App.AssetsPrices.ChangeDelta[data["Pair"]][data["Frame"]])

}

func (web *Web) updateTop(w http.ResponseWriter, r *http.Request) {

	bodyByte, err := io.ReadAll(r.Body)
	if err != nil {
		web.logError(err)
	}
	pair := string(bodyByte)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(web.App.AssetsPrices.MarketsStat[pair])

}
