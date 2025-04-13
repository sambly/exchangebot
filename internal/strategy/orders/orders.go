package orders

import (
	exModel "github.com/sambly/exchangeService/pkg/model"
)

type StrategyOrder struct {
	ID    int64
	Order exModel.Order
}
