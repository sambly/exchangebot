package databaseold

import (
	"testing"
	"time"

	"github.com/sambly/exchangebot/internal/config"

	exModel "github.com/sambly/exchangeService/pkg/model"
)

var cfg = config.Database{
	Type:     "mysql",
	Name:     "datafeeder",
	Host:     "127.0.0.1",
	Port:     "3306",
	User:     "root",
	Password: "q1w2e3",
}

func TestSelectCandlesTable(t *testing.T) {

	db, err := DbConnection(cfg.Name, cfg.Host, cfg.Port, cfg.User, cfg.Password)
	if err != nil {
		t.Error(err)
	}

	_, err = SelectCandlesTable(db)
	if err != nil {
		t.Error(err)
	}

}

func TestCreateOrder(t *testing.T) {

	db, err := DbConnection(cfg.Name, cfg.Host, cfg.Port, cfg.User, cfg.Password)
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

	db, err := DbConnection(cfg.Name, cfg.Host, cfg.Port, cfg.User, cfg.Password)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	_, err = SelectDeltaPeriod(db, "BTCUSDT", "1m")
	if err != nil {
		t.Error(err)
	}

}
