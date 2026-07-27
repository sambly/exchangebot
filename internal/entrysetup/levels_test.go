package entrysetup

import (
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/prices"
)

// fakeCandleRepo отдаёт заранее заданный набор свечей на любой запрос
// SelectCandlesFromPeriod - для тестов, которым нужен полный контроль над
// историей без БД. calls считает обращения - GetAllPriceLevels обязан делать
// один запрос НА ПЕРИОД, а не на пару.
type fakeCandleRepo struct {
	candles []exModel.Candle
	calls   *int
}

func (r fakeCandleRepo) SelectMarketStateTimev2(time.Time) ([]exModel.Candle, error) { return nil, nil }
func (r fakeCandleRepo) SelectDeltaPeriod(string, string) ([]model.ChangeDeltaForCandle, error) {
	return nil, nil
}
func (r fakeCandleRepo) SelectCandlesFromPeriod(string, time.Time) ([]exModel.Candle, error) {
	if r.calls != nil {
		*r.calls++
	}
	return r.candles, nil
}

// flatCandles строит n свечей одной пары с шагом step и одинаковым
// High=Low=Close=base, кроме индексов из spikes (High) и dips (Low) - там
// подставляется заданное значение.
func flatCandles(pair string, from time.Time, step time.Duration, n int, base float64, spikes, dips map[int]float64) []exModel.Candle {
	out := make([]exModel.Candle, 0, n)
	for i := 0; i < n; i++ {
		c := exModel.Candle{Pair: pair, Time: from.Add(step * time.Duration(i)), Close: base, High: base, Low: base}
		if v, ok := spikes[i]; ok {
			c.High = v
		}
		if v, ok := dips[i]; ok {
			c.Low = v
		}
		out = append(out, c)
	}
	// Последняя свеча - "текущая цена": держим её на базовом уровне, между
	// support и resistance.
	out[n-1].Close = base
	return out
}

func newLevelsSetup(t *testing.T, pair string, candles []exModel.Candle) *AssetsSetup {
	t.Helper()

	periods := map[string]time.Duration{"15m": 15 * time.Minute}
	ap, err := prices.NewAssetsPrices([]string{pair}, periods, periods, fakeCandleRepo{candles: candles})
	if err != nil {
		t.Fatalf("NewAssetsPrices: %v", err)
	}
	return NewAssetsSetup(ap, nil)
}

// Среди нескольких swing high/low должны находиться БЛИЖАЙШИЕ к текущей цене,
// а не самые выраженные (дальний, но более резкий выброс, для стопа/тейка
// бесполезен) - тот же принцип, что и у стен в стакане.
func TestGetPriceLevelsFindsNearestSwingPoints(t *testing.T) {
	from := time.Now().Add(-101 * 15 * time.Minute)
	candles := flatCandles("BTCUSDT", from, 15*time.Minute, 101, 100,
		map[int]float64{20: 115, 40: 108}, // resistance: 40 (108) ближе, чем 20 (115)
		map[int]float64{60: 92, 80: 85},   // support: 60 (92) ближе, чем 80 (85)
	)

	s := newLevelsSetup(t, "BTCUSDT", candles)

	levels, ok := s.GetPriceLevels("BTCUSDT", "15m")
	if !ok {
		t.Fatal("GetPriceLevels должен найти хотя бы один уровень")
	}
	if !levels.HasResistance || levels.Resistance.Price != 108 {
		t.Fatalf("ожидалось ближайшее сопротивление 108, получено %+v (ok=%v)", levels.Resistance, levels.HasResistance)
	}
	if !levels.HasSupport || levels.Support.Price != 92 {
		t.Fatalf("ожидалась ближайшая поддержка 92, получено %+v (ok=%v)", levels.Support, levels.HasSupport)
	}
	if levels.Support.DistancePercent <= 0 || levels.Resistance.DistancePercent <= 0 {
		t.Errorf("дистанции должны быть положительными: support=%v resistance=%v",
			levels.Support.DistancePercent, levels.Resistance.DistancePercent)
	}
}

// Плоская история без единого выброса - уровней нет вообще, а не мусорное
// совпадение на шуме.
func TestGetPriceLevelsNoLevelsOnFlatHistory(t *testing.T) {
	from := time.Now().Add(-101 * 15 * time.Minute)
	candles := flatCandles("BTCUSDT", from, 15*time.Minute, 101, 100, nil, nil)

	s := newLevelsSetup(t, "BTCUSDT", candles)

	if _, ok := s.GetPriceLevels("BTCUSDT", "15m"); ok {
		t.Fatal("на полностью плоской истории уровней быть не должно")
	}
}

// Недостаточно свечей для окна фрактала (меньше 2*levelsSwingWindow+1) -
// честный отказ, а не паника или мусорный уровень.
func TestGetPriceLevelsNotEnoughCandles(t *testing.T) {
	from := time.Now().Add(-3 * 15 * time.Minute)
	candles := flatCandles("BTCUSDT", from, 15*time.Minute, 3, 100, nil, nil)

	s := newLevelsSetup(t, "BTCUSDT", candles)

	if _, ok := s.GetPriceLevels("BTCUSDT", "15m"); ok {
		t.Fatal("при нехватке свечей GetPriceLevels должен вернуть ok=false")
	}
}

// GetAllPriceLevels должна запрашивать свечи ОДИН РАЗ НА ПЕРИОД, а не на
// пару: SelectCandlesFromPeriod и так отдаёт свечи всех пар сразу, дёргать её
// в цикле по парам значило бы N раз тянуть из БД один и тот же набор.
func TestGetAllPriceLevelsQueriesDBOncePerPeriod(t *testing.T) {
	from := time.Now().Add(-101 * 15 * time.Minute)

	var candles []exModel.Candle
	candles = append(candles, flatCandles("BTCUSDT", from, 15*time.Minute, 101, 100,
		map[int]float64{40: 108}, map[int]float64{60: 92})...)
	candles = append(candles, flatCandles("ETHUSDT", from, 15*time.Minute, 101, 100, nil, nil)...)

	calls := 0
	periods := map[string]time.Duration{"15m": 15 * time.Minute}
	ap, err := prices.NewAssetsPrices([]string{"BTCUSDT", "ETHUSDT"}, periods, periods,
		fakeCandleRepo{candles: candles, calls: &calls})
	if err != nil {
		t.Fatalf("NewAssetsPrices: %v", err)
	}
	calls = 0 // сбрасываем счётчик после сидирования при конструировании

	s := NewAssetsSetup(ap, nil)
	all := s.GetAllPriceLevels()

	if calls != 1 {
		t.Fatalf("ожидался 1 запрос к БД (один период), получено %d", calls)
	}

	if len(all) != 2 {
		t.Fatalf("ожидались записи по обеим парам, получено %d", len(all))
	}
	btc, ok := all["BTCUSDT"]["15m"]
	if !ok {
		t.Fatal("нет уровней для BTCUSDT/15m")
	}
	if !btc.HasSupport || btc.Support.Price != 92 {
		t.Errorf("ожидалась поддержка 92, получено %+v", btc.Support)
	}
	if !btc.HasResistance || btc.Resistance.Price != 108 {
		t.Errorf("ожидалось сопротивление 108, получено %+v", btc.Resistance)
	}

	if eth, ok := all["ETHUSDT"]["15m"]; ok {
		t.Errorf("у ETHUSDT (плоская история) уровней быть не должно, получено %+v", eth)
	}
}
