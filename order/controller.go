package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"main/database"
	"main/exchange"
	"main/log"
	"main/model"
	"main/notification"
	"main/prices"
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

	ex.Orders = orders

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

	go func() {
		c.assetsPrices.UpdateDelta()

		mkStat := c.assetsPrices.MarketsStat[pair]
		chData := c.assetsPrices.ChangePrices[pair]
		dFast := c.assetsPrices.DeltaFast[pair]

		mkStatJson, err := json.Marshal(mkStat)
		if err != nil {
			log.MyLogger.ErrorOut(fmt.Errorf("error jsonmarshal mkStatJson : %v", err))
		}
		chDataJson, err := json.Marshal(chData)
		if err != nil {
			log.MyLogger.ErrorOut(fmt.Errorf("error jsonmarshal chDataJson : %v", err))
		}
		dFastJson, err := json.Marshal(dFast)
		if err != nil {
			log.MyLogger.ErrorOut(fmt.Errorf("error jsonmarshal dFastJson : %v", err))
		}

		err = database.InsertOrdersInfoTable(c.database, id, deal.Frame, deal.Strategy, deal.Comment, mkStatJson, chDataJson, dFastJson)
		if err != nil {
			log.MyLogger.ErrorOut(fmt.Errorf("error when create order and add insertinfotables : %v", err))
		}
	}()

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

	orders, err := database.Orders(c.database)
	if err != nil {
		return err
	}

	c.exchange.Orders = orders

	return nil
}

func (c *Controller) OnMarket(ms model.MarketsStat) {

	// Обновление ордеров
	for index, order := range c.exchange.Orders {
		if order.Status == model.OrderStatusTypeActive && order.Pair == ms.Pair {
			c.exchange.Orders[index].Price = ms.Price

			if order.Side == model.SideTypeBuy {
				c.exchange.Orders[index].Profit = (ms.Price / order.PriceCreated * 100) - 100
			}
			if order.Side == model.SideTypeSell {
				c.exchange.Orders[index].Profit = (order.PriceCreated / ms.Price * 100) - 100
			}

			// Обновления цены webSocket
			messageOrder, _ := json.Marshal(order)
			c.socketsMessage.SendData(messageOrder)
		}
	}

}
