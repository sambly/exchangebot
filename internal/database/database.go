package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	loggerGorm "gorm.io/gorm/logger"
)

var (
	ordersTable       string   = "orders"
	ordersInfoTable   string   = "orders_info"
	candlesTables     string   = "candles_" // + periods
	candlesTablesList []string = []string{"1m", "3m", "15m", "1h", "4h", "1d"}
	basePeriod        string   = "1m"
)

func dsn(dbname, hostname, port, username, password string) string {
	loc := `&loc=Local`
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&%s", username, password, hostname, port, dbname, loc)
}

func DbInit(cfg config.Database) (*gorm.DB, error) {

	dbType := cfg.Type
	dbname := cfg.Name
	hostname := cfg.Host
	port := cfg.Port
	username := cfg.User
	password := cfg.Password

	var db *gorm.DB
	var err error
	var ds string

	// Настройка логирования
	logConfig := loggerGorm.Config{
		LogLevel: loggerGorm.Silent,
	}

	if dbType == "mysql" {
		ds = dsn(dbname, hostname, port, username, password)
		db, err = gorm.Open(mysql.Open(ds), &gorm.Config{
			Logger: loggerGorm.New(
				log.New(os.Stdout, "\r\n", log.LstdFlags),
				logConfig,
			),
		})
	} else if dbType == "sqlite" {
		ds = ":memory:" // Для использования SQLite в памяти
		db, err = gorm.Open(sqlite.Open(ds), &gorm.Config{
			Logger: loggerGorm.New(
				log.New(os.Stdout, "\r\n", log.LstdFlags),
				logConfig,
			),
		})
	}

	if err != nil {
		return db, err
	}

	if err := db.Table(ordersTable).AutoMigrate(&exModel.Order{}); err != nil {
		return db, err
	}
	if err := db.Table(ordersInfoTable).AutoMigrate(&model.OrderInfo{}); err != nil {
		return db, err
	}

	for _, tableName := range candlesTablesList {
		if err := db.Table(fmt.Sprintf("%s%s", candlesTables, tableName)).AutoMigrate(&exModel.Candle{}); err != nil {
			return db, err
		}
	}

	return db, nil
}

func InsertCandle(db *gorm.DB, candle exModel.Candle, period string) error {
	result := db.Table(fmt.Sprintf("%s%s", candlesTables, period)).Create(&candle)
	return result.Error
}

func InsertCandles(db *gorm.DB, candles []exModel.Candle, period string) error {
	if len(candles) == 0 {
		return nil
	}
	result := db.Table(fmt.Sprintf("%s%s", candlesTables, period)).Create(&candles)
	return result.Error
}

func GetCandlesByPeriod(db *gorm.DB, period string) ([]exModel.Candle, error) {

	var candles []exModel.Candle
	result := db.Table(fmt.Sprintf("%s%s", candlesTables, period)).Find(&candles)
	if result.Error != nil {
		return nil, result.Error
	}

	for i := range candles {
		candles[i].AmountTradeAsk = candles[i].AmountTrade - candles[i].AmountTradeBuy
		candles[i].ActiveAskVolume = candles[i].Volume - candles[i].ActiveBuyVolume

	}
	return candles, nil
}

func Orders(db *gorm.DB) ([]exModel.Order, error) {

	var orders []exModel.Order
	result := db.Table(ordersTable).Find(&orders)
	if result.Error != nil {
		return nil, result.Error
	}
	return orders, nil
}

func CreateOrder(db *gorm.DB, order *exModel.Order) error {

	result := db.Table(ordersTable).Create(order)
	return result.Error
}

func ClosePosition(db *gorm.DB, order *exModel.Order, id int64) error {

	result := db.Model(&exModel.Order{}).Where("id = ?", id).Updates(order)

	if result.Error != nil {
		return result.Error
	}
	return nil
}

func InsertOrdersInfoTable(db *gorm.DB, orderInfo model.OrderInfo) error {

	result := db.Table(ordersInfoTable).Create(&orderInfo)
	return result.Error
}

func SelectMarketStateTimev2(db *gorm.DB, timeRounding time.Time) ([]exModel.Candle, error) {

	var candles []exModel.Candle
	err := db.Table(fmt.Sprintf("%s%s", candlesTables, basePeriod)).
		Where("time >= ?", timeRounding).
		Order("time DESC").
		Find(&candles).Error

	if err != nil {
		return nil, err
	}

	for i := range candles {
		candles[i].AmountTradeAsk = candles[i].AmountTrade - candles[i].AmountTradeBuy
		candles[i].ActiveAskVolume = candles[i].Volume - candles[i].ActiveBuyVolume
	}

	return candles, nil
}

func SelectDeltaPeriod(db *gorm.DB, pair string, period string) ([]model.ChangeDeltaForCandle, error) {
	var candles []model.ChangeDeltaForCandle

	err := db.Table(fmt.Sprintf("%s%s", candlesTables, period)).
		Where("pair = ?", pair).
		Find(&candles).Error

	if err != nil {
		return nil, err
	}

	for i := range candles {
		candles[i].TradesAsk = candles[i].Trades - candles[i].TradesBuy
		candles[i].VolumeAsk = candles[i].Volume - candles[i].VolumeBuy
	}

	return candles, nil
}
