<script setup lang="ts">
import { ref, provide, onMounted, computed, inject, type Ref } from 'vue'
import Button from 'primevue/button'

import type { MarketsStat, ChangePrices } from '../types.ts'
import { useFilters } from '../composables/useFilters.ts'
import DataPanel from './DataPanel/DataPanel.vue'
import HeaderInfo from './Layout/HeaderInfo.vue'
import FilterPanel from './Layout/FilterPanel.vue'
import TradingView  from './TradeViewPanel/TradingViewChart.vue'
import ChartVolume  from './TradeViewPanel/ChartVolume.vue'
import SmartTrade from './TradeViewPanel/SmartTrade.vue'
import { useOrders } from '../composables/useOrders.ts'
import OrdersChart from './TradeViewPanel/OrdersChart.vue'
import OrdersActive from './OrdersPanel/OrdersActive.vue'
import OrdersHistory from './OrdersPanel/OrdersHistory.vue'


const PRICE_TABLE_WIDTH = '30%'

const darkMode = inject<Ref<boolean>>('darkMode')!

const marketsStat = ref<MarketsStat>({})
const changePrices = ref<ChangePrices>({})
const currentPair = ref<string>(localStorage.getItem('currentPair') || 'BTCUSDT')
const filters = useFilters()
const filteredPairs = computed(() => 
  filters.getFilteredPairs(marketsStat.value, changePrices.value)
)
const { refreshOrders } = useOrders()


// Управление видимостью компонентов
const activeChart = ref<'price' | 'volume' | 'trade-smart' | 'orders'>('price')
const activeOrdersTab = ref<'active' | 'history'>('active')


provide('marketsStat', marketsStat)
provide('changePrices', changePrices)
provide('currentPair', currentPair)
provide('selectPair', (pair: string) => {
  currentPair.value = pair
  localStorage.setItem('currentPair', pair)
})
provide('activeChart', activeChart)
provide('activeOrdersTab', activeOrdersTab)
// Фильтры
provide('volumeFilter', filters.volumeFilter)
provide('periodFilters', filters.periodFilters)
provide('periods', filters.periods)
provide('resetFilters', filters.resetFilters)
provide('filteredPairs', filteredPairs)
provide('saveFilters', filters.saveFilters)

const fetchData = async () => {
  try {
    const response = await fetch('/trade/api/getChPrice', {
      method: 'GET',
      headers: { 'Content-Type': 'application/json' }
    })
    if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`)
    const data = await response.json()
    
    console.log('Data fetched:', data)

    marketsStat.value = data.MarketsStat || {}
    changePrices.value = data.ChangePrices || {}

    const pairs = Object.keys(changePrices.value)
    if (pairs.length > 0 && !pairs.includes(currentPair.value)) {
      currentPair.value = pairs[0]
      localStorage.setItem('currentPair', currentPair.value)
    }
  } catch (err) {
    console.error('Error loading data:', err)
  }
}

const refreshData = () => {
  filters.loadFilters()
  fetchData()
  refreshOrders()
}

onMounted(() => {
  filters.loadFilters()
  fetchData()
})
</script>

<template>
  <div class="main-container">
    
    <div class="top-bar">
      <HeaderInfo />
      <div class="top-bar-controls">
        <FilterPanel />
        <Button
          label="Обновить"
          icon="pi pi-refresh"
          size="small"
          severity="secondary"
          outlined
          @click="refreshData"
        />
      </div>
    </div>

    <div class="workspace">
      
      <div class="workspace-body">
        
        <div class="pairs-panel">
          <DataPanel />
        </div>

        <div class="trading-workspace">

          <div class="trading-workspace-toolbar">
            <Button
              label="Price Chart"
              size="small"
              severity="secondary"
              :outlined="activeChart !== 'price'"
              @click="activeChart = 'price'"
            />
            <Button
              label="Volume Chart"
              size="small"
              severity="secondary"
              :outlined="activeChart !== 'volume'"
              @click="activeChart = 'volume'"
            />
            <Button
              label="Trade Console"
              size="small"
              severity="secondary"
              :outlined="activeChart !== 'trade-smart'"
              @click="activeChart = 'trade-smart'"
            />
          </div>

          <div class="trading-workspace-content">
            
            <div v-show="activeChart === 'price'" class="trading-workspace-slot">
              <TradingView :pair="currentPair" :dark-mode="darkMode" />
            </div>

            <div v-show="activeChart === 'volume'" class="trading-workspace-slot">
              <ChartVolume :pair="currentPair" :dark-mode="darkMode" />
            </div>

            <div v-show="activeChart === 'trade-smart'" class="trading-workspace-slot">
              <SmartTrade />
            </div>

            <div v-show="activeChart === 'orders'" class="trading-workspace-slot">
              <OrdersChart :pair="currentPair" :dark-mode="darkMode" :visible="activeChart === 'orders'" />
            </div>

          </div>

        </div>
      </div>

    </div>

    <div class="orders-bar">
      <div class="orders-bar-content">
        <div v-show="activeOrdersTab === 'active'" class="orders-bar-slot">
          <OrdersActive />
        </div>
        <div v-show="activeOrdersTab === 'history'" class="orders-bar-slot">
          <OrdersHistory />
        </div>
      </div>
    </div>

  </div>
</template>

<style scoped>
.main-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

.top-bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  padding: 0.5rem 1rem;
  gap: 1.5rem;
}

.top-bar-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.workspace {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 0.5rem 1rem;
}

.workspace-body {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow: hidden;
}

.pairs-panel {
  flex: 0 0 v-bind(PRICE_TABLE_WIDTH);
  max-width: v-bind(PRICE_TABLE_WIDTH);
  min-width: 300px;
  overflow: hidden;
}

.trading-workspace {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding-left: 1rem;
}

.trading-workspace-toolbar {
  flex-shrink: 0;
  display: flex;
  gap: 0.5rem;
  padding-bottom: 0.5rem;
}

.trading-workspace-content {
  flex: 1;
  min-height: 0;
  min-width: 0;
  position: relative;
  overflow: hidden;
}

.trading-workspace-slot {
  width: 100%;
  height: 100%;
}

.orders-bar {
  flex-shrink: 0;
  height: 220px;
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--p-content-border-color);
  overflow: hidden;
}

.orders-bar-content {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.orders-bar-slot {
  width: 100%;
  height: 100%;
  overflow: hidden;
}
</style>