package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"main/model"
	"main/prices"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

type Menu struct {
	Name string
	Url  string
}

type ViewData struct {
	Menu          []Menu
	Pairs         []string
	MarketsStat   map[string]*model.MarketsStat
	ChangePrices  map[string]map[string]*prices.ChangeData
	DeltaFast     map[string]map[string]*prices.DeltaFast
	OrdersActive  []*model.Order
	OrdersHistory []*model.Order
}

type Deal struct {
	Pair     string
	SideType string
	Strategy string
	Comment  string
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Пропускаем любой запрос
	},
}

func (web *Web) home(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := ViewData{
		Menu:          []Menu{{Name: "Главная", Url: "/"}},
		Pairs:         web.App.AssetsPrices.Pairs,
		MarketsStat:   web.App.AssetsPrices.MarketsStat,
		ChangePrices:  web.App.AssetsPrices.ChangePrices,
		DeltaFast:     web.App.AssetsPrices.DeltaFast,
		OrdersActive:  web.App.PaperWallet.OrdersActive(),
		OrdersHistory: web.App.PaperWallet.OrdersHistory(),
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

func (web *Web) openDeal(w http.ResponseWriter, r *http.Request) {

	bodyByte, _ := io.ReadAll(r.Body)

	deal := Deal{}

	if err := json.Unmarshal(bodyByte, &deal); err != nil {
		web.logError(err)
		return
	}

	size := 1.0

	var sideType model.SideType
	if deal.SideType == "buy" {
		sideType = model.SideTypeBuy
	}
	if deal.SideType == "sell" {
		sideType = model.SideTypeSell
	}

	_, err := web.App.OrderController.CreateOrderMarket(sideType, deal.Pair, size)
	if err != nil {
		web.logError(err)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(web.App.PaperWallet.OrdersActive())

}

func (web *Web) closeDeal(w http.ResponseWriter, r *http.Request) {
	bodyByte, err := io.ReadAll(r.Body)
	if err != nil {
		web.logError(err)
	}

	id, _ := strconv.ParseInt(string(bodyByte), 10, 64)

	err = web.App.OrderController.ClosePosition(id)
	if err != nil {
		web.logError(err)
	}

	w.Header().Set("Content-Type", "application/json")

	orders := map[string]interface{}{"OrdersActive": web.App.PaperWallet.OrdersActive(), "OrdersHistory": web.App.PaperWallet.OrdersHistory()}
	json.NewEncoder(w).Encode(orders)

}

func (web *Web) echo(w http.ResponseWriter, r *http.Request) {
	conn, _ := upgrader.Upgrade(w, r, nil) // error ignored for sake of simplicity
	_ = conn
	// for {
	// 	// Read message from browser
	// 	msgType, msg, err := conn.ReadMessage()
	// 	if err != nil {
	// 		return
	// 	}

	// 	// Print the message to the console
	// 	fmt.Printf("%s sent: %s\n", conn.RemoteAddr(), string(msg))

	// 	// Write message back to browser
	// 	if err = conn.WriteMessage(msgType, msg); err != nil {
	// 		return
	// 	}
	// }
}
