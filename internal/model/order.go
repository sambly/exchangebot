package model

import "time"

type SideType string
type OrderType string
type OrderStatusType string

var (
	SideTypeBuy  SideType = "BUY"
	SideTypeSell SideType = "SELL"

	OrderTypeLimit           OrderType = "LIMIT"
	OrderTypeMarket          OrderType = "MARKET"
	OrderTypeLimitMaker      OrderType = "LIMIT_MAKER"
	OrderTypeStopLoss        OrderType = "STOP_LOSS"
	OrderTypeStopLossLimit   OrderType = "STOP_LOSS_LIMIT"
	OrderTypeTakeProfit      OrderType = "TAKE_PROFIT"
	OrderTypeTakeProfitLimit OrderType = "TAKE_PROFIT_LIMIT"

	OrderStatusTypeNew    OrderStatusType = "NEW"
	OrderStatusTypeFilled OrderStatusType = "FILLED"
	OrderStatusTypeActive OrderStatusType = "ACTIVE"
	OrderStatusTypeClose  OrderStatusType = "Close"
)

type Order struct {
	ID           int64
	TimeCreated  time.Time
	Time         time.Time
	Pair         string
	Side         SideType
	Type         OrderType
	Status       OrderStatusType
	PriceCreated float64
	Price        float64
	Quantity     float64
	Profit       float64
}
