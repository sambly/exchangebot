package notification

import (
	"context"
	"fmt"

	"github.com/sambly/exchangebot/internal/logger"
)

var notifyLogger = logger.AddFields(map[string]interface{}{
	"package": "notification",
})

const (
	// messageBuffer - очередь шины. Раньше канал был небуферизованным, и
	// стратегия вставала на отправке до тех пор, пока Telegram не ответит.
	messageBuffer = 256
	// serviceBuffer - очередь на каждого получателя (Telegram и т.п.)
	serviceBuffer = 128
)

type Notifier interface {
	Send(message string)
}

type Notification struct {
	Enable   bool
	Message  chan string
	Services []Notifier
}

func NewNotificationService(enable bool) *Notification {
	return &Notification{
		Enable:   enable,
		Message:  make(chan string, messageBuffer),
		Services: make([]Notifier, 0),
	}
}

func (n *Notification) AddService(service Notifier) {
	n.Services = append(n.Services, service)
}

// Start раздаёт сообщения получателям.
//
// У каждого получателя своя горутина и своя очередь: Send у Telegram - это
// сетевой вызов, и выполнять его в общем цикле значило бы, что медленный ответ
// Telegram API держит всю шину, а через неё - горутины стратегий.
func (n *Notification) Start(ctx context.Context) error {

	queues := make([]chan string, 0, len(n.Services))

	for _, service := range n.Services {
		queue := make(chan string, serviceBuffer)
		queues = append(queues, queue)

		go func(service Notifier, queue chan string) {
			for {
				select {
				case msg, ok := <-queue:
					if !ok {
						return
					}
					service.Send(msg)
				case <-ctx.Done():
					return
				}
			}
		}(service, queue)
	}

	for {
		select {
		case msg, ok := <-n.Message:
			if !ok {
				return fmt.Errorf("message channel Notification was unexpectedly closed")
			}
			if !n.Enable {
				continue
			}

			for _, queue := range queues {
				select {
				case queue <- msg:
				default:
					// Получатель не справляется. Уведомление - не тот груз,
					// ради которого стоит тормозить стратегию.
					notifyLogger.Warn("очередь уведомлений переполнена, сообщение отброшено")
				}
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// SendMessage кладёт сообщение в шину. Не блокирует: при переполнении очереди
// сообщение отбрасывается, а не останавливает вызывающую горутину.
func (n *Notification) SendMessage(message string) {
	select {
	case n.Message <- message:
	default:
		notifyLogger.Warn("шина уведомлений переполнена, сообщение отброшено")
	}
}
