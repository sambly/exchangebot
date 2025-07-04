package simplebuy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sambly/exchangebot/internal/order"
	strbase "github.com/sambly/exchangebot/internal/strategy/base"
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
	btnEnableNotifications  = tele.Btn{Text: "🔔 Включить уведомления", Unique: "enable_notif_simple_buy"}
	btnDisableNotifications = tele.Btn{Text: "🔕 Отключить уведомления", Unique: "disable_notif_simple_buy"}
	inlineButtons           = [][]tele.Btn{
		{btnEnableNotifications, btnDisableNotifications},
	}
)

type StrategySimpleBuyMenu struct {
	b       *tele.Bot
	handler model.MenuHandler

	*base.BaseMenu
	Strategy *StrategySimpleBuy
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

	text := fmt.Sprintf("Настройки стратегии: %s\n", m.Strategy.Config.Name)
	if m.Strategy.Config.NotificationEnable {
		text += "Уведомления: включены"
	} else {
		text += "Уведомления: отключены"
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

func (m *StrategySimpleBuyMenu) SendMessageBuy(ctx context.Context, baseResult strbase.StrategyBaseResult) (order.Order, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	unique := "buy_btn_" + uuid.New().String()[:8]
	btn := tele.Btn{
		Unique: unique,
		Text:   "Купить",
		Data: fmt.Sprintf("%s|%s|%.2f",
			baseResult.Data.Pair,
			baseResult.Data.Period,
			baseResult.Data.ChangePercent),
	}
	localMarkup := &tele.ReplyMarkup{}
	localMarkup.Inline(localMarkup.Row(btn))

	text := fmt.Sprintf("Будете совершать покупку?\nПара: %s\nПериод: %s\nИзменение: %.2f%%",
		baseResult.Data.Pair, baseResult.Data.Period, baseResult.Data.ChangePercent)

	msg, err := m.b.Send(&tele.User{ID: m.handler.GetUser()}, text, localMarkup)
	if err != nil {
		return order.Order{}, err
	}

	defer func() {
		_ = m.b.Delete(msg)
	}()

	resultChan := make(chan order.Order, 1)

	// Обработчик кнопки
	handler := func(c tele.Context) error {
		data := c.Callback().Data
		parts := strings.Split(data, "|")
		deal := order.Deal{
			Pair:     parts[0],
			Frame:    parts[1],
			Comment:  parts[2],
			SideType: order.SideTypeBuy,
			Size:     1.0,
			Strategy: m.Strategy.Config.IDName,
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

	m.handler.RegisterCallback(unique, handler)
	defer m.handler.UnregisterCallback(btn.Unique)

	select {
	case order := <-resultChan:
		return order, nil
	case <-ctx.Done():
		return order.Order{}, ctx.Err()
	}
}

// Handle обрабатывает кнопки меню стратегий
func (m *StrategySimpleBuyMenu) Handle(b *tele.Bot, handler model.MenuHandler) {

	m.b = b
	m.handler = handler
	// Обработчик кнопки входа в меню стратегий
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})

	b.Handle(&btnEnableNotifications, func(c tele.Context) error {
		m.Strategy.Config.NotificationEnable = true
		return c.Respond(&tele.CallbackResponse{Text: "Уведомления включены ✅", ShowAlert: true})
	})

	b.Handle(&btnDisableNotifications, func(c tele.Context) error {
		m.Strategy.Config.NotificationEnable = false
		return c.Respond(&tele.CallbackResponse{Text: "Уведомления отключены ❌", ShowAlert: true})
	})
}
