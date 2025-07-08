package database

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sambly/exchangebot/internal/order"
	"gorm.io/gorm"
)

// Prometheus metrics for database operations
var (
	dbOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "database_operation_duration_seconds",
		Help:    "Duration of database operations in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation", "status"})

	dbOperationTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "database_operations_total",
		Help: "Total number of database operations",
	}, []string{"operation", "status"})
)

type OrderDb struct {
	db *gorm.DB
}

func NewOrderDb(db *gorm.DB) *OrderDb {
	return &OrderDb{db: db}
}

func (r *OrderDb) GetAll() ([]*order.Order, error) {
	start := time.Now()
	var orders []*order.Order
	err := r.db.Find(&orders).Error
	duration := time.Since(start).Seconds()

	status := "success"
	if err != nil {
		status = "error"
	}

	dbOperationDuration.WithLabelValues("get_all", status).Observe(duration)
	dbOperationTotal.WithLabelValues("get_all", status).Inc()

	if err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *OrderDb) Create(o *order.Order) error {
	start := time.Now()
	err := r.db.Create(o).Error
	duration := time.Since(start).Seconds()

	status := "success"
	if err != nil {
		status = "error"
	}

	dbOperationDuration.WithLabelValues("create", status).Observe(duration)
	dbOperationTotal.WithLabelValues("create", status).Inc()

	return err
}

func (r *OrderDb) ClosePosition(id int64, updateData *order.Order) error {
	start := time.Now()
	result := r.db.Model(&order.Order{}).
		Where("id = ?", id).
		Updates(updateData)
	duration := time.Since(start).Seconds()

	status := "success"
	if result.Error != nil {
		status = "error"
	}
	if result.RowsAffected == 0 && result.Error == nil {
		status = "not_found"
	}

	dbOperationDuration.WithLabelValues("close_position", status).Observe(duration)
	dbOperationTotal.WithLabelValues("close_position", status).Inc()

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("order with id %d not found", id)
	}

	return nil
}

func (r *OrderDb) CreateInfo(ordersInfo *order.OrderInfo) error {
	start := time.Now()
	err := r.db.Create(&ordersInfo).Error
	duration := time.Since(start).Seconds()

	status := "success"
	if err != nil {
		status = "error"
	}

	dbOperationDuration.WithLabelValues("create_info", status).Observe(duration)
	dbOperationTotal.WithLabelValues("create_info", status).Inc()

	return err
}
