package executor

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
	entryButton = tele.Btn{Text: "🤝 Исполнитель сделок"}
	// Базовые кнопки в меню
	replyButtons = [][]tele.Btn{
		{global.BtnBack, global.BtnMainMenu},
	}

	// Inline кнопки.
	//
	// Unique - это идентификатор колбэка, он уходит в Telegram и приходит обратно.
	// Менять его безопасно: старые кнопки живут только в уже отправленных
	// сообщениях, а меню перерисовывается при каждом входе.
	btnEnableStr  = tele.Btn{Text: "✅ Включить исполнителя", Unique: "enable_executor"}
	btnDisablStr  = tele.Btn{Text: "❌ Отключить исполнителя", Unique: "disable_executor"}
	inlineButtons = [][]tele.Btn{
		{btnEnableStr, btnDisablStr},
	}
)

type ExecutorMenu struct {
	// bot и handler появляются только при старте Telegram (в Handle), а читает
	// их SendMessageDeal из горутины исполнителя. Telegram и стратегии стартуют
	// параллельно, поэтому доступ обязан быть синхронизирован, а вызов до
	// инициализации - возвращать ошибку, а не падать на nil-боте.
	mu      sync.RWMutex
	b       *tele.Bot
	handler model.MenuHandler

	*base.BaseMenu
	Executor *Executor
}

func (m *ExecutorMenu) setBot(b *tele.Bot, handler model.MenuHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.b = b
	m.handler = handler
}

func (m *ExecutorMenu) bot() (*tele.Bot, model.MenuHandler, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.b == nil || m.handler == nil {
		return nil, nil, errors.New("telegram menu ещё не инициализирован")
	}
	return m.b, m.handler, nil
}

func NewMenu(name, id string, exec *Executor) *ExecutorMenu {
	menu := &ExecutorMenu{
		BaseMenu: base.NewBaseMenu(name, id),
		Executor: exec,
	}

	menu.AddButtonRows(replyButtons...)
	menu.WithEntryButton(entryButton)
	menu.AddButtonRowsInline(inlineButtons...)

	return menu
}

func (m *ExecutorMenu) Show(c tele.Context, handler model.MenuHandler) error {

	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show, nil)
	handler.DeleteUserMessages(c, userID)

	cfg := m.Executor.Config

	text := fmt.Sprintf("🤝 %s\nИсполняет сигналы детекторов: открывает и ведёт позиции.\n\n", cfg.Name)
	if m.Executor.IsEnabled() {
		text += "Исполнитель: ✅ включён\n"
	} else {
		text += "Исполнитель: ❌ отключён\n"
	}

	if cfg.Auto {
		text += fmt.Sprintf("Режим: 🤖 автоматический\nРост → %s, падение → %s (от уровня %d)\nЛимит позиций: %d, размер: %v\nОткрыто сейчас: %d\n",
			cfg.OnUp, cfg.OnDown, cfg.MinLevel, cfg.MaxPositions, cfg.Size, m.Executor.OpenPositions())
	} else {
		text += fmt.Sprintf("Режим: 🖐 ручной (подтверждение кнопкой)\nРост → %s, падение → %s (от уровня %d)\n",
			cfg.OnUp, cfg.OnDown, cfg.MinLevel)
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

// SendMessageDeal предлагает сделку и, если пользователь подтвердил, открывает её.
// План выхода (tp/sl) приходит уже посчитанным - он показывается человеку и
// сохраняется вместе со сделкой.
//
// Сторона приходит снаружи: на аномальный РОСТ это может быть покупка, на
// аномальное ПАДЕНИЕ - продажа. Раньше кнопка всегда была "Купить", и на падение
// на 10% приходило предложение ловить нож.
func (m *ExecutorMenu) SendMessageDeal(ctx context.Context, sig signal.Signal, side order.SideType, takeProfit, stopLoss float64) (order.Order, error) {

	bot, menuHandler, err := m.bot()
	if err != nil {
		return order.Order{}, err
	}

	action, question := "Купить", "Будете совершать покупку?"
	if side == order.SideTypeSell {
		action, question = "Продать", "Будете совершать продажу?"
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	unique := "deal_btn_" + uuid.New().String()[:8]
	btn := tele.Btn{
		Unique: unique,
		Text:   action,
	}
	localMarkup := &tele.ReplyMarkup{}
	localMarkup.Inline(localMarkup.Row(btn))

	text := fmt.Sprintf("%s\nПара: %s\nПериод: %s\nИзменение: %+.2f%%\nСигнал: %s\nТейк: +%.2f%%  Стоп: -%.2f%%\nhttps://www.tradingview.com/chart/?symbol=BINANCE:%s",
		question, sig.Pair, sig.Period, sig.ChangePercent, sig.Reason, takeProfit, stopLoss, sig.Pair)

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
		deal := NewDeal(sig, side, m.Executor.Config.Size, takeProfit, stopLoss, Telegram)

		order, err := m.Executor.OrderController.CreateOrderMarket(deal)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{
				Text:      fmt.Sprintf("Ошибка создания ордера: %v", err),
				ShowAlert: true,
			})
		}

		select {
		case resultChan <- order:
		default:
		}

		// Текст ответа - по стороне сделки: на падении мы продаём, и "Покупка
		// обработана" здесь врала бы.
		return c.Respond(&tele.CallbackResponse{
			Text:      fmt.Sprintf("%s: сделка открыта ✅", action),
			ShowAlert: true,
		})
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

// Handle обрабатывает кнопки меню исполнителя
func (m *ExecutorMenu) Handle(b *tele.Bot, handler model.MenuHandler) {

	m.setBot(b, handler)

	// Обработчик кнопки входа в меню исполнителя
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})

	// Выключение исполнителя НЕ выключает детекторы: аномалии продолжат
	// находиться и уходить в дайджест, просто по ним не будет сделок.
	b.Handle(&btnEnableStr, func(c tele.Context) error {
		m.Executor.SetEnabled(true)
		return c.Respond(&tele.CallbackResponse{Text: "Исполнитель включён ✅", ShowAlert: true})
	})

	b.Handle(&btnDisablStr, func(c tele.Context) error {
		m.Executor.SetEnabled(false)
		return c.Respond(&tele.CallbackResponse{Text: "Исполнитель отключён ❌", ShowAlert: true})
	})
}
