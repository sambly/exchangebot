<script setup lang="ts">
import { ref, inject, onMounted, type Ref } from 'vue'
import Button from 'primevue/button'
import { useToast } from 'primevue/usetoast'

import { useMarketStore } from '../stores/market'
import { useFiltersStore } from '../stores/filters'
import { useUIStore } from '../stores/ui'
import { useOrders } from '../composables/useOrders.ts'

import DataPanel from './DataPanel/DataPanel.vue'
import HeaderInfo from './Layout/HeaderInfo.vue'
import FilterPanel from './Layout/FilterPanel.vue'
import TradingView  from './TradeViewPanel/TradingViewChart.vue'
import ChartVolume  from './TradeViewPanel/ChartVolume.vue'
import SmartTrade from './TradeViewPanel/SmartTrade.vue'
import OrdersChart from './TradeViewPanel/OrdersChart.vue'
import OrdersActive from './OrdersPanel/OrdersActive.vue'
import OrdersHistory from './OrdersPanel/OrdersHistory.vue'

const PRICE_TABLE_WIDTH = '30%'

const darkMode = inject<Ref<boolean>>('darkMode')!

const market = useMarketStore()
const filters = useFiltersStore()
const ui = useUIStore()
const { refreshOrders } = useOrders()
const toast = useToast()

const isRefreshing = ref(false)

const fetchData = async () => {
  try {
    const response = await fetch('/trade/api/getChPrice')
    if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`)
    const data = await response.json()

    market.setMarketData({
      MarketsStat: data.MarketsStat,
      ChangePrices: data.ChangePrices,
    })

    const pairs = Object.keys(data.ChangePrices || {})
    if (pairs.length > 0 && !pairs.includes(ui.currentPair)) {
      ui.selectPair(pairs[0])
    }
  } catch (err) {
    console.error('Error loading data:', err)
    toast.add({ severity: 'error', summary: 'Ошибка', detail: 'Не удалось загрузить данные', life: 4000 })
  }
}

const refreshData = async () => {
  if (isRefreshing.value) return
  isRefreshing.value = true
  try {
    filters.loadFilters()
    await Promise.all([fetchData(), refreshOrders()])
    toast.add({ severity: 'success', summary: 'Готово', detail: 'Данные обновлены', life: 2000 })
  } finally {
    isRefreshing.value = false
  }
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
          :loading="isRefreshing"
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
              :outlined="ui.activeChart !== 'price'"
              @click="ui.activeChart = 'price'"
            />
            <Button
              label="Volume Chart"
              size="small"
              severity="secondary"
              :outlined="ui.activeChart !== 'volume'"
              @click="ui.activeChart = 'volume'"
            />
            <Button
              label="Trade Console"
              size="small"
              severity="secondary"
              :outlined="ui.activeChart !== 'trade-smart'"
              @click="ui.activeChart = 'trade-smart'"
            />
          </div>

          <div class="trading-workspace-content">

            <div v-show="ui.activeChart === 'price'" class="trading-workspace-slot">
              <TradingView :pair="ui.currentPair" :dark-mode="darkMode" />
            </div>

            <div v-if="ui.activeChart === 'volume'" class="trading-workspace-slot">
              <ChartVolume :pair="ui.currentPair" :dark-mode="darkMode" />
            </div>

            <div v-if="ui.activeChart === 'trade-smart'" class="trading-workspace-slot">
              <SmartTrade />
            </div>

            <div v-if="ui.activeChart === 'orders'" class="trading-workspace-slot">
              <OrdersChart :pair="ui.currentPair" :dark-mode="darkMode" :visible="true" />
            </div>

          </div>

        </div>
      </div>

    </div>

    <div class="orders-bar">
      <div class="orders-bar-content">
        <div v-show="ui.activeOrdersTab === 'active'" class="orders-bar-slot">
          <OrdersActive />
        </div>
        <div v-show="ui.activeOrdersTab === 'history'" class="orders-bar-slot">
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
