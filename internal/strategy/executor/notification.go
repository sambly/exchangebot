package executor

import (
	"fmt"

	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/strategy/sales"
)

// NotificationEntry - сообщение об открытой позиции.
//
// План выхода показываем сразу: в нём весь смысл сделки. Видно, на что мы
// рассчитываем, чем рискуем и когда сдаёмся, если ничего не произошло.
func (s *Executor) NotificationEntry(position sales.Position) string {
	sig := position.Signal

	icon, action := "🟢", "Покупка"
	if position.Order.Side == order.SideTypeSell {
		icon, action = "🔻", "Продажа"
	}

	out := fmt.Sprintf("%s %s %s (%s)\n", icon, action, sig.Pair, sig.Period)
	out += fmt.Sprintf("Сигнал: %s (%+.2f%%)\n", sig.Reason, sig.ChangePercent)
	out += fmt.Sprintf("Цена: %v\n", position.Order.PriceCreated)
	out += fmt.Sprintf("Тейк: +%.2f%%  Стоп: -%.2f%%\n", position.TakeProfitPercent, position.StopLossPercent)

	if sig.Volatility > 0 {
		out += fmt.Sprintf("Волатильность пары: %.2f%% за период\n", sig.Volatility)
	}
	if !position.Deadline.IsZero() {
		out += fmt.Sprintf("Держим до: %s\n", position.Deadline.Format("15:04"))
	}

	out += fmt.Sprintf("https://www.tradingview.com/chart/?symbol=BINANCE:%s", sig.Pair)
	return out
}
