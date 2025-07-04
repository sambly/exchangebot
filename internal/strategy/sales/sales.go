package sales

import (
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/order"
)

type Sales interface {
	Execute(ms exModel.MarketsStat, order order.Order) (result bool)
}
