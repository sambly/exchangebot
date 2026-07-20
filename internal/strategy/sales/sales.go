// Package sales - политики выхода из позиции.
//
// Исполнитель (executor) знает, КОГДА войти. Политика выхода знает, КОГДА
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
	// Name - имя политики выхода (IDName из её конфига), например "salesimple".
	// Пишется в Order.StrategySell СРАЗУ при входе (см. Executor.addPosition) -
	// до того, как позиция реально закрылась, чтобы было видно, какая политика
	// её ведёт, а не только то, чем в итоге закрыли.
	Name() string

	// Plan считает план выхода для сигнала: сколько брать и где резать.
	Plan(sig signal.Signal) (takeProfit, stopLoss float64, hold time.Duration)

	// ShouldExit - ЧИСТОЕ решение: пора ли выходить при такой цене в такой момент.
	//
	// Отделено от Execute намеренно. Раньше политика выхода и решала, и сама
	// закрывала позицию через OrderController - то есть решение было неразрывно
	// сцеплено с побочным эффектом, и переиспользовать его не мог никто. Бэктестер
	// зовёт именно ShouldExit: ему нужно то же самое решение, но по исторической
	// цене и по симулированному времени, без похода в БД и на биржу.
	//
	// Время передаётся аргументом, а не берётся из time.Now(), по той же причине:
	// в бэктесте "сейчас" - это момент свечи, а не момент запуска.
	ShouldExit(price float64, at time.Time, position Position) (ExitReason, bool)

	// Execute проверяет позицию на текущей цене И закрывает её, если пора.
	// Возвращает true, если позиция закрыта.
	Execute(ms exModel.MarketsStat, position Position) (closed bool)
}
