package entrysetup

import (
	"math"

	"github.com/sambly/exchangebot/internal/depth"
	"github.com/sambly/exchangebot/internal/stat"
)

const (
	// wallLevels - сколько уровней стакана с каждой стороны разбирать в поисках
	// стены. Больше - стена может найтись дальше от цены, но и вычислений
	// больше; меньше - есть риск не увидеть стену, которая чуть дальше топа.
	wallLevels = 50

	// wallZScore - во сколько раз объём уровня должен превышать типичный объём
	// уровня в этой части стакана (робастный z относительно медианы+MAD по тем
	// же wallLevels), чтобы считаться стеной. Умеренный порог: 3 сигмы - это
	// уже заметный выброс, но не настолько редкий, чтобы стены не находились
	// почти никогда.
	wallZScore = 3.0
)

// Wall - один уровень стакана, выделяющийся объёмом на фоне соседних.
type Wall struct {
	Price    float64
	Quantity float64
	// DistancePercent - расстояние от текущей цены (mid) до стены, всегда >=0.
	DistancePercent float64
	// ZScore - насколько объём уровня необычен относительно своей части
	// стакана (см. wallZScore).
	ZScore float64
}

// Walls - ближайшие стены по обе стороны от текущей цены пары.
//
// Это НЕ решение "куда ставить стоп/тейк" - это сырое наблюдение "здесь
// необычно много объёма". Support - потенциальный тесный стоп для лонга и
// потенциальный тейк для шорта; Resistance - наоборот. Что из этого выбрать -
// решает потребитель (стратегия, веб), сам показатель ничего не рекомендует.
type Walls struct {
	Pair string
	Mid  float64

	Support    Wall
	HasSupport bool

	Resistance    Wall
	HasResistance bool
}

// GetWalls ищет ближайшую к текущей цене стену снизу (в бидах) и сверху
// (в асках) стакана пары.
//
// "Ближайшая" - по порядку уровней от спреда наружу (см. depth.GetTopLevels):
// первый уровень, объём которого статистически необычен для своей стороны
// стакана, и есть стена. Дальние стены игнорируются намеренно: до них может
// не дойти вообще ни одна сделка, а для стопа/тейка важна именно ближайшая
// преграда.
func (s *AssetsSetup) GetWalls(pair string) (Walls, bool) {
	bid, ask, ok := s.depth.GetBestBidAsk(pair)
	if !ok {
		return Walls{}, false
	}

	bids, asks, ok := s.depth.GetTopLevels(pair, wallLevels)
	if !ok {
		return Walls{}, false
	}

	mid := (bid.Price + ask.Price) / 2

	walls := Walls{Pair: pair, Mid: mid}
	walls.Support, walls.HasSupport = findWall(bids, mid)
	walls.Resistance, walls.HasResistance = findWall(asks, mid)

	return walls, true
}

// findWall - первый (ближайший к цене) уровень среди levels, чей объём
// проходит порог wallZScore относительно медианы+MAD объёмов ЭТОГО ЖЕ среза.
// levels ожидаются уже отсортированными от цены наружу (как отдаёт
// depth.GetTopLevels), поэтому первое найденное совпадение - ближайшее.
func findWall(levels []depth.Level, mid float64) (Wall, bool) {
	if len(levels) == 0 {
		return Wall{}, false
	}

	quantities := make([]float64, len(levels))
	for i, l := range levels {
		quantities[i] = l.Quantity
	}
	center, scale := stat.MedianAndScale(quantities, 0)
	if scale == 0 {
		return Wall{}, false
	}

	for _, l := range levels {
		z := (l.Quantity - center) / scale
		if z < wallZScore {
			continue
		}
		if mid == 0 {
			continue
		}
		return Wall{
			Price:           l.Price,
			Quantity:        l.Quantity,
			DistancePercent: math.Abs(l.Price-mid) / mid * 100,
			ZScore:          z,
		}, true
	}

	return Wall{}, false
}
