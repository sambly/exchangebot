package simplesale

import (
	"testing"
	"time"

	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/strategy/sales"
	"github.com/sambly/exchangebot/internal/strategy/signal"
)

func testSale() *StrategySimpleSale {
	return &StrategySimpleSale{
		Config: &Config{
			IDName:               "salesimple",
			TakeProfitVol:        2.0,
			StopLossVol:          1.5,
			TakeProfitPercent:    2.0,
			StopLossPercent:      1.5,
			MinTakeProfitPercent: 0.5,
			MaxTakeProfitPercent: 20.0,
			MinStopLossPercent:   0.5,
			MaxStopLossPercent:   10.0,
			MaxHoldPeriods:       4,
		},
	}
}

// Пороги должны зависеть от волатильности пары: 1% для BTC - событие, для
// мемкоина - шум. Одинаковый стоп для обоих либо выбивается шумом, либо не защищает.
func TestPlanScalesWithVolatility(t *testing.T) {
	str := testSale()

	calm := signal.Signal{Period: "15m", Volatility: 0.4} // спокойная пара
	wild := signal.Signal{Period: "15m", Volatility: 3.0} // волатильная пара

	calmTP, calmSL, hold := str.Plan(calm)
	wildTP, wildSL, _ := str.Plan(wild)

	if calmTP >= wildTP || calmSL >= wildSL {
		t.Fatalf("у волатильной пары пороги должны быть шире: спокойная tp=%.2f sl=%.2f, волатильная tp=%.2f sl=%.2f",
			calmTP, calmSL, wildTP, wildSL)
	}

	// 3.0% волатильности x 2.0 = 6% тейк
	if wildTP < 5.9 || wildTP > 6.1 {
		t.Errorf("тейк волатильной пары = %.2f%%, ожидалось ~6%% (3%% x 2.0)", wildTP)
	}

	// 15m x 4 периода = 1 час
	if hold != time.Hour {
		t.Errorf("время удержания = %v, ожидался час (15m x 4)", hold)
	}
}

// Если детектор не знает волатильности (base), откатываемся на проценты из конфига.
func TestPlanFallsBackToPercents(t *testing.T) {
	str := testSale()

	takeProfit, stopLoss, _ := str.Plan(signal.Signal{Period: "1h", Volatility: 0})

	if takeProfit != 2.0 || stopLoss != 1.5 {
		t.Fatalf("без волатильности ожидались проценты из конфига (2.0/1.5), получено %.2f/%.2f",
			takeProfit, stopLoss)
	}
}

// Границы: тейк ниже комиссий бессмыслен, стоп шире разумного - это не стоп.
func TestPlanClampsExtremes(t *testing.T) {
	str := testSale()

	// Почти нулевая волатильность - тейк упёрся бы в 0.02%
	tiny, tinySL, _ := str.Plan(signal.Signal{Period: "15m", Volatility: 0.01})
	if tiny < 0.5 {
		t.Errorf("тейк %.2f%% ниже минимума 0.5%%", tiny)
	}
	if tinySL < 0.5 {
		t.Errorf("стоп %.2f%% ниже минимума 0.5%%", tinySL)
	}

	// Дикая волатильность - стоп ушёл бы за 30%
	huge, hugeSL, _ := str.Plan(signal.Signal{Period: "15m", Volatility: 20})
	if huge > 20.0 {
		t.Errorf("тейк %.2f%% выше максимума 20%%", huge)
	}
	if hugeSL > 10.0 {
		t.Errorf("стоп %.2f%% выше максимума 10%%", hugeSL)
	}
}

func position(entry, takeProfit, stopLoss float64, deadline time.Time) sales.Position {
	return sales.Position{
		Order: order.Order{
			ID:           1,
			Pair:         "BTCUSDT",
			Side:         order.SideTypeBuy,
			PriceCreated: entry,
		},
		TakeProfitPercent: takeProfit,
		StopLossPercent:   stopLoss,
		Deadline:          deadline,
	}
}

// Главное, чего не было раньше: стоп-лосс. Позиция в минусе висела вечно,
// потому что закрытие происходило только при достижении прибыли.
//
// Тест зовёт ShouldExit - чистое решение, общее для боя и бэктеста. Вход у всех
// позиций по 100, поэтому цена = 100 + желаемый профит в процентах.
func TestShouldExit(t *testing.T) {
	str := testSale()

	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	cases := []struct {
		name  string
		price float64
		pos   sales.Position
		want  sales.ExitReason
		exit  bool
	}{
		{"тейк сработал", 102.5, position(100, 2.0, 1.5, future), sales.ExitTakeProfit, true},
		{"стоп сработал", 98.2, position(100, 2.0, 1.5, future), sales.ExitStopLoss, true},
		{"внутри коридора - держим", 100.7, position(100, 2.0, 1.5, future), "", false},
		{"сигнал протух", 100.3, position(100, 2.0, 1.5, past), sales.ExitTimeout, true},
		{"тейк важнее таймаута", 105.0, position(100, 2.0, 1.5, past), sales.ExitTakeProfit, true},
		{"нулевая цена - не решение, а отсутствие данных", 0, position(100, 2.0, 1.5, past), "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason, exit := str.ShouldExit(c.price, now, c.pos)
			if exit != c.exit || reason != c.want {
				t.Fatalf("price=%.2f → (%q, %v), ожидалось (%q, %v)", c.price, reason, exit, c.want, c.exit)
			}
		})
	}
}

// У шорта прибыль перевёрнута: падение цены - это плюс. Проверяем, что тейк
// шорта срабатывает на падении, а стоп - на росте.
func TestShouldExitShortSide(t *testing.T) {
	str := testSale()
	future := time.Now().Add(time.Hour)

	pos := position(100, 2.0, 1.5, future)
	pos.Order.Side = order.SideTypeSell

	if reason, exit := str.ShouldExit(97.5, time.Now(), pos); !exit || reason != sales.ExitTakeProfit {
		t.Fatalf("падение до 97.5 для шорта - тейк, получено (%q, %v)", reason, exit)
	}
	if reason, exit := str.ShouldExit(101.8, time.Now(), pos); !exit || reason != sales.ExitStopLoss {
		t.Fatalf("рост до 101.8 для шорта - стоп, получено (%q, %v)", reason, exit)
	}
}
