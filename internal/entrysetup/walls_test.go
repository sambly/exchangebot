package entrysetup

import (
	"testing"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/depth"
)

// setupWithBook строит AssetsSetup с одним заполненным стаканом пары -
// без прайсов (GetWalls их не использует), только depth.
func setupWithBook(t *testing.T, pair string, bids, asks []exModel.DepthLevel) *AssetsSetup {
	t.Helper()

	ad := depth.NewAssetsDepth([]string{pair})
	ad.OnDepth(exModel.DepthUpdate{Pair: pair, Bids: bids, Asks: asks})

	return NewAssetsSetup(nil, ad)
}

func level(price, qty float64) exModel.DepthLevel {
	return exModel.DepthLevel{Price: price, Quantity: qty}
}

// Ровный стакан без выбросов - стены находиться не должны: объявлять
// "стеной" средний по размеру уровень значило бы находить стену везде.
func TestGetWallsNoWallOnFlatBook(t *testing.T) {
	bids := make([]exModel.DepthLevel, 0, 10)
	asks := make([]exModel.DepthLevel, 0, 10)
	for i := 0; i < 10; i++ {
		bids = append(bids, level(100-float64(i)*0.1, 1.0))
		asks = append(asks, level(100.1+float64(i)*0.1, 1.0))
	}

	s := setupWithBook(t, "BTCUSDT", bids, asks)

	walls, ok := s.GetWalls("BTCUSDT")
	if !ok {
		t.Fatal("GetWalls должен вернуть ok=true для готового стакана")
	}
	if walls.HasSupport {
		t.Errorf("на ровном стакане не должно быть стены снизу, получено %+v", walls.Support)
	}
	if walls.HasResistance {
		t.Errorf("на ровном стакане не должно быть стены сверху, получено %+v", walls.Resistance)
	}
}

// Один уровень с объёмом на порядки больше соседних - это и есть стена, и она
// должна находиться ближайшей к цене (первой по ходу от спреда), а не любой
// подходящей по величине.
//
// Baseline из многих ровных уровней важен: с парой всего значений выброс сам
// раздувает запасной разброс (среднее абсолютное отклонение) настолько, что
// z-score не проходит порог - реалистичный стакан (wallLevels=50) этой
// проблемы не имеет.
func TestGetWallsFindsNearestOutlier(t *testing.T) {
	bids := make([]exModel.DepthLevel, 0, 20)
	for i := 0; i < 20; i++ {
		bids = append(bids, level(100-float64(i)*0.1, 1.0))
	}
	bids[2].Quantity = 50.0 // ближайшая стена, price 99.8
	bids[5].Quantity = 60.0 // более крупная, но дальше - не должна выбираться

	asks := make([]exModel.DepthLevel, 0, 20)
	for i := 0; i < 20; i++ {
		asks = append(asks, level(100.1+float64(i)*0.1, 1.0))
	}

	s := setupWithBook(t, "BTCUSDT", bids, asks)

	walls, ok := s.GetWalls("BTCUSDT")
	if !ok {
		t.Fatal("GetWalls должен вернуть ok=true")
	}
	if !walls.HasSupport {
		t.Fatal("ожидалась стена снизу")
	}
	if walls.Support.Price != 99.8 {
		t.Errorf("ожидалась ближайшая стена на 99.8, получено %.2f", walls.Support.Price)
	}
	if walls.Support.DistancePercent <= 0 {
		t.Errorf("расстояние до стены должно быть положительным, получено %v", walls.Support.DistancePercent)
	}
	if walls.HasResistance {
		t.Errorf("на этой стороне выбросов нет, стена не ожидалась: %+v", walls.Resistance)
	}
}

// Для пары без данных стакана (не готова / нет апдейтов) GetWalls должен
// честно отдавать ok=false, а не нулевые стены как будто они посчитаны.
func TestGetWallsNotReady(t *testing.T) {
	ad := depth.NewAssetsDepth([]string{"BTCUSDT"})
	s := NewAssetsSetup(nil, ad)

	if _, ok := s.GetWalls("BTCUSDT"); ok {
		t.Fatal("для неготового стакана GetWalls должен вернуть ok=false")
	}
}
