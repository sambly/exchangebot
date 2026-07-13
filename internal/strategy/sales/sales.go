// Package sales - политики выхода из позиции.
//
// Исполнитель (simpleBuy) знает, КОГДА войти. Политика выхода знает, КОГДА
// выйти, и ничего не знает о том, почему вошли - ей достаточно самой позиции.
package sales

import (
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/strategy/signal"
)

// Position - открытая позиция вместе с планом выхода, посчитанным на входе.
//
// План считается один раз при открытии, а не на каждом тике: пороги привязаны к
// волатильности пары НА МОМЕНТ СИГНАЛА, и пересчитывать их по ходу движения
// значило бы двигать стоп за ценой в обе стороны.
type Position struct {
	Order  order.Order
	Signal signal.Signal

	// TakeProfitPercent / StopLossPercent - в процентах от цены входа.
	// StopLossPercent хранится положительным числом (глубина падения).
	TakeProfitPercent float64
	StopLossPercent   float64

	// Deadline - момент, после которого позиция закрывается независимо от цены:
	// сигнал протух, и держать позицию дальше не за что.
	Deadline time.Time
}

// ExitReason - почему закрыли. Нужен, чтобы потом отличать по логам и по БД
// "сработал план" от "выбило стопом".
type ExitReason string

const (
	ExitTakeProfit ExitReason = "take-profit"
	ExitStopLoss   ExitReason = "stop-loss"
	ExitTimeout    ExitReason = "timeout"
)

type Sales interface {
	// Plan считает план выхода для сигнала: сколько брать и где резать.
	Plan(sig signal.Signal) (takeProfit, stopLoss float64, hold time.Duration)

	// Execute проверяет позицию на текущей цене. Возвращает true, если позиция
	// закрыта.
	Execute(ms exModel.MarketsStat, position Position) (closed bool)
}
