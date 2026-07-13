package simplebuy

import (
	"context"
	"sync"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/prices"
	"github.com/sambly/exchangebot/internal/strategy/sales"
	"github.com/sambly/exchangebot/internal/strategy/signal"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	"github.com/sambly/exchangebot/internal/toggle"
)

const (
	// signalBuffer - очередь сигналов от детекторов
	signalBuffer = 128
	// marketBuffer - очередь рыночных тиков для проверки открытых позиций.
	// Тики приходят из горутины фида, и она не должна ждать нас: проверка
	// позиции ходит в БД, а фид в это время читает поток с биржи.
	marketBuffer = 1024
)

var buyLogger = logger.AddFields(map[string]interface{}{
	"package": "simplebuy",
})

type StrategySimpleBuy struct {
	Config       *Config
	Notification *notification.Notification
	TelegramMenu *StrategySimpleBuyMenu

	AssetsPrices    *prices.AssetsPrices
	OrderController *order.OrderService

	// Signals - вход от ЛЮБОГО детектора (anomaly, base, ...). Стратегия не
	// знает, кто прислал сигнал, и это осознанно: новый детектор не должен
	// требовать правок в исполнителе.
	Signals chan signal.Signal

	// markets - рыночные тики, переложенные из горутины фида в свою
	markets chan exModel.MarketsStat

	// StrategyEnable переключается из телеграм-меню, читается своей горутиной
	StrategyEnable *toggle.Bool

	positionsMu sync.Mutex
	positions   map[string][]sales.Position
	// lastClose - когда по паре последний раз закрывали позицию (cooldown)
	lastClose map[string]time.Time

	Sale sales.Sales
}

func NewStrategy(
	notify *notification.Notification,
	assetsPrices *prices.AssetsPrices,
	orderController *order.OrderService,
) (*StrategySimpleBuy, error) {

	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}

	str := &StrategySimpleBuy{
		Config:          cfg,
		Notification:    notify,
		AssetsPrices:    assetsPrices,
		OrderController: orderController,
		Signals:         make(chan signal.Signal, signalBuffer),
		markets:         make(chan exModel.MarketsStat, marketBuffer),
		StrategyEnable:  toggle.New(cfg.StrategyEnable),
		positions:       make(map[string][]sales.Position),
		lastClose:       make(map[string]time.Time),
	}

	orderController.AddOrdersDependencies(str.onOrderUpdated)
	return str, nil
}

func (s *StrategySimpleBuy) WithTelegramMenu() *StrategySimpleBuy {
	s.TelegramMenu = NewStrategyMenu(s.Config.Name, s.Config.IDName, s)
	return s
}

func (s *StrategySimpleBuy) WithSaleStrategy(sale sales.Sales) *StrategySimpleBuy {
	s.Sale = sale
	return s
}

func (s *StrategySimpleBuy) GetTelegramMenu() model.WindowHandler {
	return s.TelegramMenu
}

func (s *StrategySimpleBuy) IsStrategyEnabled() bool   { return s.StrategyEnable.Get() }
func (s *StrategySimpleBuy) SetStrategyEnabled(v bool) { s.StrategyEnable.Set(v) }

// Start - единственное место, где стратегия что-то делает.
//
// И сигналы, и рыночные тики обрабатываются ЗДЕСЬ, в своей горутине. Раньше
// проверка позиций жила в OnMarket, то есть исполнялась внутри чтения потока с
// биржи и ходила оттуда в БД.
func (s *StrategySimpleBuy) Start(ctx context.Context) error {
	if s.Config.Auto {
		buyLogger.Infof("авторежим включён: direction=%s minLevel=%d maxPositions=%d size=%v",
			s.Config.Direction, s.Config.MinLevel, s.Config.MaxPositions, s.Config.Size)
	}

	for {
		select {
		case sig := <-s.Signals:
			if s.IsStrategyEnabled() {
				s.onSignal(ctx, sig)
			}

		case ms := <-s.markets:
			if s.IsStrategyEnabled() {
				s.checkPositions(ms)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// OnMarket вызывается из горутины фида - поэтому только перекладывает тик в
// очередь и сразу возвращается.
func (s *StrategySimpleBuy) OnMarket(ms exModel.MarketsStat) {
	if !s.IsStrategyEnabled() || s.Sale == nil {
		return
	}

	select {
	case s.markets <- ms:
	default:
		// Очередь забита - тик пропускаем. Позиция проверится на следующем.
	}
}

// onSignal решает, входить ли по сигналу
func (s *StrategySimpleBuy) onSignal(ctx context.Context, sig signal.Signal) {
	if reason, ok := s.Config.Allows(sig); !ok {
		buyLogger.Debugf("сигнал %s %s отклонён: %s", sig.Pair, sig.Period, reason)
		return
	}

	if reason, ok := s.riskAllows(sig); !ok {
		buyLogger.Infof("вход по %s %s отклонён риск-лимитом: %s", sig.Pair, sig.Period, reason)
		return
	}

	if s.Config.Auto {
		s.openPosition(sig)
		return
	}

	// Ручной режим: спрашиваем подтверждение в Telegram. План выхода считаем
	// заранее - он и в кнопку попадёт, и в БД вместе со сделкой.
	takeProfit, stopLoss, hold := s.plan(sig)

	if s.TelegramMenu == nil {
		return
	}
	go func() {
		newOrder, err := s.TelegramMenu.SendMessageBuy(ctx, sig, takeProfit, stopLoss)
		if err != nil {
			return
		}
		s.addPosition(newOrder, sig, takeProfit, stopLoss, hold)
	}()
}

// OpenPositions - сколько позиций открыто сейчас (для телеграм-меню)
func (s *StrategySimpleBuy) OpenPositions() int {
	s.positionsMu.Lock()
	defer s.positionsMu.Unlock()

	total := 0
	for _, list := range s.positions {
		total += len(list)
	}
	return total
}

// riskAllows - ограничители, без которых авторежим опасен.
func (s *StrategySimpleBuy) riskAllows(sig signal.Signal) (string, bool) {
	s.positionsMu.Lock()
	defer s.positionsMu.Unlock()

	// Одна позиция на пару: на одном движении anomaly срабатывает каждую минуту,
	// и без этого мы бы набирали лестницу по всё более высокой цене.
	if len(s.positions[sig.Pair]) > 0 {
		return "по паре уже есть открытая позиция", false
	}

	total := 0
	for _, list := range s.positions {
		total += len(list)
	}
	if total >= s.Config.MaxPositions {
		return "достигнут лимит одновременных позиций", false
	}

	cooldown := time.Duration(s.Config.PairCooldownMinutes) * time.Minute
	if last, ok := s.lastClose[sig.Pair]; ok && time.Since(last) < cooldown {
		return "пара на cooldown после закрытия", false
	}

	return "", true
}

func (s *StrategySimpleBuy) openPosition(sig signal.Signal) {
	// План считаем ДО открытия: он уходит в БД вместе со сделкой, и потом по
	// нему можно понять, на что мы рассчитывали, когда входили.
	takeProfit, stopLoss, hold := s.plan(sig)

	deal := order.Deal{
		Pair:     sig.Pair,
		SideType: order.SideTypeBuy,
		Size:     s.Config.Size,
		Frame:    sig.Period,
		Strategy: s.Config.IDName,
		Comment:  sig.Reason,

		Level:      sig.Level,
		Strength:   sig.Strength,
		Volatility: sig.Volatility,
		TakeProfit: takeProfit,
		StopLoss:   stopLoss,
	}

	newOrder, err := s.OrderController.CreateOrderMarket(deal)
	if err != nil {
		buyLogger.Errorf("не удалось открыть позицию по %s: %v", sig.Pair, err)
		return
	}

	position := s.addPosition(newOrder, sig, takeProfit, stopLoss, hold)

	buyLogger.Infof("entry pair=%s id=%d source=%s period=%s level=%d z=%.2f price=%v tp=%.2f%% sl=-%.2f%%",
		sig.Pair, newOrder.ID, sig.Source, sig.Period, sig.Level, sig.Strength,
		newOrder.PriceCreated, position.TakeProfitPercent, position.StopLossPercent)

	if s.Notification != nil {
		s.Notification.SendMessage(s.NotificationEntry(position))
	}
}

func (s *StrategySimpleBuy) plan(sig signal.Signal) (takeProfit, stopLoss float64, hold time.Duration) {
	if s.Sale == nil {
		return 0, 0, 0
	}
	return s.Sale.Plan(sig)
}

// addPosition запоминает позицию вместе с планом выхода
func (s *StrategySimpleBuy) addPosition(newOrder order.Order, sig signal.Signal, takeProfit, stopLoss float64, hold time.Duration) sales.Position {
	position := sales.Position{
		Order:             newOrder,
		Signal:            sig,
		TakeProfitPercent: takeProfit,
		StopLossPercent:   stopLoss,
	}
	if hold > 0 {
		position.Deadline = time.Now().Add(hold)
	}

	s.positionsMu.Lock()
	s.positions[sig.Pair] = append(s.positions[sig.Pair], position)
	s.positionsMu.Unlock()

	return position
}

// checkPositions прогоняет открытые позиции пары через политику выхода
func (s *StrategySimpleBuy) checkPositions(ms exModel.MarketsStat) {
	if s.Sale == nil {
		return
	}

	s.positionsMu.Lock()
	open := make([]sales.Position, len(s.positions[ms.Pair]))
	copy(open, s.positions[ms.Pair])
	s.positionsMu.Unlock()

	if len(open) == 0 {
		return
	}

	// Execute ходит в БД, поэтому зовём его БЕЗ мьютекса. Закрытые убираем по
	// id из актуального списка, а не перезаписываем список снимком.
	closed := make(map[int64]bool)
	for _, position := range open {
		if s.Sale.Execute(ms, position) {
			closed[position.Order.ID] = true
		}
	}
	if len(closed) == 0 {
		return
	}

	s.removePositions(ms.Pair, closed)
}

func (s *StrategySimpleBuy) removePositions(pair string, closed map[int64]bool) {
	s.positionsMu.Lock()
	defer s.positionsMu.Unlock()

	current := s.positions[pair]
	remaining := current[:0]
	for _, position := range current {
		if !closed[position.Order.ID] {
			remaining = append(remaining, position)
		}
	}
	s.positions[pair] = remaining
	s.lastClose[pair] = time.Now()
}

// onOrderUpdated вызывается OrderService при изменении ордера: позиция могла
// быть закрыта не нами (руками из веба или из Telegram).
func (s *StrategySimpleBuy) onOrderUpdated(updated order.Order) {
	if updated.Status != order.OrderStatusTypeClose {
		return
	}
	s.removePositions(updated.Pair, map[int64]bool{updated.ID: true})
}
