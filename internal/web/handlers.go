package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/sambly/exchangebot/internal/order"
	"gopkg.in/yaml.v3"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool {
		return true // Пропускаем любой запрос
	},
}

func (web *Web) updateFull(w http.ResponseWriter, _ *http.Request) {

	maps := map[string]interface{}{
		"MarketsStat": web.App.AssetsPrices.GetAllMarketsStat(),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(maps); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}

}

func (web *Web) formingPage(w http.ResponseWriter, _ *http.Request) {

	configPath := filepath.Join("configs", "strategy.yaml")
	var optionByte []byte

	// Список стратегий
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, []byte{}, 0644); err != nil {
			appWebLogger.Errorf("failed to create strategy file: %v", err)
			return
		}
		optionByte = []byte{}
	} else {
		optionByte, err = os.ReadFile(configPath)
		if err != nil {
			appWebLogger.Errorf("failed to read strategy file: %v", err)
			return
		}
	}

	var option map[string]interface{}
	if err := yaml.Unmarshal(optionByte, &option); err != nil {
		appWebLogger.Errorf("error yaml unmarshal: %v", err)
	}

	maps := map[string]interface{}{
		"Pairs":          web.App.AssetsPrices.Pairs,
		"MarketsStat":    web.App.AssetsPrices.GetAllMarketsStat(),
		"OrdersActive":   web.App.PaperWallet.GetOrdersActive(),
		"OrdersHistory":  web.App.PaperWallet.GetOrdersHistory(),
		"OptionStrategy": option,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(maps); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
}

func (web *Web) getDeltaFast(w http.ResponseWriter, r *http.Request) {

	data := map[string]string{}

	bodyByte, _ := io.ReadAll(r.Body)

	if err := json.Unmarshal(bodyByte, &data); err != nil {
		appWebLogger.Errorf("error json unmarshal: %v", err)
	}

	candles, err := web.App.AssetsPrices.GetDeltaPeriod(data["Pair"], data["Frame"])
	if err != nil {
		appWebLogger.Errorf("error GetDeltaPeriod: %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(candles); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
}

func (web *Web) updateTop(w http.ResponseWriter, r *http.Request) {

	bodyByte, err := io.ReadAll(r.Body)
	if err != nil {
		appWebLogger.Errorf("error readfile: %v", err)
	}
	pair := string(bodyByte)
	top := web.App.AssetsPrices.GetMarketsStatForPair(pair)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(top); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
}

func (web *Web) openDeal(w http.ResponseWriter, r *http.Request) {

	bodyByte, _ := io.ReadAll(r.Body)

	deal := order.Deal{}

	if err := json.Unmarshal(bodyByte, &deal); err != nil {
		appWebLogger.Errorf("error json unmarshal: %v", err)
		return
	}

	deal.Size = 1.0
	_, err := web.App.OrderController.CreateOrderMarket(deal)
	if err != nil {
		appWebLogger.Errorf("error CreateOrderMarket: %v", err)
	}

	orderActives := web.App.PaperWallet.GetOrdersActive()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(orderActives); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
}

func (web *Web) closeDeal(w http.ResponseWriter, r *http.Request) {
	bodyByte, err := io.ReadAll(r.Body)
	if err != nil {
		appWebLogger.Errorf("error readfile: %v", err)
	}

	id, _ := strconv.ParseInt(string(bodyByte), 10, 64)
	deal := order.Deal{Strategy: "manual"}

	if err := web.App.OrderController.ClosePosition(id, deal); err != nil {
		appWebLogger.Errorf("error ClosePosition: %v", err)
	}

	orders := map[string]interface{}{"OrdersActive": web.App.PaperWallet.GetOrdersActive(), "OrdersHistory": web.App.PaperWallet.GetOrdersHistory()}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(orders); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
}

func (web *Web) closeAllDeal(w http.ResponseWriter, _ *http.Request) {

	// Делаем глубокую копию OrdersActive
	OrdersActiveCopy := make(map[string][]*order.Order)
	for key, value := range web.App.PaperWallet.OrdersActive {
		OrdersActiveCopy[key] = make([]*order.Order, len(value))
		for i, order := range value {
			// Делаем копию каждого элемента
			orderCopy := *order
			OrdersActiveCopy[key][i] = &orderCopy
		}
	}
	deal := order.Deal{Strategy: "manual"}
	for _, orders := range OrdersActiveCopy {
		for _, order := range orders {
			if err := web.App.OrderController.ClosePosition(order.ID, deal); err != nil {
				appWebLogger.Errorf("error ClosePosition: %v", err)
			}
		}
	}

	orders := map[string]interface{}{"OrdersActive": web.App.PaperWallet.GetOrdersActive(), "OrdersHistory": web.App.PaperWallet.GetOrdersHistory()}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(orders); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
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
			if err := conn.WriteMessage(websocket.TextMessage, []byte("Hello")); err != nil {
				appWebLogger.Errorf("error Sockets: %v", err)
			}
		}
	}
}

func (web *Web) getChPrice(w http.ResponseWriter, _ *http.Request) {

	maps := map[string]interface{}{
		"MarketsStat":  web.App.AssetsPrices.GetAllMarketsStat(),
		"ChangePrices": web.App.AssetsPrices.GetAllChPrice(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(maps); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
}

func (web *Web) getChDelta(w http.ResponseWriter, _ *http.Request) {

	maps := map[string]interface{}{
		"DeltaFast": web.App.AssetsPrices.GetAllChDelta(),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(maps); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
}

func (web *Web) grafana(w http.ResponseWriter, r *http.Request) {
	// fmt.Println("REQUEST TO:", r.URL.Path, r.URL.RawQuery)
	target := "http://grafana:3000/"
	proxyURL, _ := url.Parse(target)
	proxy := httputil.NewSingleHostReverseProxy(proxyURL)

	// // Если запрос - WebSocket, меняем Director для корректного перенаправления
	// if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {
	// 	proxy.Director = func(req *http.Request) {
	// 		req.URL.Scheme = proxyURL.Scheme
	// 		req.URL.Host = proxyURL.Host
	// 		req.URL.Path = r.URL.Path
	// 		req.Header = r.Header
	// 	}
	// }

	proxy.ServeHTTP(w, r)
}

func (web *Web) jaeger(w http.ResponseWriter, r *http.Request) {
	target := "http://jaeger:16686/"
	proxyURL, _ := url.Parse(target)
	proxy := httputil.NewSingleHostReverseProxy(proxyURL)
	proxy.ServeHTTP(w, r)
}
