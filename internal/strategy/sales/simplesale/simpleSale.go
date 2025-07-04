package simplesale

import (
	"fmt"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/order"
)

type StrategySimpleSale struct {
	Config          *Config
	OrderController *order.OrderService
}

func NewStrategy(
	orderController *order.OrderService,
) (*StrategySimpleSale, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	str := &StrategySimpleSale{
		Config:          cfg,
		OrderController: orderController,
	}
	return str, nil
}

func (str *StrategySimpleSale) Execute(ms exModel.MarketsStat, o order.Order) (result bool) {
	if (ms.Price/o.PriceCreated)*100-100 >= float64(str.Config.Procent) {
		deal := order.Deal{Strategy: str.Config.IDName}
		if err := str.OrderController.ClosePosition(o.ID, deal); err != nil {
			fmt.Printf("error XXXXXXX - %v", err)
			return false
		}
		return true
	}
	return false
}
