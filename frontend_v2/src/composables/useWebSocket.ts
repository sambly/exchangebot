import { onMounted, onBeforeUnmount } from 'vue'

type MessageHandler = (data: Record<string, unknown>) => void

export function useWebSocket(onMessage: MessageHandler) {
  let socket: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let isDestroyed = false
  let isConnecting = false

  function connect() {
    if (isDestroyed || isConnecting) return
    isConnecting = true

    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const url = `${protocol}//${window.location.host}/trade/ws`
      socket = new WebSocket(url)

      socket.onopen = () => {
        isConnecting = false
      }

      socket.onmessage = (e: MessageEvent) => {
        try {
          const data = JSON.parse(e.data) as Record<string, unknown>
          onMessage(data)
        } catch (err) {
          console.error('WebSocket parse error:', err)
        }
      }

      socket.onerror = () => {
        isConnecting = false
      }

      socket.onclose = () => {
        isConnecting = false
        if (!isDestroyed) {
          reconnectTimer = setTimeout(() => connect(), 3000)
        }
      }
    } catch {
      isConnecting = false
      if (!isDestroyed) {
        reconnectTimer = setTimeout(() => connect(), 3000)
      }
    }
  }

  function disconnect() {
    isDestroyed = true
    if (reconnectTimer) clearTimeout(reconnectTimer)
    if (socket) {
      socket.onclose = null
      socket.close()
      socket = null
    }
  }

  onMounted(() => connect())
  onBeforeUnmount(() => disconnect())

  return { disconnect }
}