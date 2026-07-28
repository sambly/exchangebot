import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ActiveChart = 'price' | 'volume' | 'orders' | 'depth'
export type ActiveOrdersTab = 'active' | 'history'
export type ActiveDataPanel = 'price' | 'volume' | 'imbalance' | 'strength'
export type FilterMode = 'all' | 'favorites'
export type OrdersBarSize = 'compact' | 'large'
export type MobileTab = 'pairs' | 'chart' | 'trade' | 'orders'

function loadFavoritesFromStorage(): Set<string> {
  try {
    const stored = localStorage.getItem('favoritePairs')
    if (!stored) return new Set()
    const parsed = JSON.parse(stored)
    if (!Array.isArray(parsed)) return new Set()
    return new Set(parsed.filter((p: unknown) => typeof p === 'string'))
  } catch {
    return new Set()
  }
}

export const useUIStore = defineStore('ui', () => {
  const currentPair = ref<string>(localStorage.getItem('currentPair') || 'BTCUSDT')
  const activeChart = ref<ActiveChart>('price')
  const activeOrdersTab = ref<ActiveOrdersTab>('active')
  const selectedOrderId = ref<number | null>(null)
  const activeDataPanel = ref<ActiveDataPanel>('price')
  const filterMode = ref<FilterMode>('all')
  const favoritePairs = ref<Set<string>>(loadFavoritesFromStorage())
  const darkMode = ref<boolean>(localStorage.getItem('darkMode') === 'true')
  const ordersBarSize = ref<OrdersBarSize>('compact')
  const tradeConsoleCollapsed = ref<boolean>(false)
  // На узких экранах панели (пары/график/trade console/ордера) показываются по
  // одной за раз через нижний таб-бар вместо одновременных колонок - см.
  // media query в MainView.vue.
  const mobileTab = ref<MobileTab>('chart')

  function selectPair(pair: string) {
    currentPair.value = pair
    localStorage.setItem('currentPair', pair)
  }

  function toggleDarkMode() {
    darkMode.value = !darkMode.value
    localStorage.setItem('darkMode', String(darkMode.value))
  }

  function toggleOrdersBarSize() {
    ordersBarSize.value = ordersBarSize.value === 'compact' ? 'large' : 'compact'
  }

  function toggleTradeConsole() {
    tradeConsoleCollapsed.value = !tradeConsoleCollapsed.value
  }

  function toggleFavorite(pairFull: string) {
    if (favoritePairs.value.has(pairFull)) {
      favoritePairs.value.delete(pairFull)
    } else {
      favoritePairs.value.add(pairFull)
    }
    localStorage.setItem('favoritePairs', JSON.stringify(Array.from(favoritePairs.value)))
  }

  return {
    currentPair,
    activeChart,
    activeOrdersTab,
    selectedOrderId,
    activeDataPanel,
    filterMode,
    favoritePairs,
    darkMode,
    ordersBarSize,
    tradeConsoleCollapsed,
    mobileTab,
    selectPair,
    toggleFavorite,
    toggleDarkMode,
    toggleOrdersBarSize,
    toggleTradeConsole,
  }
})
