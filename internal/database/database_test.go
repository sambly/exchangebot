package database

import (
	"testing"
	"time"

	"github.com/sambly/exchangebot/internal/config"

	exModel "github.com/sambly/exchangeService/pkg/model"
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
