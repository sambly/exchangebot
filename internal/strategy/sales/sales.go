package sales

import (
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/strategy/orders"
)

type Sales interface {
	Execute(ms exModel.MarketsStat, orders []orders.StrategyOrder)
}
