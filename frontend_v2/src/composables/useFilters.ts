// composables/useFilters.ts
import { ref, computed, inject, type Ref } from 'vue'
import type { MarketsStat, ChangePrices } from '../types'

const periods = ['1m', '3m', '15m', '1h', '4h', '1d'] as const

export function useFilters() {
  // Состояние фильтров
  const volumeFilter = ref<{ min: number | null; max: number | null }>({ 
    min: null, 
    max: null 
  })
  
  const periodFilters = ref<Record<string, { min: number | null; max: number | null }>>(
    Object.fromEntries(periods.map(p => [p, { min: null, max: null }]))
  )

  // Загрузка из localStorage
  const loadFilters = () => {
    try {
      const saved = localStorage.getItem('filterValues')
      if (saved) {
        const data = JSON.parse(saved)
        if (data.volume) volumeFilter.value = data.volume
        if (data.periods) periodFilters.value = data.periods
      }
    } catch (e) {
      console.error('Error loading filters:', e)
    }
  }

  // Сохранение в localStorage
  const saveFilters = () => {
    localStorage.setItem('filterValues', JSON.stringify({
      volume: volumeFilter.value,
      periods: periodFilters.value
    }))
  }

  // Сброс фильтров
  const resetFilters = () => {
    volumeFilter.value = { min: null, max: null }
    periodFilters.value = Object.fromEntries(periods.map(p => [p, { min: null, max: null }]))
    saveFilters()
  }

  // Логика фильтрации (чистая функция)
  const getFilteredPairs = (marketsStat: MarketsStat, changePrices: ChangePrices) => {
    const result: string[] = []
    
    for (const pair in changePrices) {
      const volume = marketsStat[pair]?.Volume || 0
      let pass = true

      // Volume filter
      if (volumeFilter.value.min !== null && volume < volumeFilter.value.min) pass = false
      if (volumeFilter.value.max !== null && volume > volumeFilter.value.max) pass = false

      // Periods filter
      if (pass) {
        for (const period of periods) {
          const filter = periodFilters.value[period]
          const chValue = changePrices[pair]?.[period]?.ChangePercent
          
          if (chValue === undefined) continue
          
          if (filter.min !== null && chValue < filter.min) {
            pass = false
            break
          }
          if (filter.max !== null && chValue > filter.max) {
            pass = false
            break
          }
        }
      }

      if (pass) result.push(pair)
    }
    
    return result
  }

  return {
    periods,
    volumeFilter,
    periodFilters,
    loadFilters,
    saveFilters,
    resetFilters,
    getFilteredPairs
  }
}