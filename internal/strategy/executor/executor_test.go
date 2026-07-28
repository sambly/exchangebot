package executor

import (
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/model"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/paperwallet"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/sales"
	"github.com/sambly/exchangebot/internal/strategy/signal"
	"github.com/sambly/exchangebot/internal/toggle"
)

func testStrategy() *Executor {
	return &Executor{
		Config: &Config{
			IDName:              "simplebuy",
			Auto:                true,
			OnUp:                "buy",
			OnDown:              "sell",
			MinLevel:            2,
			Sources:             []string{"anomaly"},
			Size:                1.0,
			MaxPositions:        2,
			PairCooldownMinutes: 60,
		},
		Enabled:   toggle.New(true),
		positions: make(map[string][]sales.Position),
		lastClose: make(map[string]time.Time),
	}
}

func sig(pair string, level int, direction signal.Direction, source string) signal.Signal {
	return signal.Signal{
		Source: source, Pair: pair, Period: "15m",
		Direction: direction, Level: level, Strength: 6.5,
	}
}

// Направление сигнала и сторона сделки - РАЗНЫЕ вещи. Раньше сторона всегда
// была BUY, и на аномальное падение приходило предложение купить (поймать нож).
func TestSideForSignal(t *testing.T) {
	cases := []struct {
		name     string
		onUp     string
		onDown   string
		sig      signal.Signal
		wantSide order.SideType
		wantOK   bool
	}{
		{"рост → покупка", "buy", "sell", sig("BTCUSDT", 2, signal.DirectionUp, "anomaly"), order.SideTypeBuy, true},
		{"падение → продажа", "buy", "sell", sig("BTCUSDT", 2, signal.DirectionDown, "anomaly"), order.SideTypeSell, true},
		{"падение → покупка (игра на отскок)", "buy", "buy", sig("BTCUSDT", 2, signal.DirectionDown, "anomaly"), order.SideTypeBuy, true},
		{"рост → продажа (игра на откат)", "sell", "skip", sig("BTCUSDT", 2, signal.DirectionUp, "anomaly"), order.SideTypeSell, true},
		{"падения не торгуем", "buy", "skip", sig("BTCUSDT", 3, signal.DirectionDown, "anomaly"), "", false},
		{"слабый уровень", "buy", "sell", sig("BTCUSDT", 1, signal.DirectionUp, "anomaly"), "", false},
		{"чужой источник", "buy", "sell", sig("BTCUSDT", 3, signal.DirectionUp, "base"), "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := testStrategy().Config
			cfg.OnUp, cfg.OnDown = c.onUp, c.onDown

			side, _, ok := cfg.SideFor(c.sig)
			if ok != c.wantOK || side != c.wantSide {
				t.Fatalf("SideFor() = (%q, %v), ожидалось (%q, %v)", side, ok, c.wantSide, c.wantOK)
			}
		})
	}
}

// Разные периоды сигнала могут требовать разного направления входа: бэктест
// показал, что на 15m/1h выгоднее fade (шорт против импульса), а на 4h -
// momentum. Periods переопределяет глобальный OnUp/OnDown только для своего
// периода, остальные периоды используют глобальное значение.
func TestSideForPeriodOverride(t *testing.T) {
	cfg := &Config{
		OnUp: "buy", OnDown: "sell", MinLevel: 2,
		Periods: map[string]PeriodDirection{
			"15m": {OnUp: "sell", OnDown: "buy"},
		},
	}

	overridden := signal.Signal{Period: "15m", Level: 2, Direction: signal.DirectionUp}
	if side, _, ok := cfg.SideFor(overridden); !ok || side != order.SideTypeSell {
		t.Fatalf("15m должен использовать переопределение (fade): получено (%v, %v)", side, ok)
	}

	fallback := signal.Signal{Period: "1h", Level: 2, Direction: signal.DirectionUp}
	if side, _, ok := cfg.SideFor(fallback); !ok || side != order.SideTypeBuy {
		t.Fatalf("1h без переопределения должен использовать глобальный OnUp: получено (%v, %v)", side, ok)
	}

	// Переопределён только OnUp - OnDown у 15m должен остаться из своего же
	// переопределения (buy), а не провалиться в глобальный.
	down := signal.Signal{Period: "15m", Level: 2, Direction: signal.DirectionDown}
	if side, _, ok := cfg.SideFor(down); !ok || side != order.SideTypeBuy {
		t.Fatalf("15m DOWN должен давать BUY по переопределению: получено (%v, %v)", side, ok)
	}
}

// SkipDivergent отсекает сигналы "пустого стакана" (цена сходила при
// активности ниже обычной) ещё до определения стороны сделки.
func TestSideForSkipsDivergent(t *testing.T) {
	cfg := &Config{OnUp: "buy", OnDown: "sell", MinLevel: 2, SkipDivergent: true}

	divergent := signal.Signal{Level: 2, Direction: signal.DirectionUp, Divergent: true}
	if _, reject, ok := cfg.SideFor(divergent); ok || reject.Code != "divergent" {
		t.Fatalf("Divergent-сигнал при SkipDivergent должен отсекаться, получено ok=%v code=%q", ok, reject.Code)
	}

	normal := signal.Signal{Level: 2, Direction: signal.DirectionUp, Divergent: false}
	if _, _, ok := cfg.SideFor(normal); !ok {
		t.Fatal("не-Divergent сигнал не должен отсекаться SkipDivergent")
	}
}

// Стратегия в ордере - это ИСТОЧНИК СИГНАЛА (почему вошли), а не имя исполнителя.
// Раньше во все сделки писалось "simplebuy" - но это механизм покупки, а не причина.
func TestDealCarriesSignalSourceAsStrategy(t *testing.T) {
	s := sig("BTCUSDT", 2, signal.DirectionUp, "anomaly")

	deal := NewDeal(s, order.SideTypeBuy, 1.0, 2.0, 1.5, Telegram, "salesimple")

	if deal.Strategy != "anomaly" {
		t.Errorf("стратегия сделки = %q, ожидалось %q (источник сигнала)", deal.Strategy, "anomaly")
	}
	if deal.Executor != Telegram {
		t.Errorf("исполнитель = %q, ожидалось %q", deal.Executor, Telegram)
	}
	if deal.Level != s.Level || deal.Strength != s.Strength {
		t.Error("сила сигнала должна уезжать в сделку вместе с планом")
	}
	if deal.SalePolicy != "salesimple" {
		t.Errorf("SalePolicy = %q, ожидалось %q (имя политики выхода, для статуса в открытых сделках)", deal.SalePolicy, "salesimple")
	}
}

// Anomaly на одном движении срабатывает каждую минуту (плюс эскалация уровней).
// Без лимита "одна позиция на пару" автомат набрал бы лестницу по всё более
// высокой цене - это главный риск авторежима.
func TestRiskOnePositionPerPair(t *testing.T) {
	str := testStrategy()

	if _, ok := str.riskAllows(sig("BTCUSDT", 2, signal.DirectionUp, "anomaly")); !ok {
		t.Fatal("первый вход по паре должен разрешаться")
	}

	str.addPosition(order.Order{ID: 1, Pair: "BTCUSDT", PriceCreated: 100}, sig("BTCUSDT", 2, signal.DirectionUp, "anomaly"), 2.0, 1.5, time.Hour)

	if _, ok := str.riskAllows(sig("BTCUSDT", 3, signal.DirectionUp, "anomaly")); ok {
		t.Fatal("второй вход по той же паре должен быть запрещён")
	}
}

// На рыночном движении аномальными становятся десятки пар разом. Войти во все -
// это не диверсификация, а плечо.
func TestRiskMaxPositions(t *testing.T) {
	str := testStrategy() // MaxPositions: 2

	str.addPosition(order.Order{ID: 1, Pair: "BTCUSDT"}, sig("BTCUSDT", 2, signal.DirectionUp, "anomaly"), 2.0, 1.5, time.Hour)
	str.addPosition(order.Order{ID: 2, Pair: "ETHUSDT"}, sig("ETHUSDT", 2, signal.DirectionUp, "anomaly"), 2.0, 1.5, time.Hour)

	if _, ok := str.riskAllows(sig("SOLUSDT", 3, signal.DirectionUp, "anomaly")); ok {
		t.Fatal("при достигнутом лимите позиций вход должен быть запрещён")
	}
}

// После закрытия позиции пара уходит на cooldown: иначе тот же сигнал тут же
// откроет её заново.
func TestRiskPairCooldownAfterClose(t *testing.T) {
	str := testStrategy()

	str.addPosition(order.Order{ID: 1, Pair: "BTCUSDT"}, sig("BTCUSDT", 2, signal.DirectionUp, "anomaly"), 2.0, 1.5, time.Hour)
	str.removePositions("BTCUSDT", map[int64]bool{1: true})

	if _, ok := str.riskAllows(sig("BTCUSDT", 3, signal.DirectionUp, "anomaly")); ok {
		t.Fatal("сразу после закрытия вход по паре должен быть запрещён (cooldown)")
	}

	// Cooldown истёк
	str.lastClose["BTCUSDT"] = time.Now().Add(-2 * time.Hour)
	if _, ok := str.riskAllows(sig("BTCUSDT", 3, signal.DirectionUp, "anomaly")); !ok {
		t.Fatal("после истечения cooldown вход должен разрешаться")
	}
}

// Позицию могут закрыть не мы - руками из веба или из Telegram. Стратегия должна
// об этом узнать, иначе она будет считать пару занятой навсегда.
func TestPositionRemovedOnExternalClose(t *testing.T) {
	str := testStrategy()

	str.addPosition(order.Order{ID: 7, Pair: "BTCUSDT"}, sig("BTCUSDT", 2, signal.DirectionUp, "anomaly"), 2.0, 1.5, time.Hour)
	str.onOrderUpdated(order.Order{ID: 7, Pair: "BTCUSDT", Status: order.OrderStatusTypeClose})

	str.positionsMu.Lock()
	open := len(str.positions["BTCUSDT"])
	str.positionsMu.Unlock()

	if open != 0 {
		t.Fatalf("после внешнего закрытия позиция должна исчезнуть, осталось %d", open)
	}
}

// SideFor превращает направление сигнала в сторону сделки по onUp/onDown
// и отсекает слабые сигналы по minLevel.
func TestSideFor(t *testing.T) {
	cfg := &Config{OnUp: "buy", OnDown: "sell", MinLevel: 2}

	up := signal.Signal{Level: 2, Direction: signal.DirectionUp}
	if side, reason, ok := cfg.SideFor(up); !ok || side != order.SideTypeBuy {
		t.Fatalf("рост уровня 2 должен давать BUY, получено (%v, %q, %v)", side, reason, ok)
	}

	down := signal.Signal{Level: 3, Direction: signal.DirectionDown}
	if side, reason, ok := cfg.SideFor(down); !ok || side != order.SideTypeSell {
		t.Fatalf("падение уровня 3 должно давать SELL, получено (%v, %q, %v)", side, reason, ok)
	}

	weak := signal.Signal{Level: 1, Direction: signal.DirectionUp}
	if _, _, ok := cfg.SideFor(weak); ok {
		t.Fatal("уровень 1 ниже minLevel 2 - торговаться не должен")
	}
}

// --- частичный тейк-профит ---

type stubPricesRepo struct{}

func (stubPricesRepo) SelectMarketStateTimev2(time.Time) ([]exModel.Candle, error) { return nil, nil }
func (stubPricesRepo) SelectDeltaPeriod(string, string) ([]model.ChangeDeltaForCandle, error) {
	return nil, nil
}
func (stubPricesRepo) SelectCandlesFromPeriod(string, time.Time) ([]exModel.Candle, error) {
	return nil, nil
}

type stubOrderRepo struct{}

func (stubOrderRepo) GetAll() ([]*order.Order, error)          { return nil, nil }
func (stubOrderRepo) Create(*order.Order) error                { return nil }
func (stubOrderRepo) ClosePosition(int64, *order.Order) error  { return nil }
func (stubOrderRepo) ReducePosition(int64, *order.Order) error { return nil }
func (stubOrderRepo) CreateInfo(*order.OrderInfo) error        { return nil }
func (stubOrderRepo) Delete(int64) error                       { return nil }
func (stubOrderRepo) DeleteAllHistory() error                  { return nil }
func (stubOrderRepo) ClearSalePolicyForActiveOrders() error     { return nil }

// newTestOrderController - настоящий OrderService поверх настоящего
// PaperWallet (частичное закрытие меняет реальное состояние ордера, тестировать
// это стоит на реальной реализации, а не на моке). Возвращает и *prices.
// AssetsPrices: PaperWallet.ReducePosition/ClosePosition берут ТЕКУЩУЮ цену
// именно оттуда, не из ms, переданного в Executor.checkPositions - в бою оба
// обновляются одним и тем же тиком, а в тесте это нужно сделать явно.
func newTestOrderController(t *testing.T, price float64) (*order.OrderService, *prices.AssetsPrices) {
	t.Helper()

	periods := map[string]time.Duration{"15m": 15 * time.Minute}
	ap, err := prices.NewAssetsPrices([]string{"BTCUSDT"}, periods, periods, stubPricesRepo{})
	if err != nil {
		t.Fatalf("NewAssetsPrices: %v", err)
	}
	ap.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: price, Time: time.Now()})

	pw := paperwallet.NewPaperWallet(ap)
	svc, err := order.NewOrderService(stubOrderRepo{}, pw, notification.NewSocketsMessage(), ap)
	if err != nil {
		t.Fatalf("NewOrderService: %v", err)
	}
	return svc, ap
}

// stubSalesWithPartial - минимальная sales.Sales с одним уровнем частичного
// тейка; Execute всегда false - этот тест проверяет ТОЛЬКО частичное
// закрытие, полный выход здесь не нужен и не тестируется.
type stubSalesWithPartial struct {
	partialAtFraction, partialFraction float64
}

func (s stubSalesWithPartial) Name() string { return "stub-partial" }
func (s stubSalesWithPartial) Plan(sig signal.Signal) (float64, float64, time.Duration) {
	return 0, 0, 0
}
func (s stubSalesWithPartial) ShouldExit(float64, time.Time, sales.Position) (sales.ExitReason, bool) {
	return "", false
}
func (s stubSalesWithPartial) Execute(exModel.MarketsStat, sales.Position) bool { return false }
func (s stubSalesWithPartial) PartialTakeProfit(position sales.Position) (float64, float64, bool) {
	if s.partialAtFraction <= 0 {
		return 0, 0, false
	}
	return position.TakeProfitPercent * s.partialAtFraction, s.partialFraction, true
}

// Цена достигла частичного уровня - должна списаться доля объёма и вырасти
// RealizedProfit, а сама позиция - остаться открытой (не уйти из Executor).
func TestTryPartialTakeProfitReducesPosition(t *testing.T) {
	oc, ap := newTestOrderController(t, 100)

	created, err := oc.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 2.0, SideType: order.SideTypeBuy})
	if err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	exec := testStrategy()
	exec.OrderController = oc
	exec.Sale = stubSalesWithPartial{partialAtFraction: 0.5, partialFraction: 0.5}

	position := sales.Position{Order: created, TakeProfitPercent: 10.0, StopLossPercent: 8.0}
	exec.positions["BTCUSDT"] = []sales.Position{position}

	// +5% - ровно половина дистанции до тейка (10% x 0.5). Обновляем и ap
	// (откуда PaperWallet реально берёт цену на закрытии), и ms - в бою оба
	// приходят из одного тика.
	ap.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: 105, Time: time.Now()})
	exec.checkPositions(exModel.MarketsStat{Pair: "BTCUSDT", Price: 105, Time: time.Now()})

	active := oc.State.GetActiveOrdersBySymbol("BTCUSDT")
	if len(active) != 1 {
		t.Fatalf("ожидалась 1 активная позиция, получено %d", len(active))
	}
	if active[0].Quantity != 1.0 {
		t.Fatalf("Quantity: ожидалось 1.0 (половина от 2.0), получено %v", active[0].Quantity)
	}
	if active[0].RealizedProfit <= 0 {
		t.Fatalf("RealizedProfit: ожидалось положительное значение, получено %v", active[0].RealizedProfit)
	}

	if len(exec.positions["BTCUSDT"]) != 1 {
		t.Fatal("позиция должна остаться в Executor после частичного закрытия, а не пропасть")
	}
}

// Повторный тик после срабатывания не должен резать позицию ещё раз - один
// уровень частичного тейка срабатывает максимум один раз на позицию.
func TestTryPartialTakeProfitFiresOnce(t *testing.T) {
	oc, ap := newTestOrderController(t, 100)

	created, err := oc.CreateOrderMarket(order.Deal{Pair: "BTCUSDT", Size: 2.0, SideType: order.SideTypeBuy})
	if err != nil {
		t.Fatalf("CreateOrderMarket: %v", err)
	}

	exec := testStrategy()
	exec.OrderController = oc
	exec.Sale = stubSalesWithPartial{partialAtFraction: 0.5, partialFraction: 0.5}

	position := sales.Position{Order: created, TakeProfitPercent: 10.0, StopLossPercent: 8.0}
	exec.positions["BTCUSDT"] = []sales.Position{position}

	for _, price := range []float64{105, 106, 107} {
		ap.OnMarket(exModel.MarketsStat{Pair: "BTCUSDT", Price: price, Time: time.Now()})
		exec.checkPositions(exModel.MarketsStat{Pair: "BTCUSDT", Price: price, Time: time.Now()})
	}

	active := oc.State.GetActiveOrdersBySymbol("BTCUSDT")
	if len(active) != 1 {
		t.Fatalf("ожидалась 1 активная позиция, получено %d", len(active))
	}
	if active[0].Quantity != 1.0 {
		t.Fatalf("повторные тики не должны резать позицию снова: Quantity = %v, ожидалось 1.0", active[0].Quantity)
	}
}
