package database

// import (
// 	"testing"

// 	"github.com/sambly/exchangebot/internal/config"
// 	"github.com/sambly/exchangebot/internal/model"
// )

// var cfg = config.Database{
// 	Type:     "mysql",
// 	Name:     "datafeeder",
// 	Host:     "127.0.0.1",
// 	Port:     "3306",
// 	User:     "root",
// 	Password: "q1w2e3",
// }

// // func TestCreateOrder(t *testing.T) {

// // 	db, err := DbInit(cfg)
// // 	if err != nil {
// // 		t.Error(err)
// // 	}

// // 	order := &exModel.Order{
// // 		TimeCreated: time.Now(),
// // 		Time:        time.Now(),
// // 		Pair:        "BTCUSDT",
// // 		Side:        exModel.SideTypeBuy,
// // 		Type:        exModel.OrderTypeMarket,
// // 	}

// // 	err = CreateOrder(db, order)
// // 	if err != nil {
// // 		t.Error(err)
// // 	}

// // 	fmt.Println(order.ID)
// // }

// // func TestClosePosition(t *testing.T) {

// // 	db, err := DbInit(cfg)
// // 	if err != nil {
// // 		t.Error(err)
// // 	}

// // 	order := &exModel.Order{
// // 		TimeCreated: time.Now(),
// // 		Time:        time.Now(),
// // 		Status:      exModel.OrderStatusTypeClose,
// // 		Profit:      1000,
// // 	}

// // 	id := 1
// // 	err = ClosePosition(db, order, int64(id))
// // 	if err != nil {
// // 		t.Error(err)
// // 	}
// // }

// func TestCreateOrderInfo(t *testing.T) {

// 	db, err := DbInit(cfg)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	orderInfo := model.OrderInfo{
// 		IdOrder: 1,
// 	}

// 	err = InsertOrdersInfoTable(db, orderInfo)
// 	if err != nil {
// 		t.Error(err)
// 	}
// }
