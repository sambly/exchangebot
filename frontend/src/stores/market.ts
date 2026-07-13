import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { MarketsStat, ChangePrices, DeltaFast } from '../types'

export const useMarketStore = defineStore('market', () => {
  const marketsStat = ref<MarketsStat>({})
  const changePrices = ref<ChangePrices>({})
  const deltaFast = ref<DeltaFast>({})

  const isDeltaLoading = ref(false)
  const deltaError = ref<string | null>(null)

  function setMarketData(data: { MarketsStat?: MarketsStat; ChangePrices?: ChangePrices }) {
    if (data.MarketsStat) marketsStat.value = data.MarketsStat
    if (data.ChangePrices) changePrices.value = data.ChangePrices
  }

  async function fetchDelta() {
    isDeltaLoading.value = true
    deltaError.value = null

    try {
      const response = await fetch('/trade/api/getChDelta', {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
      })
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`)

      const data = await response.json()
      deltaFast.value = data.DeltaFast || {}
    } catch (err) {
      deltaError.value = err instanceof Error ? err.message : 'Ошибка загрузки данных'
      console.error('Error loading delta data:', err)
    } finally {
      isDeltaLoading.value = false
    }
  }

  return {
    marketsStat,
    changePrices,
    deltaFast,
    isDeltaLoading,
    deltaError,
    setMarketData,
    fetchDelta,
  }
})
