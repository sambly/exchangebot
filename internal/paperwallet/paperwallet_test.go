package paperwallet

import (
	"errors"
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
)

type stubRepo struct{}

func (stubRepo) SelectMarketStateTimev2(time.Time) ([]exModel.Candle, error) { return nil, nil }
func (stubRepo) SelectDeltaPeriod(string, string) ([]model.ChangeDeltaForCandle, error) {
	return nil, nil
}
func (stubRepo) SelectCandlesFromPeriod(string, time.Time) ([]exModel.Candle, error) {
	return nil, nil
}

func newTestWallet(t *testing.T, price float64) *PaperWallet {
	t.Helper()

	periods := map[string]time.Duration{"15m": 15 * time.Minute}
	ap, err := prices.NewAssetsPrices([]string{"BTCUSDT"}, periods, periods, stubRepo{})
	if err != nil {
		t.Fatalf("NewAssetsPrices: %v", err)
	}
	ap.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: price, Time: time.Now()})

	return NewPaperWallet(ap)
}

// Закрытие ордера, которого нет среди активных, обязано возвращать ошибку.
// Раньше здесь было (nil, nil), и вызывающий код разыменовывал nil - то есть
// повторный closeDeal по тому же id ронял процесс.
func TestClosePositionUnknownIDReturnsError(t *testing.T) {
	pw := newTestWallet(t, 100)

	got, err := pw.ClosePosition(999, order.Deal{Strategy: "manual"})
	if err == nil {
		t.Fatalf("ожидалась ошибка, получено: order=%v, err=nil", got)
	}
	if got != nil {
		t.Fatalf("при ошибке ордер должен быть nil, получено: %v", got)
	}
}

// Повторное закрытие того же ордера тоже должно давать ошибку, а не тихо
// закрывать позицию дважды.
func TestClosePositionTwice(t *testing.T) {
	pw := newTestWallet(t, 100)

	created, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 1, SideType: order.SideTypeBuy})
	if err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	if _, err := pw.ClosePosition(created.ID, order.Deal{Strategy: "manual"}); err != nil {
		t.Fatalf("первое закрытие должно проходить: %v", err)
	}
	if _, err := pw.ClosePosition(created.ID, order.Deal{Strategy: "manual"}); err == nil {
		t.Fatal("повторное закрытие должно возвращать ошибку")
	}
}

// Причина выхода должна сохраняться в ордере: без неё по БД не отличить
// "сработал тейк" от "выбило стопом", то есть нельзя оценить, работает ли
// стратегия. Раньше deal.Comment при закрытии просто выбрасывался.
func TestClosePositionSavesExitReason(t *testing.T) {
	pw := newTestWallet(t, 100)

	created, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 1, SideType: order.SideTypeBuy})
	if err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	closed, err := pw.ClosePosition(created.ID, order.Deal{
		Strategy:   "salesimple",
		ExitReason: "stop-loss",
	})
	if err != nil {
		t.Fatalf("ClosePosition: %v", err)
	}

	if closed.ExitReason != "stop-loss" {
		t.Fatalf("причина выхода = %q, ожидалось %q", closed.ExitReason, "stop-loss")
	}
	if closed.StrategySell != "salesimple" {
		t.Fatalf("стратегия выхода = %q, ожидалось %q", closed.StrategySell, "salesimple")
	}
}

// Пара, по которой ещё не приходило тиков: цена и время нулевые.
//
// Раньше такой ордер молча создавался с ценой 0 и датой 0000-00-00, падал уже в
// MySQL, а до интерфейса ошибка не доходила - сделка просто "не появлялась".
func TestCreateOrderWithoutMarketData(t *testing.T) {
	periods := map[string]time.Duration{"15m": 15 * time.Minute}
	ap, err := prices.NewAssetsPrices([]string{"BTCUSDT"}, periods, periods, stubRepo{})
	if err != nil {
		t.Fatalf("NewAssetsPrices: %v", err)
	}
	// OnMarket не вызываем: тиков по паре не было

	pw := NewPaperWallet(ap)

	got, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 1, SideType: order.SideTypeBuy})
	if !errors.Is(err, ErrNoMarketData) {
		t.Fatalf("ожидалась ErrNoMarketData, получено: order=%v, err=%v", got, err)
	}

	if orders := pw.GetActiveOrdersBySymbol("BTCUSDT"); len(orders) != 0 {
		t.Fatalf("ордер не должен был появиться в активных, их %d", len(orders))
	}
}

// Время создания - момент сделки по нашим часам, а не время события на бирже.
func TestCreateOrderSetsCreationTime(t *testing.T) {
	pw := newTestWallet(t, 100)

	created, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 1, SideType: order.SideTypeBuy})
	if err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	if created.TimeCreated.IsZero() {
		t.Fatal("время создания не должно быть нулевым: MySQL отвергает 0000-00-00")
	}
	if time.Since(created.TimeCreated) > time.Minute {
		t.Fatalf("время создания = %v, ожидалось текущее", created.TimeCreated)
	}
}

func TestCreateOrderZeroSize(t *testing.T) {
	pw := newTestWallet(t, 100)

	if _, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 0}); err != ErrInvalidQuantity {
		t.Fatalf("ожидалась ErrInvalidQuantity, получено: %v", err)
	}
}

// UpdateOrdersPrice считает профит и отдаёт КОПИИ: наружу не должны утекать
// указатели на ордера, которые живут под мьютексом кошелька.
func TestUpdateOrdersPrice(t *testing.T) {
	pw := newTestWallet(t, 100)

	if _, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 1, SideType: order.SideTypeBuy}); err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	updated := pw.UpdateOrdersPrice("BTCUSDT", 150)
	if len(updated) != 1 {
		t.Fatalf("ожидался 1 обновлённый ордер, получено %d", len(updated))
	}
	// Профит ЧИСТЫЙ: +50% по цене минус комиссии обеих сторон (2 * 0.1%)
	want := 50.0 - 2*FeePercent
	if updated[0].Profit < want-0.1 || updated[0].Profit > want+0.1 {
		t.Fatalf("профит = %+.2f%%, ожидалось %+.2f%% (гросс +50%% минус комиссии)", updated[0].Profit, want)
	}

	// Портим копию - оригинал не должен измениться
	updated[0].Price = 0

	if orders := pw.GetActiveOrdersBySymbol("BTCUSDT"); orders[0].Price != 150 {
		t.Fatalf("наружу утекла ссылка на внутренний ордер: цена стала %v", orders[0].Price)
	}

	// По паре без ордеров - пусто, без паники
	if got := pw.UpdateOrdersPrice("ETHUSDT", 100); got != nil {
		t.Fatalf("по паре без ордеров ожидался nil, получено %v", got)
	}
}

// Нулевая цена не должна давать Inf/NaN в профите.
func TestUpdateOrdersPriceZeroPrice(t *testing.T) {
	pw := newTestWallet(t, 100)

	if _, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 1, SideType: order.SideTypeBuy}); err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	updated := pw.UpdateOrdersPrice("BTCUSDT", 0)
	if len(updated) != 1 {
		t.Fatalf("ожидался 1 ордер, получено %d", len(updated))
	}
	if updated[0].Profit != 0 {
		t.Fatalf("при нулевой цене профит должен остаться 0, получено %v", updated[0].Profit)
	}
}

func TestCalculatePNL(t *testing.T) {
	pw := newTestWallet(t, 100)

	for i := 0; i < 2; i++ {
		if _, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 1, SideType: order.SideTypeBuy}); err != nil {
			t.Fatalf("CreateOrderMarket: %v", err)
		}
	}
	pw.UpdateOrdersPrice("BTCUSDT", 110)

	count, profit := pw.CalculatePNL()
	if count != 2 {
		t.Fatalf("активных ордеров %d, ожидалось 2", count)
	}
	// Два ордера по +10% гросс, у каждого минус комиссии обеих сторон
	want := 2 * (10.0 - 2*FeePercent)
	if profit < want-0.1 || profit > want+0.1 {
		t.Fatalf("суммарный профит %+.2f%%, ожидалось %+.2f%%", profit, want)
	}
}

// ReducePosition должен закрыть только часть позиции: списать долю профита в
// RealizedProfit, уменьшить Quantity и оставить ордер активным.
func TestReducePosition(t *testing.T) {
	pw := newTestWallet(t, 100)

	created, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 2.0, SideType: order.SideTypeBuy})
	if err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	pw.assetsPrices.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 110, Time: time.Now()}) // +10%

	reduced, err := pw.ReducePosition(created.ID, 0.5, order.Deal{Strategy: "structsale"})
	if err != nil {
		t.Fatalf("ReducePosition: %v", err)
	}

	if reduced.Quantity != 1.0 {
		t.Fatalf("Quantity: ожидалось 1.0 (половина от 2.0), получено %v", reduced.Quantity)
	}
	want := 0.5 * (10.0 - 2*FeePercent)
	if reduced.RealizedProfit < want-0.01 || reduced.RealizedProfit > want+0.01 {
		t.Fatalf("RealizedProfit: ожидалось ~%.3f, получено %v", want, reduced.RealizedProfit)
	}

	// Позиция должна остаться активной, а не переехать в историю.
	active := pw.GetActiveOrdersBySymbol("BTCUSDT")
	if len(active) != 1 {
		t.Fatalf("ожидалась 1 активная позиция после частичного закрытия, получено %d", len(active))
	}
}

// Финальное закрытие после частичного должно взвесить обе части в ОДИН
// Profit, а не потерять уже реализованную часть и не задвоить её.
func TestClosePositionAfterPartialReduce(t *testing.T) {
	pw := newTestWallet(t, 100)

	created, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 2.0, SideType: order.SideTypeBuy})
	if err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	// Половина закрывается по +10%.
	pw.assetsPrices.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 110, Time: time.Now()})
	if _, err := pw.ReducePosition(created.ID, 0.5, order.Deal{Strategy: "structsale"}); err != nil {
		t.Fatalf("ReducePosition: %v", err)
	}

	// Остаток закрывается по +20%.
	pw.assetsPrices.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 120, Time: time.Now()})
	closed, err := pw.ClosePosition(created.ID, order.Deal{Strategy: "structsale", ExitReason: "take-profit"})
	if err != nil {
		t.Fatalf("ClosePosition: %v", err)
	}

	// Взвешенно: 0.5*(10%-fee) + 0.5*(20%-fee) = 0.5*9.8 + 0.5*19.8 = 14.8
	want := 0.5*(10.0-2*FeePercent) + 0.5*(20.0-2*FeePercent)
	if closed.Profit < want-0.01 || closed.Profit > want+0.01 {
		t.Fatalf("Profit: ожидалось ~%.3f (взвешенно 50%%@10%% + 50%%@20%%), получено %v", want, closed.Profit)
	}

	if active := pw.GetActiveOrdersBySymbol("BTCUSDT"); len(active) != 0 {
		t.Fatalf("после полного закрытия активных позиций быть не должно, получено %d", len(active))
	}
}

// Без частичных закрытий ClosePosition должен вести себя ровно так же, как
// до появления этой фичи - RealizedProfit=0, вся позиция закрывается по
// текущему профиту.
func TestClosePositionWithoutPartialUnaffected(t *testing.T) {
	pw := newTestWallet(t, 100)

	created, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 1, SideType: order.SideTypeBuy})
	if err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	pw.assetsPrices.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 150, Time: time.Now()})
	closed, err := pw.ClosePosition(created.ID, order.Deal{Strategy: "test"})
	if err != nil {
		t.Fatalf("ClosePosition: %v", err)
	}

	want := 50.0 - 2*FeePercent
	if closed.Profit < want-0.1 || closed.Profit > want+0.1 {
		t.Fatalf("Profit = %+.2f%%, ожидалось %+.2f%%", closed.Profit, want)
	}
}

// Доля вне (0,1) должна отклоняться - 0/100%+ - это не частичное закрытие.
func TestReducePositionRejectsInvalidFraction(t *testing.T) {
	pw := newTestWallet(t, 100)

	created, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 1, SideType: order.SideTypeBuy})
	if err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	if _, err := pw.ReducePosition(created.ID, 0, order.Deal{}); err == nil {
		t.Fatal("ожидалась ошибка при fraction=0")
	}
	if _, err := pw.ReducePosition(created.ID, 1.0, order.Deal{}); err == nil {
		t.Fatal("ожидалась ошибка при fraction=1.0 (это уже полное закрытие)")
	}
}

// Комиссия списывается за обе стороны сделки и на закрытии, и в текущем профите
// открытой позиции. Шорт - симметрично лонгу.
func TestFeesAppliedToProfit(t *testing.T) {
	pw := newTestWallet(t, 100)

	created, err := pw.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 1, SideType: order.SideTypeBuy})
	if err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	// Цена не сдвинулась: гросс 0, чистый результат - минус комиссии.
	// Ровно это и отличает честный кошелёк от бесплатного.
	updated := pw.UpdateOrdersPrice("BTCUSDT", 100)
	if want := -2 * FeePercent; updated[0].Profit != want {
		t.Fatalf("профит без движения цены = %v, ожидалось %v (комиссии)", updated[0].Profit, want)
	}

	closed, err := pw.ClosePosition(created.ID, order.Deal{Strategy: "test"})
	if err != nil {
		t.Fatalf("ClosePosition: %v", err)
	}
	if want := -2 * FeePercent; closed.Profit != want {
		t.Fatalf("профит закрытой сделки = %v, ожидалось %v", closed.Profit, want)
	}
}
