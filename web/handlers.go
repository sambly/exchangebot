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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Пропускаем любой запрос
	},
}

func (web *Web) updateFull(w http.ResponseWriter, r *http.Request) {

	web.App.AssetsPrices.MarketsStatMu.RLock()
	defer web.App.AssetsPrices.MarketsStatMu.RUnlock()

	maps := map[string]interface{}{
		"MarketsStat": web.App.AssetsPrices.MarketsStat,
	}

	mapsJson, err := json.Marshal(maps)
	if err != nil {
		web.logError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(mapsJson)
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

	web.App.AssetsPrices.MarketsStatMu.RLock()
	defer web.App.AssetsPrices.MarketsStatMu.RUnlock()

	maps := map[string]interface{}{
		"Pairs":          web.App.AssetsPrices.Pairs,
		"MarketsStat":    web.App.AssetsPrices.MarketsStat,
		"OrdersActive":   web.App.PaperWallet.GetOrdersActive(),
		"OrdersHistory":  web.App.PaperWallet.GetOrdersHistory(),
		"OptionStrategy": option,
	}

	mapsJson, err := json.Marshal(maps)
	if err != nil {
		fmt.Println("ERROR1")
		web.logError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(mapsJson)
}

func (web *Web) getDeltaFast(w http.ResponseWriter, r *http.Request) {

	data := map[string]string{}

	bodyByte, _ := io.ReadAll(r.Body)

	if err := json.Unmarshal(bodyByte, &data); err != nil {
		web.logError(err)
	}
	w.Header().Set("Content-Type", "application/json")

	candles, err := web.App.AssetsPrices.GetDeltaPeriod(data["Pair"], data["Frame"])
	if err != nil {
		web.logError(err)
	}

	json.NewEncoder(w).Encode(candles)
}

func (web *Web) updateTop(w http.ResponseWriter, r *http.Request) {

	bodyByte, err := io.ReadAll(r.Body)
	if err != nil {
		web.logError(err)
	}

	web.App.AssetsPrices.MarketsStatMu.RLock()
	defer web.App.AssetsPrices.MarketsStatMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(web.App.AssetsPrices.MarketsStat[string(bodyByte)]) // string(bodyByte)  - pair
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
	json.NewEncoder(w).Encode(web.App.PaperWallet.GetOrdersActive())
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

	orders := map[string]interface{}{"OrdersActive": web.App.PaperWallet.GetOrdersActive(), "OrdersHistory": web.App.PaperWallet.GetOrdersHistory()}
	json.NewEncoder(w).Encode(orders)
}

func (web *Web) closeAllDeal(w http.ResponseWriter, r *http.Request) {

	// Делаем глубокую копию OrdersActive
	OrdersActiveCopy := make(map[string][]*model.Order)
	for key, value := range web.App.PaperWallet.OrdersActive {
		OrdersActiveCopy[key] = make([]*model.Order, len(value))
		for i, order := range value {
			// Делаем копию каждого элемента
			orderCopy := *order
			OrdersActiveCopy[key][i] = &orderCopy
		}
	}

	for _, orders := range OrdersActiveCopy {
		for _, order := range orders {
			err := web.App.OrderController.ClosePosition(order.ID)
			if err != nil {
				web.logError(err)
			}
		}

	}

	w.Header().Set("Content-Type", "application/json")

	orders := map[string]interface{}{"OrdersActive": web.App.PaperWallet.GetOrdersActive(), "OrdersHistory": web.App.PaperWallet.GetOrdersHistory()}
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

func (web *Web) getChPrice(w http.ResponseWriter, r *http.Request) {

	web.App.AssetsPrices.MarketsStatMu.RLock()
	defer web.App.AssetsPrices.MarketsStatMu.RUnlock()

	web.App.AssetsPrices.ChangePricesMu.RLock()
	defer web.App.AssetsPrices.ChangePricesMu.RUnlock()

	maps := map[string]interface{}{
		"MarketsStat":  web.App.AssetsPrices.MarketsStat,
		"ChangePrices": web.App.AssetsPrices.ChangePrices,
	}

	mapsJson, err := json.Marshal(maps)
	if err != nil {
		web.logError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(mapsJson)
}

func (web *Web) getChDelta(w http.ResponseWriter, r *http.Request) {

	web.App.AssetsPrices.DeltaFastMu.RLock()
	defer web.App.AssetsPrices.DeltaFastMu.RUnlock()

	maps := map[string]interface{}{
		"DeltaFast": web.App.AssetsPrices.DeltaFast,
	}

	mapsJson, err := json.Marshal(maps)
	if err != nil {
		web.logError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(mapsJson)
}

func (web *Web) exp(w http.ResponseWriter, r *http.Request) {

	bodyByte, err := io.ReadAll(r.Body)
	if err != nil {
		web.logError(err)
	}

	maps := map[string]interface{}{
		"ChangePricesForing": web.App.AssetsPrices.FormingChangePrices[string(bodyByte)]["ch12h"],
		"ChangePrices":       web.App.AssetsPrices.ChangePrices[string(bodyByte)]["ch12h"],
	}

	mapsJson, err := json.Marshal(maps)
	if err != nil {
		web.logError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(mapsJson)
}
