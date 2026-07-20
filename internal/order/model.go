package order

import (
	"time"

	"gorm.io/datatypes"
)

type SideType string
type OrderType string
type OrderStatusType string

var (
	SideTypeBuy  SideType = "BUY"
	SideTypeSell SideType = "SELL"

	OrderTypeLimit           OrderType = "LIMIT"
	OrderTypeMarket          OrderType = "MARKET"
	OrderTypeLimitMaker      OrderType = "LIMIT_MAKER"
	OrderTypeStopLoss        OrderType = "STOP_LOSS"
	OrderTypeStopLossLimit   OrderType = "STOP_LOSS_LIMIT"
	OrderTypeTakeProfit      OrderType = "TAKE_PROFIT"
	OrderTypeTakeProfitLimit OrderType = "TAKE_PROFIT_LIMIT"

	OrderStatusTypeNew    OrderStatusType = "NEW"
	OrderStatusTypeFilled OrderStatusType = "FILLED"
	OrderStatusTypeActive OrderStatusType = "ACTIVE"
	OrderStatusTypeClose  OrderStatusType = "Close"
)

type Order struct {
	ID           int64           `gorm:"primarykey;autoIncrement"`
	TimeCreated  time.Time       `gorm:"column:time_created"`
	Time         time.Time       `gorm:"column:time"`
	Pair         string          `gorm:"column:pair"`
	Side         SideType        `gorm:"column:side"`
	Type         OrderType       `gorm:"column:type"`
	Status       OrderStatusType `gorm:"column:status"`
	PriceCreated float64         `gorm:"column:price_created"`
	Price        float64         `gorm:"column:price"`
	Quantity float64 `gorm:"column:quantity"`
	Profit   float64 `gorm:"column:profit"`

	// StrategyBuy / StrategySell - ПОЧЕМУ вошли и почему вышли: имя детектора
	// (anomaly, base) или "manual".
	//
	// Раньше сюда писался simplebuy - но это исполнитель, механизм покупки, а не
	// причина сделки. Кто именно нажал кнопку, теперь пишется в Executor.
	StrategyBuy  string `gorm:"column:strategy_buy"`
	StrategySell string `gorm:"column:strategy_sell"`

	// Executor - КЕМ инициирована сделка: auto, telegram, web.
	Executor string `gorm:"column:executor"`

	// ExitReason - ПОЧЕМУ закрыли: take-profit, stop-loss, timeout, manual.
	// Раньше причина терялась: при закрытии сохранялась только стратегия, и по
	// БД нельзя было отличить "сработал план" от "выбило стопом" - то есть
	// нельзя было оценить, работает ли стратегия вообще.
	ExitReason string `gorm:"column:exit_reason"`
}

type OrderInfo struct {
	ID           uint           `gorm:"primarykey;autoIncrement"`
	IdOrder      uint           `gorm:"column:id_order"`
	Frame        string         `gorm:"column:frame"`
	Strategy     string         `gorm:"column:strategy"`
	Comment      string         `gorm:"column:comment"`
	Executor     string         `gorm:"column:executor"`
	MarketsStat  datatypes.JSON `gorm:"column:markets_stat"`
	ChangePrices datatypes.JSON `gorm:"column:change_prices"`
	DeltaFast    datatypes.JSON `gorm:"column:delta_fast"`

	// План сделки и сила сигнала на момент входа - в отдельных колонках, а не
	// в тексте комментария: по ним потом можно группировать и считать статистику
	// (винрейт по уровням, как часто выбивает стоп при таком-то плане).
	Level      int     `gorm:"column:level"`
	Strength   float64 `gorm:"column:strength"`
	Volatility float64 `gorm:"column:volatility"`
	TakeProfit float64 `gorm:"column:take_profit"`
	StopLoss   float64 `gorm:"column:stop_loss"`
}

type Deal struct {
	Pair     string
	SideType SideType
	Size     float64
	Frame    string
	// Strategy - источник сигнала (anomaly, base) или "manual", а НЕ имя
	// исполнителя: см. комментарий к Order.StrategyBuy.
	Strategy string
	// Executor - кто инициировал: auto, telegram, web
	Executor string
	Comment  string

	// Заполняется при входе (план сделки и сила сигнала) и при выходе
	// (ExitReason). Пустые значения допустимы: ручная сделка из веба плана
	// не имеет.
	Level      int
	Strength   float64
	Volatility float64
	TakeProfit float64
	StopLoss   float64
	ExitReason string

	// SalePolicy - имя политики выхода (Sales.Name), которая при входе
	// поставила позицию под наблюдение (см. Executor.addPosition). Пусто у
	// сделок, открытых напрямую из веба: они минуют Executor и никогда не
	// были ничем не отслежены - это так и для новых, и для старых сделок,
	// не следствие рестарта. Пишется в Order.StrategySell при создании.
	SalePolicy string
}
