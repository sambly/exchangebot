package database

import (
	"fmt"
	"log"
	"os"

	"github.com/glebarez/sqlite"
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/config"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/order"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	loggerGorm "gorm.io/gorm/logger"
)

var (
	ordersTable       string   = "orders"
	ordersInfoTable   string   = "order_infos"
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

	if err := db.Table(ordersTable).AutoMigrate(&order.Order{}); err != nil {
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
