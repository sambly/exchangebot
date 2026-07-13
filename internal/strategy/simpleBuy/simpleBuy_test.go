package simplebuy

import (
	"testing"
	"time"

	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/strategy/sales"
	"github.com/sambly/exchangebot/internal/strategy/signal"
	"github.com/sambly/exchangebot/internal/toggle"
)

func testStrategy() *StrategySimpleBuy {
	return &StrategySimpleBuy{
		Config: &Config{
			IDName:              "simplebuy",
			Auto:                true,
			Direction:           "up",
			MinLevel:            2,
			Sources:             []string{"anomaly"},
			Size:                1.0,
			MaxPositions:        2,
			PairCooldownMinutes: 60,
		},
		StrategyEnable: toggle.New(true),
		positions:      make(map[string][]sales.Position),
		lastClose:      make(map[string]time.Time),
	}
}

func sig(pair string, level int, direction signal.Direction, source string) signal.Signal {
	return signal.Signal{
		Source: source, Pair: pair, Period: "15m",
		Direction: direction, Level: level, Strength: 6.5,
	}
}

// Фильтры конфига: слабые сигналы, чужое направление и чужой источник не торгуем.
func TestConfigAllows(t *testing.T) {
	cfg := testStrategy().Config

	cases := []struct {
		name string
		sig  signal.Signal
		want bool
	}{
		{"подходит", sig("BTCUSDT", 2, signal.DirectionUp, "anomaly"), true},
		{"слабый уровень", sig("BTCUSDT", 1, signal.DirectionUp, "anomaly"), false},
		{"падение при direction=up", sig("BTCUSDT", 3, signal.DirectionDown, "anomaly"), false},
		{"чужой источник", sig("BTCUSDT", 3, signal.DirectionUp, "base"), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, ok := cfg.Allows(c.sig); ok != c.want {
				t.Fatalf("Allows() = %v, ожидалось %v", ok, c.want)
			}
		})
	}
}

func TestConfigDirectionBoth(t *testing.T) {
	cfg := testStrategy().Config
	cfg.Direction = "both"

	if _, ok := cfg.Allows(sig("BTCUSDT", 2, signal.DirectionDown, "anomaly")); !ok {
		t.Fatal("при direction=both падение должно торговаться")
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
