package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"main/internal/database"
	"main/internal/exchange"
	"main/internal/logging"
	"main/internal/model"
	"main/internal/notification"
	"main/internal/prices"
	"sync"
)

type Status string

type Controller struct {
	mtx      sync.Mutex
	ctx      context.Context
	exchange *exchange.PaperWallet
	database *sql.DB

	assetsPrices   *prices.AsetsPrices
	socketsMessage *notification.SocketsMessage
}

func NewController(ctx context.Context, ex *exchange.PaperWallet, db *sql.DB, socketsMessage *notification.SocketsMessage, assetsPrices *prices.AsetsPrices) (*Controller, error) {

	ctrl := &Controller{
		ctx:            ctx,
		exchange:       ex,
		database:       db,
		assetsPrices:   assetsPrices,
		socketsMessage: socketsMessage,
	}

	orders, err := database.Orders(db)
	if err != nil {
		return nil, err
	}
	countOrdersActive := 0
	for _, order := range orders {
		if order.Status == model.OrderStatusTypeActive {
			ex.AddOrderActive(order)
			countOrdersActive += 1
		}

		if order.Status == model.OrderStatusTypeClose {
			ex.AddOrderHistory(order)
		}

	}
	ex.Pnl.CountOrdersActive = countOrdersActive

	return ctrl, nil

}

func (c *Controller) CreateOrderMarket(deal model.Deal, size float64) (*model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	pair := deal.Pair

	var sideType model.SideType
	if deal.SideType == "buy" {
		sideType = model.SideTypeBuy
	}
	if deal.SideType == "sell" {
		sideType = model.SideTypeSell
	}

	order, err := c.exchange.CreateOrderMarket(sideType, pair, size)
	if err != nil {
		return nil, err
	}

	id, err := database.CreateOrder(c.database, order)
	if err != nil {
		return nil, err
	}
	order.ID = id

	mkStat := c.assetsPrices.MarketsStat[pair]
	chData := c.assetsPrices.ChangePrices[pair]
	dFast := c.assetsPrices.ChangeDelta[pair]

	mkStatJson, err := json.Marshal(mkStat)
	if err != nil {
		logging.MyLogger.ErrorOut(fmt.Errorf("error jsonmarshal mkStatJson : %v", err))
	}
	chDataJson, err := json.Marshal(chData)
	if err != nil {
		logging.MyLogger.ErrorOut(fmt.Errorf("error jsonmarshal chDataJson : %v", err))
	}
	dFastJson, err := json.Marshal(dFast)
	if err != nil {
		logging.MyLogger.ErrorOut(fmt.Errorf("error jsonmarshal dFastJson : %v", err))
	}

	err = database.InsertOrdersInfoTable(c.database, id, deal.Frame, deal.Strategy, deal.Comment, mkStatJson, chDataJson, dFastJson)
	if err != nil {
		logging.MyLogger.ErrorOut(fmt.Errorf("error when create order and add insertinfotables : %v", err))
	}

	return order, err
}

func (c *Controller) ClosePosition(id int64) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	order, err := c.exchange.ClosePosition(id)
	if err != nil {
		return err
	}

	err = database.ClosePosition(c.database, order, id)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) OnMarket(ms model.MarketsStat) {

	// Обновление ордеров

	if _, ok := c.exchange.OrdersActive[ms.Pair]; ok {

		for _, order := range c.exchange.OrdersActive[ms.Pair] {
			order.Price = ms.Price
			if order.Side == model.SideTypeBuy {
				order.Profit = (ms.Price / order.PriceCreated * 100) - 100
			}
			if order.Side == model.SideTypeSell {
				order.Profit = (order.PriceCreated / ms.Price * 100) - 100
			}
			// Обновления цены webSocket
			messageOrder, _ := json.Marshal(map[string]interface{}{"order": order})
			c.socketsMessage.SendData(messageOrder)
		}

		// Подсчет PNL
		c.exchange.Pnl.Profit = 0
		for _, listOrder := range c.exchange.OrdersActive {
			for _, order := range listOrder {
				c.exchange.Pnl.Profit += order.Profit
			}
		}
		messageOrder, _ := json.Marshal(map[string]interface{}{"pnl": c.exchange.Pnl.Profit})
		c.socketsMessage.SendData(messageOrder)

	}

}
