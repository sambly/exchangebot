package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/sambly/exchangebot/internal/config"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestSelectCandlesTable(t *testing.T) {

	config, err := config.NewConfig()
	if err != nil {
		t.Error(err)
	}

	db, err := DbConnection(config.NameDb, config.HostDb, config.PortDb, config.UserDb, config.PasswordDb)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	_, err = SelectCandlesTable(db)
	if err != nil {
		t.Error(err)
	}

}

func TestCreateOrder(t *testing.T) {
	config, err := config.NewConfig()
	if err != nil {
		t.Error(err)
	}

	db, err := DbConnection(config.NameDb, config.HostDb, config.PortDb, config.UserDb, config.PasswordDb)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	err = CreateOrdersTable(db)
	if err != nil {
		t.Error(err)
	}

	var time time.Time
	order := &exModel.Order{
		TimeCreated: time,
		Time:        time,
		Pair:        "BTCUSDT",
	}

	id, err := CreateOrder(db, order)
	if err != nil {
		t.Error(err)
	}

	err = ClosePosition(db, order, id)
	if err != nil {
		t.Error(err)
	}
}

func TestSelectDeltaPeriod(t *testing.T) {
	config, err := config.NewConfig()
	if err != nil {
		t.Error(err)
	}

	db, err := DbConnection(config.NameDb, config.HostDb, config.PortDb, config.UserDb, config.PasswordDb)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	_, err = SelectDeltaPeriod(db, "BTCUSDT", "1m")
	if err != nil {
		t.Error(err)
	}

}

func TestORM(t *testing.T) {

	ds := dsn("datafeeder", "127.0.0.1", "3306", "root", "q1w2e3")
	db, err := gorm.Open(mysql.Open(ds), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	periods := []periods{
		{Name: "ch1m", Duration: time.Second * 60},
		{Name: "ch3m", Duration: time.Minute * 3},
		{Name: "ch15m", Duration: time.Minute * 15},
		{Name: "ch1h", Duration: time.Hour},
		{Name: "ch4h", Duration: time.Hour * 4},
		{Name: "ch12h", Duration: time.Hour * 12},
	}

	for _, period := range periods {
		db.Table(fmt.Sprintf("candles_%s", period.Name)).AutoMigrate(&Candle{})
	}

	candle := Candle{Pair: "BTCUSDT"}

	result := db.Table(fmt.Sprintf("candles_%s", "ch1m")).Create(&candle)
	if result.Error != nil {
		fmt.Println("ОШИБКА ЕПТ")
	}

}
