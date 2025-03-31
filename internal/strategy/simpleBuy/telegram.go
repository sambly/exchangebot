package simplebuy

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	exModel "github.com/sambly/exchangeService/pkg/model"
	dealModel "github.com/sambly/exchangebot/internal/model"
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
	btnsNotify              = []tele.Btn{btnEnableNotifications, btnDisableNotifications}
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

	return menu
}

func (m *StrategySimpleBuyMenu) Show(c tele.Context, handler model.MenuHandler) error {

	userID := c.Sender().ID
	handler.SetCurrentMenu(userID, m.Show, nil)
	handler.DeleteUserMessages(c, userID)

	text := "Настройки Base стратегии:\n"
	if m.Strategy.Config.NotificationEnable {
		text += "Уведомления: включены"
	} else {
		text += "Уведомления: отключены"
	}

	if err := c.Send(text, m.Markup); err != nil {
		return err
	}

	m.InlineMarkup.Inline(m.InlineMarkup.Row(btnsNotify...))

	// Кнопки inline отправляем отдельно
	if len(m.InlineButtons) > 0 {
		if err := c.Send("Выберите действие:", m.InlineMarkup); err != nil {
			return err
		}
	}
	return nil
}

func (m *StrategySimpleBuyMenu) SendMessageBuy(baseResult strbase.StrategyBaseResult) (*exModel.Order, error) {
	uniqueID := uuid.New().String()[:8]
	fmt.Println("Unic - ", uniqueID)
	btn := tele.Btn{
		Unique: "buy_btn_" + uniqueID, // Уникальное название
		Text:   "Купить",
		Data: fmt.Sprintf("%s|%s|%.2f|%s",
			baseResult.Data.Pair,
			baseResult.Data.Period,
			baseResult.Data.ChangePercent,
			uniqueID),
	}

	m.InlineMarkup.Inline(m.InlineMarkup.Row([]tele.Btn{btn}...))

	text := fmt.Sprintf("Будете совершать покупку?\nПара: %s\nПериод: %s\nИзменение: %.2f%%",
		baseResult.Data.Pair, baseResult.Data.Period, baseResult.Data.ChangePercent)

	msg, err := m.b.Send(&tele.User{ID: m.handler.GetUser()}, text, m.InlineMarkup)
	if err != nil {
		return nil, err
	}

	resultChan := make(chan *exModel.Order)
	done := make(chan struct{})

	timer := time.NewTimer(5 * time.Minute)

	// Обработчик кнопки
	handler := func(c tele.Context) error {
		data := c.Callback().Data
		parts := strings.Split(data, "|")

		if len(parts) != 4 || parts[3] != uniqueID {
			return nil
		}

		select {
		case <-done:
			return nil
		default:
			deal := dealModel.Deal{
				Pair:     parts[0],
				SideType: "buy",
				Frame:    parts[1],
				Strategy: "simplebuy",
				Comment:  parts[2],
			}

			order, err := m.Strategy.OrderController.CreateOrderMarket(deal, 1.0)
			if err != nil {
				return c.Respond(&tele.CallbackResponse{Text: "Ошибка создания ордера", ShowAlert: true})
			}

			// Отправляем ордер в канал и завершаем ожидание
			select {
			case resultChan <- order:
			default:
			}
			close(done)

			_ = m.b.Delete(&tele.Message{ID: msg.ID, Chat: &tele.Chat{ID: c.Chat().ID}})
			return c.Respond(&tele.CallbackResponse{Text: "Покупка обработана ✅", ShowAlert: true})
		}
	}

	m.b.Handle(&btn, handler)

	// Горутина для таймаута
	go func() {
		<-timer.C
		close(done)
	}()

	// Ожидание результата либо таймаута
	select {
	case order := <-resultChan:
		timer.Stop()
		return order, nil
	case <-done:
		timer.Stop()
		_ = m.b.Delete(&tele.Message{ID: msg.ID, Chat: &tele.Chat{ID: m.handler.GetUser()}})
		return nil, nil
	}
}

// Handle обрабатывает кнопки меню стратегий
func (m *StrategySimpleBuyMenu) Handle(b *tele.Bot, handler model.MenuHandler) {

	m.handler = handler
	m.b = b
	// Обработчик кнопки входа в меню стратегий
	b.Handle(&m.ButtonsHandler.EntryButton, func(c tele.Context) error {
		return m.Show(c, handler)
	})

	b.Handle(&btnEnableNotifications, func(c tele.Context) error {
		m.Strategy.Config.NotificationEnable = true
		return c.Respond(&tele.CallbackResponse{Text: "Уведомления включены ✅", ShowAlert: true})
	})

	b.Handle(&btnDisableNotifications, func(c tele.Context) error {
		m.Strategy.Config.NotificationEnable = true
		return c.Respond(&tele.CallbackResponse{Text: "Уведомления отключены ❌", ShowAlert: true})
	})
}
