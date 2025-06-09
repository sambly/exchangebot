package notification

import (
	"context"
	"fmt"
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
		Message:  make(chan string),
		Services: make([]Notifier, 0),
	}
}

func (n *Notification) AddService(service Notifier) {
	n.Services = append(n.Services, service)
}
func (n *Notification) Start(ctx context.Context) error {
	for {
		select {
		case msg, ok := <-n.Message:
			if !ok {
				return fmt.Errorf("message channel Notification was unexpectedly closed")
			}
			if n.Enable {
				for _, service := range n.Services {
					service.Send(msg)
				}
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (n *Notification) SendMessage(message string) {
	n.Message <- message
}
