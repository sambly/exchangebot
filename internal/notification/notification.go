package notification

import "fmt"

type Notification struct {
	Enable  bool
	Message chan string
}

func NewNotificationService(enable bool) Notification {
	return Notification{Message: make(chan string), Enable: enable}
}

func (n Notification) NotificationWeightPercent(pair, period string, changePercent float64) {
	out := fmt.Sprintf("Цена пары %s\nИзменалсь на %s\nЗа период %s", pair, fmt.Sprint(changePercent), period)
	n.Message <- out
}
