import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { useMarketStore } from './market'

export const PERIODS = ['1m', '3m', '15m', '1h', '4h', '12h'] as const
export type Period = typeof PERIODS[number]

type PeriodFilterMap = Record<Period, { min: number | null; max: number | null }>

function emptyPeriodFilters(): PeriodFilterMap {
  return Object.fromEntries(PERIODS.map(p => [p, { min: null, max: null }])) as PeriodFilterMap
}

export const useFiltersStore = defineStore('filters', () => {
  const market = useMarketStore()

  const volumeFilter = ref<{ min: number | null; max: number | null }>({ min: null, max: null })
  const periodFilters = ref<PeriodFilterMap>(emptyPeriodFilters())

  const filteredPairs = computed(() => {
    const result: string[] = []
    const ms = market.marketsStat
    const cp = market.changePrices

    for (const pair in cp) {
      const volume = ms[pair]?.Volume || 0
      let pass = true

      if (volumeFilter.value.min !== null && volume < volumeFilter.value.min) pass = false
      if (volumeFilter.value.max !== null && volume > volumeFilter.value.max) pass = false

      if (pass) {
        for (const period of PERIODS) {
          const filter = periodFilters.value[period]
          const chValue = cp[pair]?.[period]?.ChangePercent
          if (chValue === undefined) continue
          if (filter.min !== null && chValue < filter.min) { pass = false; break }
          if (filter.max !== null && chValue > filter.max) { pass = false; break }
        }
      }

      if (pass) result.push(pair)
    }
    return result
  })

  function loadFilters() {
    try {
      const saved = localStorage.getItem('filterValues')
      if (!saved) return
      const data = JSON.parse(saved)
      if (data.volume) volumeFilter.value = data.volume
      if (data.periods) periodFilters.value = data.periods
    } catch {
      // ignore corrupt data
    }
  }

  function saveFilters() {
    localStorage.setItem('filterValues', JSON.stringify({
      volume: volumeFilter.value,
      periods: periodFilters.value,
    }))
  }

  function resetFilters() {
    volumeFilter.value = { min: null, max: null }
    periodFilters.value = emptyPeriodFilters()
    saveFilters()
  }

  return {
    periods: PERIODS,
    volumeFilter,
    periodFilters,
    filteredPairs,
    loadFilters,
    saveFilters,
    resetFilters,
  }
})
