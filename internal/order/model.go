package order

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
	ID           int64           `gorm:"primarykey;autoIncrement"`
	TimeCreated  time.Time       `gorm:"column:time_created"`
	Time         time.Time       `gorm:"column:time"`
	Pair         string          `gorm:"column:pair"`
	Side         SideType        `gorm:"column:side"`
	Type         OrderType       `gorm:"column:type"`
	Status       OrderStatusType `gorm:"column:status"`
	PriceCreated float64         `gorm:"column:price_created"`
	Price        float64         `gorm:"column:price"`
	Quantity     float64         `gorm:"column:quantity"`
	Profit       float64         `gorm:"column:profit"`
}
