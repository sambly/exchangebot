package entrysetup

import (
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/depth"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
)

// levels строит n уровней с шагом step от price0, все с одинаковым объёмом
// baseQty, кроме index wallIdx - там объём wallQty (стена).
func levels(price0, step float64, n int, baseQty float64, wallIdx int, wallQty float64) []exModel.DepthLevel {
	out := make([]exModel.DepthLevel, 0, n)
	for i := 0; i < n; i++ {
		qty := baseQty
		if i == wallIdx {
			qty = wallQty
		}
		out = append(out, level(price0+step*float64(i), qty))
	}
	return out
}

// Стена сверху (resistance) значительно дальше стены снизу (support) -
// асимметрия должна давать сторону BUY (тесный стоп снизу, простор сверху) и
// Score > 1.
func TestGetQualityFavorsBuyWhenSupportCloser(t *testing.T) {
	// support: ближняя стена на индексе 1 (0.2% вниз от mid)
	bids := levels(100.0, -0.1, 20, 1.0, 1, 50.0)
	// resistance: ближняя стена на индексе 10 (~1% вверх от mid)
	asks := levels(100.1, 0.1, 20, 1.0, 10, 50.0)

	s := setupWithBook(t, "BTCUSDT", bids, asks)

	q, ok := s.GetQuality("BTCUSDT", "15m")
	if !ok {
		t.Fatal("GetQuality должен вернуть ok=true, когда обе стены найдены")
	}
	if q.Side != order.SideTypeBuy {
		t.Errorf("ожидался Side=BUY (стена снизу ближе), получено %v", q.Side)
	}
	if q.Score <= 1 {
		t.Errorf("ожидался Score > 1 (дальняя стена дальше ближней), получено %v", q.Score)
	}
	if q.TakeDistancePercent <= q.StopDistancePercent {
		t.Errorf("TakeDistancePercent должен быть больше StopDistancePercent: take=%v stop=%v",
			q.TakeDistancePercent, q.StopDistancePercent)
	}
}

// Зеркальный случай: стена сверху ближе - сторона SELL.
func TestGetQualityFavorsSellWhenResistanceCloser(t *testing.T) {
	bids := levels(100.0, -0.1, 20, 1.0, 10, 50.0) // support далеко
	asks := levels(100.1, 0.1, 20, 1.0, 1, 50.0)   // resistance близко

	s := setupWithBook(t, "BTCUSDT", bids, asks)

	q, ok := s.GetQuality("BTCUSDT", "15m")
	if !ok {
		t.Fatal("GetQuality должен вернуть ok=true, когда обе стены найдены")
	}
	if q.Side != order.SideTypeSell {
		t.Errorf("ожидался Side=SELL (стена сверху ближе), получено %v", q.Side)
	}
	if q.Score <= 1 {
		t.Errorf("ожидался Score > 1, получено %v", q.Score)
	}
}

// Если хотя бы одна стена не найдена (ровный стакан с одной из сторон),
// соотношение посчитать честно нельзя - GetQuality должен вернуть ok=false,
// а не подставлять произвольное значение вместо отсутствующей стены.
func TestGetQualityRequiresBothWalls(t *testing.T) {
	bids := levels(100.0, -0.1, 20, 1.0, 1, 50.0) // стена есть
	asks := levels(100.1, 0.1, 20, 1.0, 0, 1.0)   // ровный, стены нет

	s := setupWithBook(t, "BTCUSDT", bids, asks)

	if _, ok := s.GetQuality("BTCUSDT", "15m"); ok {
		t.Fatal("без одной из стен GetQuality должен вернуть ok=false")
	}
}

// emptyRepo отдаёт пустую историю по всем запросам - для тестов, которым
// не нужно сидирование волатильности, только сама структура AssetsPrices.
type emptyRepo struct{}

func (emptyRepo) SelectMarketStateTimev2(time.Time) ([]exModel.Candle, error) { return nil, nil }
func (emptyRepo) SelectDeltaPeriod(string, string) ([]model.ChangeDeltaForCandle, error) {
	return nil, nil
}
func (emptyRepo) SelectCandlesFromPeriod(string, time.Time) ([]exModel.Candle, error) {
	return nil, nil
}

// GetAllQuality должна отдать запись по каждой отслеживаемой паре (даже если
// стен нет - тогда с пустой картой периодов), и для пары со стенами - по
// каждому настроенному периоду.
func TestGetAllQualityCoversPairsAndPeriods(t *testing.T) {
	periods := map[string]time.Duration{"15m": 15 * time.Minute, "1h": time.Hour}
	assetsPrices, err := prices.NewAssetsPrices([]string{"BTCUSDT", "ETHUSDT"}, periods, periods, emptyRepo{})
	if err != nil {
		t.Fatalf("NewAssetsPrices: %v", err)
	}

	assetsDepth := depth.NewAssetsDepth([]string{"BTCUSDT", "ETHUSDT"})
	// BTCUSDT - стакан с двумя стенами
	assetsDepth.OnDepth(exModel.DepthUpdate{
		Pair: "BTCUSDT",
		Bids: levels(100.0, -0.1, 20, 1.0, 1, 50.0),
		Asks: levels(100.1, 0.1, 20, 1.0, 10, 50.0),
	})
	// ETHUSDT - ровный стакан, стен нет вообще (wallIdx=-1 не совпадёт ни с
	// одним индексом)
	assetsDepth.OnDepth(exModel.DepthUpdate{
		Pair: "ETHUSDT",
		Bids: levels(100.0, -0.1, 20, 1.0, -1, 0),
		Asks: levels(100.1, 0.1, 20, 1.0, -1, 0),
	})

	s := NewAssetsSetup(assetsPrices, assetsDepth)
	all := s.GetAllQuality()

	if len(all) != 2 {
		t.Fatalf("ожидались записи по обеим парам, получено %d", len(all))
	}

	btc, ok := all["BTCUSDT"]
	if !ok {
		t.Fatal("нет записи по BTCUSDT")
	}
	for _, period := range []string{"15m", "1h"} {
		q, ok := btc[period]
		if !ok {
			t.Errorf("нет Quality для BTCUSDT/%s", period)
			continue
		}
		if q.Score <= 1 {
			t.Errorf("BTCUSDT/%s: ожидался Score > 1, получено %v", period, q.Score)
		}
	}

	eth, ok := all["ETHUSDT"]
	if !ok {
		t.Fatal("нет записи по ETHUSDT")
	}
	if len(eth) != 0 {
		t.Errorf("у ETHUSDT (ровный стакан) не должно быть Quality ни по одному периоду, получено %v", eth)
	}
}
