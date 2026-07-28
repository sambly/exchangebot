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

	// Updates(struct) молча пропускает поля с zero-value (пустая строка, 0,
	// нулевое время) - не попадают в SQL вообще. Один совпавший с 0 Profit
	// или будущий вызов с пустым ExitReason тихо не запишутся в БД, при этом
	// в памяти и в ответе API будет казаться, что всё сохранилось. Явная
	// map с колонками гарантирует запись независимо от значений.
	result := r.db.Model(&order.Order{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"time":          updateData.Time,
			"status":        updateData.Status,
			"price":         updateData.Price,
			"profit":        updateData.Profit,
			"strategy_sell": updateData.StrategySell,
			"exit_reason":   updateData.ExitReason,
		})
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

// ReducePosition персистит частичное закрытие: Quantity и RealizedProfit,
// без смены Status - позиция остаётся активной. Та же защита от молчаливого
// пропуска zero-value через Updates(struct), что и у ClosePosition: явная
// map с колонками.
func (r *OrderDb) ReducePosition(id int64, updateData *order.Order) error {
	start := time.Now()

	result := r.db.Model(&order.Order{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"time":            updateData.Time,
			"quantity":        updateData.Quantity,
			"realized_profit": updateData.RealizedProfit,
		})
	duration := time.Since(start).Seconds()

	status := "success"
	if result.Error != nil {
		status = "error"
	}
	if result.RowsAffected == 0 && result.Error == nil {
		status = "not_found"
	}

	dbOperationDuration.WithLabelValues("reduce_position", status).Observe(duration)
	dbOperationTotal.WithLabelValues("reduce_position", status).Inc()

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("order with id %d not found", id)
	}

	return nil
}

func (r *OrderDb) ClearSalePolicyForActiveOrders() error {
	start := time.Now()
	err := r.db.Model(&order.Order{}).
		Where("status = ?", order.OrderStatusTypeActive).
		Update("strategy_sell", "").Error
	duration := time.Since(start).Seconds()

	status := "success"
	if err != nil {
		status = "error"
	}

	dbOperationDuration.WithLabelValues("clear_sale_policy_for_active_orders", status).Observe(duration)
	dbOperationTotal.WithLabelValues("clear_sale_policy_for_active_orders", status).Inc()

	return err
}

func (r *OrderDb) Delete(id int64) error {
	start := time.Now()

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id_order = ?", id).Delete(&order.OrderInfo{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&order.Order{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("order with id %d not found", id)
		}
		return nil
	})

	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
	}

	dbOperationDuration.WithLabelValues("delete", status).Observe(duration)
	dbOperationTotal.WithLabelValues("delete", status).Inc()

	return err
}

func (r *OrderDb) DeleteAllHistory() error {
	start := time.Now()

	err := r.db.Transaction(func(tx *gorm.DB) error {
		historyIDs := tx.Model(&order.Order{}).Select("id").Where("status = ?", order.OrderStatusTypeClose)
		if err := tx.Where("id_order IN (?)", historyIDs).Delete(&order.OrderInfo{}).Error; err != nil {
			return err
		}
		return tx.Where("status = ?", order.OrderStatusTypeClose).Delete(&order.Order{}).Error
	})

	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
	}

	dbOperationDuration.WithLabelValues("delete_all_history", status).Observe(duration)
	dbOperationTotal.WithLabelValues("delete_all_history", status).Inc()

	return err
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
