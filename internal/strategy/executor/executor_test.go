package executor

import (
	"testing"
	"time"

	"github.com/sambly/exchangebot/internal/order"
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
