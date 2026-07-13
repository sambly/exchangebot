package notification

// socketsBuffer - запас сообщений, который переживает короткие всплески
// (открытие/закрытие пачки ордеров, медленный клиент).
const socketsBuffer = 1024

type SocketsMessage struct {
	Message chan []byte
}

func NewSocketsMessage() *SocketsMessage {
	return &SocketsMessage{
		Message: make(chan []byte, socketsBuffer),
	}
}

// SendData кладёт сообщение в очередь веб-сокетов НЕблокирующе.
//
// Вызывается в том числе из OrderService.OnMarket, а тот исполняется внутри
// горутины, читающей WebSocket биржи (observer'ы в exchangeService зовутся
// синхронно). Блокировка здесь означала бы остановку чтения котировок, поэтому
// при переполнении очереди сообщение выбрасывается: обновления интерфейса
// эфемерны, поток рыночных данных - нет.
func (n SocketsMessage) SendData(data []byte) {
	select {
	case n.Message <- data:
	default:
		notifyLogger.Warn("очередь веб-сокетов переполнена, сообщение отброшено")
	}
}
