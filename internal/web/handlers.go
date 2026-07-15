package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/strategy/executor"
	"gopkg.in/yaml.v3"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool {
		return true // Пропускаем любой запрос
	},
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

func (web *Web) openDeal(w http.ResponseWriter, r *http.Request) {

	bodyByte, _ := io.ReadAll(r.Body)

	deal := order.Deal{}

	if err := json.Unmarshal(bodyByte, &deal); err != nil {
		appWebLogger.Errorf("error json unmarshal: %v", err)
		http.Error(w, "некорректное тело запроса", http.StatusBadRequest)
		return
	}

	// Имя пары приходит из интерфейса и может нести лишние пробелы или перевод
	// строки. Без обрезки лукап падал с загадочным
	// "market stat for pair NEOUSDT  not found" - обрати внимание на два пробела.
	deal.Pair = strings.TrimSpace(deal.Pair)
	deal.Size = 1.0
	deal.Executor = executor.Web

	if _, err := web.App.OrderController.CreateOrderMarket(deal); err != nil {
		// Ошибку ОБЯЗАТЕЛЬНО отдаём наружу: фронт проверяет res.ok и показывает
		// toast. Раньше при ошибке уходил 200 OK, и интерфейс бодро сообщал
		// "Сделка открыта", хотя её не было.
		appWebLogger.Errorf("error CreateOrderMarket: %v", err)
		http.Error(w, fmt.Sprintf("не удалось открыть сделку: %v", err), http.StatusBadRequest)
		return
	}

	orderActives := web.App.OrderController.State.GetOrdersActiveCopy()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(orderActives); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
}

func (web *Web) closeDeal(w http.ResponseWriter, r *http.Request) {
	bodyByte, err := io.ReadAll(r.Body)
	if err != nil {
		appWebLogger.Errorf("error readfile: %v", err)
		http.Error(w, "не удалось прочитать тело запроса", http.StatusBadRequest)
		return
	}

	// Раньше ParseInt шёл без проверки, и мусорный id молча превращался в 0.
	id, err := strconv.ParseInt(strings.TrimSpace(string(bodyByte)), 10, 64)
	if err != nil {
		appWebLogger.Errorf("некорректный id ордера %q: %v", string(bodyByte), err)
		http.Error(w, "некорректный id ордера", http.StatusBadRequest)
		return
	}

	deal := order.Deal{Strategy: "manual", ExitReason: "manual", Executor: executor.Web}

	if err := web.App.OrderController.ClosePosition(id, deal); err != nil {
		appWebLogger.Errorf("error ClosePosition: %v", err)
		http.Error(w, fmt.Sprintf("не удалось закрыть сделку: %v", err), http.StatusBadRequest)
		return
	}

	orders := map[string]interface{}{
		"OrdersActive":  web.App.OrderController.State.GetOrdersActiveCopy(),
		"OrdersHistory": web.App.OrderController.State.GetOrdersHistoryCopy()}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(orders); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
}

func (web *Web) closeAllDeal(w http.ResponseWriter, _ *http.Request) {

	deal := order.Deal{Strategy: "manual", ExitReason: "manual", Executor: executor.Web}
	for _, orders := range web.App.OrderController.State.GetOrdersActiveCopy() {
		for _, order := range orders {
			if err := web.App.OrderController.ClosePosition(order.ID, deal); err != nil {
				appWebLogger.Errorf("error ClosePosition: %v", err)
			}
		}
	}

	orders := map[string]interface{}{
		"OrdersActive":  web.App.OrderController.State.GetOrdersActiveCopy(),
		"OrdersHistory": web.App.OrderController.State.GetOrdersHistoryCopy()}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(orders); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
}

// echo регистрирует WebSocket-клиента для рассылки обновлений (SendDataRun).
// Читающий цикл нужен только чтобы заметить закрытие соединения клиентом.
func (web *Web) echo(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		appWebLogger.Errorf("websocket upgrade: %v", err)
		return
	}
	defer conn.Close()

	web.Sockets.clients.Store(conn, true)
	defer web.Sockets.clients.Delete(conn)
	for {
		mt, _, err := conn.ReadMessage()
		if err != nil || mt == websocket.CloseMessage {
			break // Выходим из цикла, если клиент пытается закрыть соединение или связь с клиентом прервана
		}

	}
}

func (web *Web) getChPrice(w http.ResponseWriter, r *http.Request) {

	maps := map[string]interface{}{
		"MarketsStat":  web.App.AssetsPrices.GetAllMarketsStat(),
		"ChangePrices": web.App.AssetsPrices.GetAllChPrice(),
		"FeedStatus":   web.App.GetMarketPairsStatus(r.Context()),
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

func (web *Web) getOrders(w http.ResponseWriter, _ *http.Request) {

	maps := map[string]interface{}{
		"OrdersActive":  web.App.OrderController.State.GetOrdersActiveCopy(),
		"OrdersHistory": web.App.OrderController.State.GetOrdersHistoryCopy(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(maps); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
}

func (web *Web) getStrategies(w http.ResponseWriter, _ *http.Request) {

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

	var option map[string]any
	if err := yaml.Unmarshal(optionByte, &option); err != nil {
		appWebLogger.Errorf("error yaml unmarshal: %v", err)
	}

	maps := map[string]any{
		"OptionStrategy": option,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(maps); err != nil {
		appWebLogger.Errorf("error json encoder: %v", err)
	}
}
