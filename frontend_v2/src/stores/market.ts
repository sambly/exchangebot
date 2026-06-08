import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { MarketsStat, ChangePrices } from '../types'

export const useMarketStore = defineStore('market', () => {
  const marketsStat = ref<MarketsStat>({})
  const changePrices = ref<ChangePrices>({})

  function setMarketData(data: { MarketsStat?: MarketsStat; ChangePrices?: ChangePrices }) {
    if (data.MarketsStat) marketsStat.value = data.MarketsStat
    if (data.ChangePrices) changePrices.value = data.ChangePrices
  }

  return { marketsStat, changePrices, setMarketData }
})
