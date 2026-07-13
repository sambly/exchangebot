package simplebuy

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sambly/exchangebot/internal/order"
	"github.com/sambly/exchangebot/internal/strategy/signal"
	"github.com/sambly/exchangebot/internal/telegram/menu/base"
	"github.com/sambly/exchangebot/internal/telegram/menu/global"
	"github.com/sambly/exchangebot/internal/telegram/menu/model"
	tele "gopkg.in/telebot.v3"
)

var (
	// Кнопка точка входа
	entryButton = tele.Btn{Text: "SimpleBuy"}
	// Базовые кнопки в меню
	replyButtons = [][]tele.Btn{
		{global.BtnBack, global.BtnMainMenu},
	}

	// Inline кнопки
	btnEnableStr  = tele.Btn{Text: "✅ Включить стратегию", Unique: "enable_str_simple_buy"}
	btnDisablStr  = tele.Btn{Text: "❌ Отключить стратегию", Unique: "disable_str_simple_buy"}
	inlineButtons = [][]tele.Btn{
		{btnEnableStr, btnDisablStr},
	}
)

type StrategySimpleBuyMenu struct {
	// bot и handler появляются только при старте Telegram (в Handle), а читает
	// их SendMessageBuy из горутины стратегии. Telegram и стратегии стартуют
	// параллельно, поэтому доступ обязан быть синхронизирован, а вызов до
	// инициализации - возвращать ошибку, а не падать на nil-боте.
	mu      sync.RWMutex
	b       *tele.Bot
	handler model.MenuHandler

	*base.BaseMenu
	Strategy *StrategySimpleBuy
}

func (m *StrategySimpleBuyMenu) setBot(b *tele.Bot, handler model.MenuHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.b = b
	m.handler = handler
}

func (m *StrategySimpleBuyMenu) bot() (*tele.Bot, model.MenuHandler, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.b == nil || m.handler == nil {
		return nil, nil, errors.New("telegram menu ещё не инициализирован")
	}
	return m.b, m.handler, nil
}

func NewStrategyMenu(name, id string, str *StrategySimpleBuy) *StrategySimpleBuyMenu {
	menu := &StrategySimpleBuyMenu{
		BaseMenu: base.NewBaseMenu(name, id),
		Strategy: str,
	}

	menu.AddButtonRows(replyButtons...)
	menu.WithEntryButton(entryButton)
	menu.AddButtonRowsInline(inlineButtons...)

	return menu
}

func (m *StrategySimpleBuyMenu) Show(c tele.Context, handler model.MenuHandler) error {

	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show, nil)
	handler.DeleteUserMessages(c, userID)

	cfg := m.Strategy.Config

	text := fmt.Sprintf("Настройки стратегии: %s\n", cfg.Name)
	if m.Strategy.IsStrategyEnabled() {
		text += "Стратегия: включена\n"
	} else {
		text += "Стратегия: отключена\n"
	}

	if cfg.Auto {
		text += fmt.Sprintf("Режим: 🤖 автоматический\nВходим: %s, от уровня %d\nЛимит позиций: %d, размер: %v\nОткрыто сейчас: %d\n",
			cfg.Direction, cfg.MinLevel, cfg.MaxPositions, cfg.Size, m.Strategy.OpenPositions())
	} else {
		text += "Режим: 🖐 ручной (подтверждение кнопкой)\n"
	}

	if err := c.Send(text, m.Markup); err != nil {
		return err
	}

	// Кнопки inline отправляем отдельно
	if len(m.InlineButtons) > 0 {
		if err := c.Send("Выберите действие:", m.InlineMarkup); err != nil {
			return err
		}
	}
	return nil
}

// SendMessageBuy спрашивает подтверждение на вход и, если пользователь нажал
// кнопку, открывает позицию. План выхода (tp/sl) приходит уже посчитанным - он
// показывается человеку и сохраняется вместе со сделкой.
func (m *StrategySimpleBuyMenu) SendMessageBuy(ctx context.Context, sig signal.Signal, takeProfit, stopLoss float64) (order.Order, error) {

	bot, menuHandler, err := m.bot()
	if err != nil {
		return order.Order{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	unique := "buy_btn_" + uuid.New().String()[:8]
	btn := tele.Btn{
		Unique: unique,
		Text:   "Купить",
	}
	localMarkup := &tele.ReplyMarkup{}
	localMarkup.Inline(localMarkup.Row(btn))

	text := fmt.Sprintf("Будете совершать покупку?\nПара: %s\nПериод: %s\nИзменение: %.2f%%\nСигнал: %s\nТейк: +%.2f%%  Стоп: -%.2f%%\nhttps://www.tradingview.com/chart/?symbol=BINANCE:%s",
		sig.Pair, sig.Period, sig.ChangePercent, sig.Reason, takeProfit, stopLoss, sig.Pair)

	msg, err := bot.Send(&tele.User{ID: menuHandler.GetUser()}, text, localMarkup)
	if err != nil {
		return order.Order{}, err
	}

	defer func() {
		_ = bot.Delete(msg)
	}()

	resultChan := make(chan order.Order, 1)

	// Обработчик кнопки.
	//
	// Сделку собираем из самого сигнала, а не из данных кнопки: раньше pair,
	// frame и comment парсились из строки callback'а, что было и хрупко
	// (индексация без проверки длины), и бедно - план сделки туда не влезал.
	handler := func(c tele.Context) error {
		deal := order.Deal{
			Pair:     sig.Pair,
			Frame:    sig.Period,
			Comment:  sig.Reason,
			SideType: order.SideTypeBuy,
			Size:     m.Strategy.Config.Size,
			Strategy: m.Strategy.Config.IDName,

			Level:      sig.Level,
			Strength:   sig.Strength,
			Volatility: sig.Volatility,
			TakeProfit: takeProfit,
			StopLoss:   stopLoss,
		}

		order, err := m.Strategy.OrderController.CreateOrderMarket(deal)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка создания ордера", ShowAlert: true})
		}

		select {
		case resultChan <- order:
		default:
		}

		return c.Respond(&tele.CallbackResponse{Text: "Покупка обработана ✅", ShowAlert: true})

	}

	if err := menuHandler.RegisterCallback(unique, handler); err != nil {
		return order.Order{}, err
	}
	defer menuHandler.UnregisterCallback(btn.Unique)

	select {
	case order := <-resultChan:
		return order, nil
	case <-ctx.Done():
		return order.Order{}, ctx.Err()
	}
}

// Handle обрабатывает кнопки меню стратегий
func (m *StrategySimpleBuyMenu) Handle(b *tele.Bot, handler model.MenuHandler) {

	m.setBot(b, handler)

	// Обработчик кнопки входа в меню стратегий
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})

	b.Handle(&btnEnableStr, func(c tele.Context) error {
		m.Strategy.SetStrategyEnabled(true)
		return c.Respond(&tele.CallbackResponse{Text: "Стратегия включена ✅", ShowAlert: true})
	})

	b.Handle(&btnDisablStr, func(c tele.Context) error {
		m.Strategy.SetStrategyEnabled(false)
		return c.Respond(&tele.CallbackResponse{Text: "Стратегия отключена ❌", ShowAlert: true})
	})
}
