package database

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sambly/exchangebot/internal/order"
	"gorm.io/gorm"
)

// newTestDB поднимает изолированный sqlite in-memory с миграцией только
// таблиц ordersTable/ordersInfoTable, без DbInit: тот же процесс мигрирует
// ещё и таблицы свечей на разделяемое ":memory:" соединение, из-за чего
// несколько тестов в одном пакете начинают конфликтовать за один и тот же
// индекс.
func newTestDB(t *testing.T) *OrderDb {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	if err := db.Table(ordersTable).AutoMigrate(&order.Order{}); err != nil {
		t.Fatalf("AutoMigrate orders: %v", err)
	}
	if err := db.Table(ordersInfoTable).AutoMigrate(&order.OrderInfo{}); err != nil {
		t.Fatalf("AutoMigrate order_infos: %v", err)
	}
	return NewOrderDb(db)
}

func countRows(t *testing.T, r *OrderDb, table string) int64 {
	t.Helper()

	var count int64
	if err := r.db.Table(table).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

// Delete должен убрать сделку и её OrderInfo из БД насовсем, а не только
// пометить закрытой.
func TestOrderDbDelete(t *testing.T) {
	r := newTestDB(t)

	o := &order.Order{TimeCreated: time.Now(), Time: time.Now(), Pair: "BTCUSDT", Status: order.OrderStatusTypeClose}
	if err := r.Create(o); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := r.CreateInfo(&order.OrderInfo{IdOrder: uint(o.ID), Strategy: "manual"}); err != nil {
		t.Fatalf("CreateInfo: %v", err)
	}

	if err := r.Delete(o.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if got := countRows(t, r, ordersTable); got != 0 {
		t.Fatalf("orders table: ожидалось 0 строк, получено %d", got)
	}
	if got := countRows(t, r, ordersInfoTable); got != 0 {
		t.Fatalf("order_infos table: ожидалось 0 строк, получено %d", got)
	}
}

// Delete несуществующего id - ошибка, ничего не удаляется по чужим строкам.
func TestOrderDbDeleteNotFound(t *testing.T) {
	r := newTestDB(t)

	if err := r.Delete(999); err == nil {
		t.Fatal("ожидалась ошибка при удалении несуществующего id")
	}
}

// DeleteAllHistory должен удалить только закрытые сделки (и их OrderInfo),
// оставив активные нетронутыми.
func TestOrderDbDeleteAllHistory(t *testing.T) {
	r := newTestDB(t)

	closedOrder := &order.Order{TimeCreated: time.Now(), Time: time.Now(), Pair: "BTCUSDT", Status: order.OrderStatusTypeClose}
	if err := r.Create(closedOrder); err != nil {
		t.Fatalf("Create closed: %v", err)
	}
	if err := r.CreateInfo(&order.OrderInfo{IdOrder: uint(closedOrder.ID), Strategy: "manual"}); err != nil {
		t.Fatalf("CreateInfo closed: %v", err)
	}

	activeOrder := &order.Order{TimeCreated: time.Now(), Time: time.Now(), Pair: "BTCUSDT", Status: order.OrderStatusTypeActive}
	if err := r.Create(activeOrder); err != nil {
		t.Fatalf("Create active: %v", err)
	}
	if err := r.CreateInfo(&order.OrderInfo{IdOrder: uint(activeOrder.ID), Strategy: "manual"}); err != nil {
		t.Fatalf("CreateInfo active: %v", err)
	}

	if err := r.DeleteAllHistory(); err != nil {
		t.Fatalf("DeleteAllHistory: %v", err)
	}

	orders, err := r.GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(orders) != 1 || orders[0].ID != activeOrder.ID {
		t.Fatalf("ожидался только активный ордер id=%d, получено: %+v", activeOrder.ID, orders)
	}

	if got := countRows(t, r, ordersInfoTable); got != 1 {
		t.Fatalf("order_infos table: ожидалась 1 строка (активного ордера), получено %d", got)
	}
}
