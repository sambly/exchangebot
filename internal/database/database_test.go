package database

import (
	"fmt"
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/model"
)

func TestCreateOrder(t *testing.T) {

	db, err := DbInit("mysql", "datafeeder", "127.0.0.1", "3306", "root", "q1w2e3")
	if err != nil {
		t.Error(err)
	}

	order := &exModel.Order{
		TimeCreated: time.Now(),
		Time:        time.Now(),
		Pair:        "BTCUSDT",
		Side:        exModel.SideTypeBuy,
		Type:        exModel.OrderTypeMarket,
	}

	err = CreateOrder(db, order)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(order.ID)
}

func TestClosePosition(t *testing.T) {

	db, err := DbInit("mysql", "datafeeder", "127.0.0.1", "3306", "root", "q1w2e3")
	if err != nil {
		t.Error(err)
	}

	order := &exModel.Order{
		TimeCreated: time.Now(),
		Time:        time.Now(),
		Status:      exModel.OrderStatusTypeClose,
		Profit:      1000,
	}

	id := 1
	err = ClosePosition(db, order, int64(id))
	if err != nil {
		t.Error(err)
	}
}

func TestCreateOrderInfo(t *testing.T) {

	db, err := DbInit("mysql", "datafeeder", "127.0.0.1", "3306", "root", "q1w2e3")
	if err != nil {
		t.Error(err)
	}

	orderInfo := model.OrderInfo{
		IdOrder: 1,
	}

	err = InsertOrdersInfoTable(db, orderInfo)
	if err != nil {
		t.Error(err)
	}
}
