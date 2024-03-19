package web

import (
	"encoding/json"
	"fmt"
	"io"
	"main/model"
	"net/http"
	"os"
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
	ChangePrices  map[string]map[string]*model.ChangeData
	DeltaFast     map[string]map[string]*model.DeltaFast
	OrdersActive  []*model.Order
	OrdersHistory []*model.Order
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Пропускаем любой запрос
	},
}

func (web *Web) catchAllRoute(w http.ResponseWriter, r *http.Request) {
	fmt.Println("catchAllRoute URL :", r.URL.Path)
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

	mapsJson, err := json.Marshal(maps)
	if err != nil {
		web.logError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(mapsJson)

	//json.NewEncoder(w).Encode(maps)
}

func (web *Web) formingPage(w http.ResponseWriter, r *http.Request) {

	// Список стратегий
	optionByte, err := os.ReadFile("web/strategy.json")
	if err != nil {
		web.logError(err)
	}
	var option map[string]interface{}
	if err := json.Unmarshal(optionByte, &option); err != nil {
		web.logError(err)
	}

	maps := map[string]interface{}{
		"Pairs":          web.App.AssetsPrices.Pairs,
		"MarketsStat":    web.App.AssetsPrices.MarketsStat,
		"ChangePrices":   web.App.AssetsPrices.ChangePrices,
		"DeltaFast":      web.App.AssetsPrices.DeltaFast,
		"OrdersActive":   web.App.PaperWallet.OrdersActive(),
		"OrdersHistory":  web.App.PaperWallet.OrdersHistory(),
		"OptionStrategy": option,
	}

	mapsJson, err := json.Marshal(maps)
	if err != nil {
		web.logError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(mapsJson)

	//json.NewEncoder(w).Encode(maps)
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

	deal := model.Deal{}

	if err := json.Unmarshal(bodyByte, &deal); err != nil {
		web.logError(err)
		return
	}

	size := 1.0
	_, err := web.App.OrderController.CreateOrderMarket(deal, size)
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
	defer conn.Close()
	web.Sockets.clients[conn] = true        //Сохраняем соединение, используя его как ключ
	defer delete(web.Sockets.clients, conn) // Удаляем соединение
	for {
		mt, _, err := conn.ReadMessage()
		if err != nil || mt == websocket.CloseMessage {
			break // Выходим из цикла, если клиент пытается закрыть соединение или связь с клиентом прервана
		}

		for conn := range web.Sockets.clients {
			conn.WriteMessage(websocket.TextMessage, []byte("Hello"))
		}

	}

}
