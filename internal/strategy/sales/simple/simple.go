package simplebuy

import (
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/orders"
)

type SalesSimple struct {
	Config          *Config
	OrderController *order.Controller
}

func NewSalesSimple(
	assetsPrices *prices.AsetsPrices,
) (*SalesSimple, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	str := &SalesSimple{
		Config: cfg,
	}
	return str, nil
}

func (s *SalesSimple) Execute(ms exModel.MarketsStat, orders []orders.StrategyOrder) {

}
