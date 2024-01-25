package order

import (
	"context"
	"database/sql"
	"main/database"
	"main/exchange"
	"main/model"
	"sync"
	"time"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusError   Status = "error"
)

type Controller struct {
	mtx            sync.Mutex
	ctx            context.Context
	exchange       *exchange.PaperWallet
	database       *sql.DB
	tickerInterval time.Duration
	finish         chan bool
	status         Status
}

func NewController(ctx context.Context, ex *exchange.PaperWallet, db *sql.DB) (*Controller, error) {

	ctrl := &Controller{
		ctx:      ctx,
		exchange: ex,
		database: db,
	}

	orders, err := database.Orders(db)
	if err != nil {
		return nil, err
	}

	ex.Orders = orders

	return ctrl, nil

}

func (c *Controller) updateOrders() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

}

func (c *Controller) CreateOrderMarket(side model.SideType, pair string, size float64) (*model.Order, error) {
	// c.mtx.Lock()
	// defer c.mtx.Unlock()

	order, err := c.exchange.CreateOrderMarket(side, pair, size)
	if err != nil {
		return nil, err
	}

	id, err := database.CreateOrder(c.database, order)
	if err != nil {
		return nil, err
	}
	order.ID = id

	return order, err
}

func (c *Controller) ClosePosition(id int64) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	order, err := c.exchange.ClosePosition(id)
	if err != nil {
		return err
	}

	err = database.ClosePosition(c.database, order, id)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) Start() {
	if c.status != StatusRunning {
		c.status = StatusRunning
		go func() {
			ticker := time.NewTicker(c.tickerInterval)
			for {
				select {
				case <-ticker.C:
					c.updateOrders()
				case <-c.finish:
					ticker.Stop()
					return
				}
			}
		}()
	}
}
func (c *Controller) Stop() {
	if c.status == StatusRunning {
		c.status = StatusStopped
		c.updateOrders()
		c.finish <- true
	}
}
