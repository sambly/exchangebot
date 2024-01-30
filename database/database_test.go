package database

import (
	"main/model"
	"testing"
	"time"
)

func TestSelectCandlesTable(t *testing.T) {

	db, err := DbConnection()
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
	db, err := DbConnection()
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	err = CreateOrdersTable(db)
	if err != nil {
		t.Error(err)
	}

	var time time.Time
	order := &model.Order{
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
