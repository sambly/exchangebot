// Package signal - общий язык между детекторами и исполнителями.
//
// Детекторы (anomaly, base) находят события и публикуют сигналы. Исполнитель
// (simpleBuy) подписан на сигналы и ничего не знает о том, кто их породил.
// Благодаря этому новый детектор не тащит за собой копию торговой логики, а
// новая торговая логика не переписывает детекторы.
package signal

import "time"

type Direction string

const (
	// DirectionUp - цена аномально выросла (игра на продолжение движения)
	DirectionUp Direction = "UP"
	// DirectionDown - цена аномально упала (игра на отскок)
	DirectionDown Direction = "DOWN"
)

type Signal struct {
	Source string // кто нашёл: "anomaly", "base"
	Pair   string
	Period string
	Time   time.Time

	Direction Direction
	// Level - сила сигнала: у anomaly это уровень 1..3, у base всегда 1
	Level int
	// Strength - z-score (anomaly) со знаком
	Strength float64
	// ChangePercent - изменение цены за период, %
	ChangePercent float64

	// Volatility - ТИПИЧНОЕ движение цены этой пары за этот период, %.
	// Это робастная оценка разброса (та же, по которой anomaly считает z).
	//
	// Нужна, чтобы пороги выхода не были одинаковыми для BTC и мемкоина:
	// 1% для BTC - событие, для мемкоина - шум. Ноль означает "неизвестно",
	// тогда политика выхода откатывается на проценты из конфига.
	Volatility float64

	// Reason - человекочитаемое объяснение, попадает в комментарий к ордеру
	Reason string
}
