import { onMounted, ref } from 'vue'
import { useOrdersStore, type Order } from '../stores/orders'
import { useWebSocket } from './useWebSocket'

export function useOrders() {
  const store = useOrdersStore()
  const isInitialized = ref(false)

  async function fetchOrders() {
    try {
      const response = await fetch('/trade/api/getOrders', {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' }
      })
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`)
      const data = await response.json()

      if (data.OrdersActive) {
        const activeOrdersArray = (Object.values(data.OrdersActive).flat() as Order[])
          .filter(o => o.ID > 0)
        store.setActive(activeOrdersArray)
      } else {
        store.setActive([])
      }

      if (data.OrdersHistory) {
        const historyOrdersArray = (Object.values(data.OrdersHistory).flat() as Order[])
          .filter(o => o.ID > 0)
        store.setHistory(historyOrdersArray)
      } else {
        store.setHistory([])
      }

      isInitialized.value = true
    } catch (err) {
      console.error('Error loading orders:', err)
    }
  }

  useWebSocket((data) => {
    if (!isInitialized.value) return
    if (data.orderUpdate) store.updateOrder(data.orderUpdate as Order)
    if (data.orderAdd) store.addOrder(data.orderAdd as Order)
    if (data.orderDelete) store.removeOrder((data.orderDelete as Order).ID)
  })

  onMounted(() => fetchOrders())

  return { refreshOrders: fetchOrders }
}