import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ActiveChart = 'price' | 'volume' | 'trade-smart' | 'orders' | 'depth'
export type ActiveOrdersTab = 'active' | 'history'
export type ActiveDataPanel = 'price' | 'volume' | 'imbalance'
export type FilterMode = 'all' | 'favorites'
export type OrdersBarSize = 'compact' | 'large'

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
  const activeDataPanel = ref<ActiveDataPanel>('price')
  const filterMode = ref<FilterMode>('all')
  const favoritePairs = ref<Set<string>>(loadFavoritesFromStorage())
  const darkMode = ref<boolean>(localStorage.getItem('darkMode') === 'true')
  const ordersBarSize = ref<OrdersBarSize>('compact')

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
    activeDataPanel,
    filterMode,
    favoritePairs,
    darkMode,
    ordersBarSize,
    selectPair,
    toggleFavorite,
    toggleDarkMode,
    toggleOrdersBarSize,
  }
})
