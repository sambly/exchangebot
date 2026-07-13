package prices

import "time"

// timed - значение, у которого есть время. Окно опирается на него, чтобы
// отличать свежие значения от повторов и устаревших.
type timed interface {
	at() time.Time
}

func (d DatasetChangePrices) at() time.Time { return d.Time }
func (c ChangeDelta) at() time.Time         { return c.Time }

// window - скользящее окно фиксированного размера, хранящее значения
// ОТ НОВЫХ К СТАРЫМ (items[0] - самое свежее).
//
// Этот порядок - не деталь реализации, а контракт: на нём держится вся
// арифметика. LastPrice берётся с хвоста (цена "period минут назад"), а дельта
// делит окно пополам по индексу, считая первую половину свежей.
//
// Раньше окно было написано четырежды (init/update x цены/дельты), копии
// разошлись, и в двух из них донабор добавлял свежее значение в КОНЕЦ - туда,
// где лежат самые старые. Отсюда росли оба бага с перепутанным порядком.
type window[T timed] struct {
	items []T
	size  int
}

func newWindow[T timed](size int) window[T] {
	return window[T]{
		items: make([]T, 0, size),
		size:  size,
	}
}

// filled - окно набрало нужное количество значений и готово к расчётам
func (w *window[T]) filled() bool {
	return w.size > 0 && len(w.items) >= w.size
}

func (w *window[T]) values() []T {
	return w.items
}

// newest возвращает самое свежее значение окна
func (w *window[T]) newest() (T, bool) {
	var zero T
	if len(w.items) == 0 {
		return zero, false
	}
	return w.items[0], true
}

// oldest возвращает самое старое значение окна - то есть то, что было
// "размер окна" назад. Именно с ним сравнивают текущее состояние.
func (w *window[T]) oldest() (T, bool) {
	var zero T
	if len(w.items) == 0 {
		return zero, false
	}
	return w.items[len(w.items)-1], true
}

// pushNewest вставляет свежее значение в голову, вытесняя самое старое.
//
// Возвращает false, если значение не новее головы: запросы к БД берут время с
// запасом, поэтому одна и та же свеча приходит в нескольких выборках подряд.
// Без этой проверки дубликат занимал бы место в окне, и оно покрывало бы меньше
// реального времени, чем должно.
func (w *window[T]) pushNewest(item T) bool {
	if head, ok := w.newest(); ok && !item.at().After(head.at()) {
		return false
	}

	w.items = append(w.items, item)
	copy(w.items[1:], w.items[:len(w.items)-1])
	w.items[0] = item

	if len(w.items) > w.size {
		w.items = w.items[:w.size]
	}
	return true
}

// pushOlder дописывает значение в хвост - к самым старым.
//
// Нужен для прогрева из БД: свечи приходят от новых к старым, то есть каждая
// следующая старше предыдущей. Донабирать окно можно только пока оно не полное.
func (w *window[T]) pushOlder(item T) bool {
	if w.filled() {
		return false
	}
	if tail, ok := w.oldest(); ok && !item.at().Before(tail.at()) {
		return false
	}

	w.items = append(w.items, item)
	return true
}
