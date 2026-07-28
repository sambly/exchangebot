package executor

import (
	"context"
	"sync"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/exchangebot/internal/logger"
	"github.com/sambly/exchangebot/internal/notification"
	"github.com/sambly/exchangebot/internal/order"
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

var execLogger = logger.AddFields(map[string]interface{}{
	"package": "executor",
})

// Executor - ИСПОЛНИТЕЛЬ СДЕЛОК, а не стратегия.
//
// Он не ищет события и не решает, что на рынке происходит - это работа
// детекторов (anomaly, base). Он отвечает на другой вопрос: "пришёл сигнал -
// открывать ли по нему позицию, на какую сторону и каким объёмом".
//
// Пакет намеренно НЕ ЗНАЕТ ни про anomaly, ни про base: единственный вход -
// канал signal.Signal. Поэтому новый детектор не требует здесь ни строчки, а
// новая торговая логика не трогает детекторы.
type Executor struct {
	Config       *Config
	Notification *notification.Notification
	TelegramMenu *ExecutorMenu

	OrderController *order.OrderService

	// Signals - вход от ЛЮБОГО детектора. Кто прислал сигнал, исполнителю
	// неинтересно: важны только его направление, сила и волатильность пары.
	Signals chan signal.Signal

	// markets - рыночные тики, переложенные из горутины фида в свою
	markets chan exModel.MarketsStat

	// Enabled переключается из телеграм-меню, читается своей горутиной.
	// Выключение исполнителя НЕ выключает детекторы: сигналы продолжат приходить,
	// просто по ним не будет сделок.
	Enabled *toggle.Bool

	positionsMu sync.Mutex
	positions   map[string][]sales.Position
	// lastClose - когда по паре последний раз закрывали позицию (cooldown)
	lastClose map[string]time.Time

	// partialTaken - по каким Order.ID уже сработал частичный тейк (см.
	// tryPartialTakeProfit). У sales.Sales.PartialTakeProfit самого по себе
	// нет состояния "уже сработало" - оно здесь, под тем же мьютексом, что и
	// positions. Чистится в removePositions вместе с самой позицией.
	partialTaken map[int64]bool

	Sale sales.Sales
}

func New(
	notify *notification.Notification,
	orderController *order.OrderService,
) (*Executor, error) {

	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}

	exec := &Executor{
		Config:          cfg,
		Notification:    notify,
		OrderController: orderController,
		Signals:         make(chan signal.Signal, signalBuffer),
		markets:         make(chan exModel.MarketsStat, marketBuffer),
		Enabled:         toggle.New(cfg.StrategyEnable),
		positions:       make(map[string][]sales.Position),
		lastClose:       make(map[string]time.Time),
		partialTaken:    make(map[int64]bool),
	}

	orderController.AddOrdersDependencies(exec.onOrderUpdated)
	return exec, nil
}

func (s *Executor) WithTelegramMenu() *Executor {
	s.TelegramMenu = NewMenu(s.Config.Name, s.Config.IDName, s)
	return s
}

func (s *Executor) WithSaleStrategy(sale sales.Sales) *Executor {
	s.Sale = sale
	return s
}

func (s *Executor) GetTelegramMenu() model.WindowHandler {
	return s.TelegramMenu
}

func (s *Executor) IsEnabled() bool   { return s.Enabled.Get() }
func (s *Executor) SetEnabled(v bool) { s.Enabled.Set(v) }

// GetIDName/GetName - см. strategy.WebToggle.
func (s *Executor) GetIDName() string { return s.Config.IDName }
func (s *Executor) GetName() string   { return s.Config.Name }

// Start - единственное место, где исполнитель что-то делает.
//
// И сигналы, и рыночные тики обрабатываются ЗДЕСЬ, в своей горутине. Раньше
// проверка позиций жила в OnMarket, то есть исполнялась внутри чтения потока с
// биржи и ходила оттуда в БД.
func (s *Executor) Start(ctx context.Context) error {
	if s.Config.Auto {
		execLogger.Infof("авторежим включён: рост→%s падение→%s minLevel=%d maxPositions=%d size=%v",
			s.Config.OnUp, s.Config.OnDown, s.Config.MinLevel, s.Config.MaxPositions, s.Config.Size)
	}

	for {
		select {
		case sig := <-s.Signals:
			if s.IsEnabled() {
				s.onSignal(ctx, sig)
			}

		case ms := <-s.markets:
			if s.IsEnabled() {
				s.checkPositions(ms)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// OnMarket вызывается из горутины фида - поэтому только перекладывает тик в
// очередь и сразу возвращается.
func (s *Executor) OnMarket(ms exModel.MarketsStat) {
	if !s.IsEnabled() || s.Sale == nil {
		return
	}

	select {
	case s.markets <- ms:
	default:
		// Очередь забита - тик пропускаем. Позиция проверится на следующем.
	}
}

// onSignal решает, входить ли по сигналу и НА КАКУЮ СТОРОНУ
func (s *Executor) onSignal(ctx context.Context, sig signal.Signal) {
	side, reject, ok := s.Config.SideFor(sig)
	if !ok {
		// Debug, не Info: minLevel и sources отсекают почти весь входящий поток,
		// на уровне info лог был бы нечитаем.
		execLogger.Debugf("сигнал %s %s отклонён: %s", sig.Pair, sig.Period, reject)
		return
	}

	if reason, ok := s.riskAllows(sig); !ok {
		execLogger.Infof("вход по %s %s отклонён риск-лимитом: %s", sig.Pair, sig.Period, reason)
		return
	}

	if s.Config.Auto {
		s.openPosition(sig, side)
		return
	}

	// Ручной режим: спрашиваем подтверждение в Telegram. План выхода считаем
	// заранее - он и в кнопку попадёт, и в БД вместе со сделкой.
	takeProfit, stopLoss, hold := s.plan(sig)

	if s.TelegramMenu == nil {
		return
	}
	go func() {
		newOrder, err := s.TelegramMenu.SendMessageDeal(ctx, sig, side, takeProfit, stopLoss)
		if err != nil {
			return
		}
		s.addPosition(newOrder, sig, takeProfit, stopLoss, hold)
	}()
}

// OpenPositions - сколько позиций открыто сейчас (для телеграм-меню)
func (s *Executor) OpenPositions() int {
	s.positionsMu.Lock()
	defer s.positionsMu.Unlock()

	total := 0
	for _, list := range s.positions {
		total += len(list)
	}
	return total
}

// riskAllows - ограничители, без которых авторежим опасен.
func (s *Executor) riskAllows(sig signal.Signal) (string, bool) {
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

func (s *Executor) openPosition(sig signal.Signal, side order.SideType) {
	// План считаем ДО открытия: он уходит в БД вместе со сделкой, и потом по
	// нему можно понять, на что мы рассчитывали, когда входили.
	takeProfit, stopLoss, hold := s.plan(sig)

	deal := NewDeal(sig, side, s.Config.Size, takeProfit, stopLoss, Auto, s.saleName())

	newOrder, err := s.OrderController.CreateOrderMarket(deal)
	if err != nil {
		execLogger.Errorf("не удалось открыть позицию по %s: %v", sig.Pair, err)
		return
	}

	position := s.addPosition(newOrder, sig, takeProfit, stopLoss, hold)

	execLogger.Infof("entry pair=%s id=%d side=%s strategy=%s period=%s level=%d z=%.2f price=%v tp=%.2f%% sl=-%.2f%%",
		sig.Pair, newOrder.ID, side, sig.Source, sig.Period, sig.Level, sig.Strength,
		newOrder.PriceCreated, position.TakeProfitPercent, position.StopLossPercent)

	if s.Notification != nil {
		s.Notification.SendMessage(s.NotificationEntry(position))
	}
}

// КЕМ инициирована сделка. Отделено от стратегии: стратегия отвечает на вопрос
// "почему вошли", а это - "как именно нажали кнопку".
const (
	Auto     = "auto"     // автомат по сигналу
	Telegram = "telegram" // подтверждение кнопкой в Telegram
	Web      = "web"      // руками из веб-интерфейса
)

// NewDeal собирает сделку из сигнала.
//
// Ключевое: Strategy - это ИСТОЧНИК СИГНАЛА (anomaly, base), а не имя
// исполнителя. Раньше сюда писался simplebuy, и в интерфейсе у всех сделок
// значилась стратегия "simplebuy" - но simplebuy это механизм покупки, а не
// причина, по которой мы вошли. Причина - детектор, который дал сигнал.
//
// salePolicy - имя политики выхода, которая берёт позицию под наблюдение
// (см. Sales.Name, Executor.addPosition); пишется в Order.StrategySell сразу,
// а не только при закрытии.
func NewDeal(sig signal.Signal, side order.SideType, size, takeProfit, stopLoss float64, executor, salePolicy string) order.Deal {
	return order.Deal{
		Pair:     sig.Pair,
		SideType: side,
		Size:     size,
		Frame:    sig.Period,
		Strategy: sig.Source,
		Executor: executor,
		Comment:  sig.Reason,

		Level:      sig.Level,
		Strength:   sig.Strength,
		Volatility: sig.Volatility,
		TakeProfit: takeProfit,
		StopLoss:   stopLoss,
		SalePolicy: salePolicy,
	}
}

// saleName - имя прикреплённой политики выхода, если она есть. Отдельный
// метод, а не прямой s.Sale.Name(): у Sale бывает nil (пока не подключена
// политика выхода, см. WithSaleStrategy), и вызывающим не нужно об этом помнить.
func (s *Executor) saleName() string {
	if s.Sale == nil {
		return ""
	}
	return s.Sale.Name()
}

func (s *Executor) plan(sig signal.Signal) (takeProfit, stopLoss float64, hold time.Duration) {
	if s.Sale == nil {
		return 0, 0, 0
	}
	return s.Sale.Plan(sig)
}

// addPosition запоминает позицию вместе с планом выхода
func (s *Executor) addPosition(newOrder order.Order, sig signal.Signal, takeProfit, stopLoss float64, hold time.Duration) sales.Position {
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
func (s *Executor) checkPositions(ms exModel.MarketsStat) {
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

	// Execute/ReducePosition ходят в БД, поэтому зовём их БЕЗ мьютекса.
	// Закрытые убираем по id из актуального списка, а не перезаписываем
	// список снимком.
	closed := make(map[int64]bool)
	for _, position := range open {
		// Частичный тейк - ДО полного выхода: позиция может и подрезаться, и
		// в тот же тик закрыться целиком (например, следом сработал стоп по
		// оставшейся части) - тот же порядок, что в backtest.Engine.Run.
		s.tryPartialTakeProfit(ms, position)

		if s.Sale.Execute(ms, position) {
			closed[position.Order.ID] = true
		}
	}
	if len(closed) == 0 {
		return
	}

	s.removePositions(ms.Pair, closed)
}

// tryPartialTakeProfit - см. sales.Sales.PartialTakeProfit. Срабатывает
// МАКСИМУМ один раз на позицию (partialTaken.ID) - у самого
// PartialTakeProfit состояния нет, "уже сработало" помнит только Executor.
func (s *Executor) tryPartialTakeProfit(ms exModel.MarketsStat, position sales.Position) {
	id := position.Order.ID

	s.positionsMu.Lock()
	already := s.partialTaken[id]
	s.positionsMu.Unlock()
	if already {
		return
	}

	distancePercent, fraction, ok := s.Sale.PartialTakeProfit(position)
	if !ok || fraction <= 0 || fraction >= 1 {
		return
	}

	entry := position.Order.PriceCreated
	if entry == 0 || ms.Price == 0 {
		return
	}

	profit := (ms.Price/entry)*100 - 100
	if position.Order.Side == order.SideTypeSell {
		profit = -profit
	}
	if profit < distancePercent {
		return
	}

	deal := order.Deal{Strategy: s.saleName(), Comment: "partial-take-profit"}
	if err := s.OrderController.ReducePosition(id, fraction, deal); err != nil {
		execLogger.Errorf("не удалось частично закрыть позицию id=%d: %v", id, err)
		return
	}

	s.positionsMu.Lock()
	if s.partialTaken == nil {
		s.partialTaken = make(map[int64]bool)
	}
	s.partialTaken[id] = true
	s.positionsMu.Unlock()

	execLogger.Infof("partial take-profit pair=%s id=%d fraction=%.0f%% profit=%+.2f%%",
		position.Order.Pair, id, fraction*100, profit)
}

func (s *Executor) removePositions(pair string, closed map[int64]bool) {
	s.positionsMu.Lock()
	defer s.positionsMu.Unlock()

	current := s.positions[pair]
	remaining := current[:0]
	for _, position := range current {
		if !closed[position.Order.ID] {
			remaining = append(remaining, position)
		} else {
			delete(s.partialTaken, position.Order.ID)
		}
	}
	s.positions[pair] = remaining
	s.lastClose[pair] = time.Now()
}

// onOrderUpdated вызывается OrderService при изменении ордера: позиция могла
// быть закрыта не нами (руками из веба или из Telegram).
func (s *Executor) onOrderUpdated(updated order.Order) {
	if updated.Status != order.OrderStatusTypeClose {
		return
	}
	s.removePositions(updated.Pair, map[int64]bool{updated.ID: true})
}
