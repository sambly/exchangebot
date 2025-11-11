package simpleindicator

import "fmt"

func (str Strategy) Notify(signalBuy, signalSell bool, pair, period string) {
	if !signalBuy && !signalSell {
		return
	}
	if signalBuy && signalSell {
		fmt.Println("error signalBuy && signalSell = 1")
		return
	}

	var signal string
	if signalBuy {
		signal = "ПОКУПКА"
	} else if signalSell {
		signal = "ПРОДАЖА"
	}

	out := fmt.Sprintf("Получен сигнал %s\nПара %s\nПериод %s\n", signal, pair, period)
	out += fmt.Sprintf("🔍 %s?pair=%s&strategy=%s&period=%s", str.config.HostWeb, pair, str.config.IDName, period)
	str.notification.Message <- out
}
