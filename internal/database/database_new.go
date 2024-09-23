package database

import (
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Candle struct {
	ID                   uint       `gorm:"primarykey;autoIncrement"`
	Time                 *time.Time `gorm:"column:time"`
	Pair                 string     `gorm:"column:pair;size:20;index"`
	Open                 float64    `gorm:"column:open"`
	Close                float64    `gorm:"column:close"`
	Low                  float64    `gorm:"column:low"`
	High                 float64    `gorm:"column:high"`
	Volume               float64    `gorm:"column:volume"`
	QuoteVolume          float64    `gorm:"column:quote_volume"`
	AmountTrade          int        `gorm:"column:amount_trade"`
	AmountTradeBuy       int        `gorm:"column:amount_trade_buy"`
	ActiveBuyVolume      float64    `gorm:"column:active_buy_volume"`
	ActiveBuyQuoteVolume float64    `gorm:"column:active_buy_quote_volume"`

	StartT bool    `gorm:"-"`
	Price  float64 `gorm:"-"`

	UpdatedAt            time.Time `gorm:"-"`
	AmountTradeAsk       int       `gorm:"-"`
	ActiveAskVolume      float64   `gorm:"-"`
	ActiveAskQuoteVolume float64   `gorm:"-"`

	Complete      bool `gorm:"-"`
	CompleteTrade bool `gorm:"-"`

	// Aditional collums from CSV inputs
	Metadata map[string]float64 `gorm:"-"`
}

type SideType string
type OrderType string
type OrderStatusType string

type Order struct {
	ID           uint            `gorm:"primarykey;autoIncrement"`
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

type OrderInfo struct {
	ID           uint   `gorm:"primarykey;autoIncrement"`
	IdOrder      uint   `gorm:"column:id_order"`
	Frame        string `gorm:"column:frame"`
	Strategy     string `gorm:"column:strategy"`
	Comment      string `gorm:"column:comment"`
	marketsStat  exModel.MarketsStat
	changePrices exModel.ChangePrices
	deltaFast    exModel.ChangeDelta
}

func DbInit1(dbname, hostname, port, username, password string) (*gorm.DB, error) {
	ds := dsn(dbname, hostname, port, username, password)
	db, err := gorm.Open(mysql.Open(ds), &gorm.Config{})
	if err != nil {
		return db, err
	}

	db.Table("orders").AutoMigrate(&Order{})

	return db, nil

}
