package database

import (
	"fmt"

	"github.com/sambly/exchangebot/internal/order"
	"gorm.io/gorm"
)

type OrderDb struct {
	db *gorm.DB
}

func NewOrderDb(db *gorm.DB) *OrderDb {
	return &OrderDb{db: db}
}

func (r *OrderDb) GetAll() ([]*order.Order, error) {
	var orders []*order.Order
	if err := r.db.Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *OrderDb) Create(o *order.Order) error {
	return r.db.Create(o).Error
}

func (r *OrderDb) ClosePosition(id int64, updateData *order.Order) error {
	result := r.db.Model(&order.Order{}).
		Where("id = ?", id).
		Updates(updateData)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("order with id %d not found", id)
	}

	return nil
}

func (r *OrderDb) CreateInfo(ordersInfo *order.OrderInfo) error {
	return r.db.Create(&ordersInfo).Error
}
