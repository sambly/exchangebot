package notification

type SocketsMessage struct {
	Message chan []byte
}

func (n SocketsMessage) SendData(data []byte) {
	n.Message <- data

}
