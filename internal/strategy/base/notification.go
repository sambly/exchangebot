package base

import "fmt"

func (str StrategyBase) NotificationWeightPercent(pair, period string, changePercent float64) {
	out := fmt.Sprintf("Цена пары %s\nИзменалсь на %s\nЗа период %s", pair, fmt.Sprint(changePercent), period)
	str.Notification.Message <- out
}
