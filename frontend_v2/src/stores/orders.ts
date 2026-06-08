import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export interface Order {
  ID: number
  Pair: string
  Side: 'BUY' | 'SELL'
  PriceCreated?: number
  Price?: number
  Profit?: number
  StrategyBuy?: string
  StrategySell?: string
  TimeCreated?: string
  Time?: string
  Status?: string
}

export const useOrdersStore = defineStore('orders', () => {
  const active = ref<Order[]>([])
  const history = ref<Order[]>([])

  function setActive(orders: Order[]) {
    active.value = Array.isArray(orders) ? orders : []
  }

  function setHistory(orders: Order[]) {
    history.value = Array.isArray(orders) ? orders : []
  }

  function updateOrder(order: Order) {
    const idx = active.value.findIndex(o => o.ID === order.ID)
    if (idx !== -1) {
      active.value[idx] = { ...active.value[idx], ...order }
    }
  }

  function addOrder(order: Order) {
    if (!active.value.some(o => o.ID === order.ID)) {
      active.value.push(order)
    }
  }

  function removeOrder(orderId: number) {
    active.value = active.value.filter(o => o.ID !== orderId)
  }

const sortedActive = computed(() =>
    [...active.value].sort((a, b) => new Date(b.TimeCreated || 0).getTime() - new Date(a.TimeCreated || 0).getTime())
  )

  const sortedHistory = computed(() =>
    [...history.value].sort((a, b) => new Date(b.Time || 0).getTime() - new Date(a.Time || 0).getTime())
  )

  return {
    active,
    history,
    setActive,
    setHistory,
    updateOrder,
    addOrder,
    removeOrder,
    sortedActive,
    sortedHistory,
  }
})