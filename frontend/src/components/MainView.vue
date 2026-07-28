<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import Button from 'primevue/button'
import { useToast } from 'primevue/usetoast'

import { useMarketStore } from '../stores/market'
import { useFiltersStore } from '../stores/filters'
import { useUIStore } from '../stores/ui'
import { useOrders } from '../composables/useOrders.ts'
import { timed } from '../utils/timing'

import DataPanel from './DataPanel/DataPanel.vue'
import HeaderInfo from './Layout/HeaderInfo.vue'
import FilterPanel from './Layout/FilterPanel.vue'
import TradingView  from './TradeViewPanel/TradingViewChart.vue'
import ChartVolume  from './TradeViewPanel/ChartVolume.vue'
import SmartTrade from './TradeViewPanel/SmartTrade.vue'
import OrdersChart from './TradeViewPanel/OrdersChart.vue'
import DepthChart from './TradeViewPanel/DepthChart.vue'
import OrdersActive from './OrdersPanel/OrdersActive.vue'
import OrdersHistory from './OrdersPanel/OrdersHistory.vue'
import StrategiesDialog from './StrategyPanel/StrategiesDialog.vue'

const PRICE_TABLE_WIDTH = '30%'

const market = useMarketStore()
const filters = useFiltersStore()
const ui = useUIStore()
const { refreshOrders } = useOrders()
const toast = useToast()

const isRefreshing = ref(false)
const strategiesDialogVisible = ref(false)

const ordersBarHeight = computed(() => ui.ordersBarSize === 'large' ? '40vh' : '20vh')

const fetchData = async () => {
  try {
    const response = await fetch('/trade/api/getChPrice')
    if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`)
    const data = await response.json()

    market.setMarketData({
      MarketsStat: data.MarketsStat,
      ChangePrices: data.ChangePrices,
      FeedStatus: data.FeedStatus,
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
  const start = performance.now()
  try {
    filters.loadFilters()
    // Каждый запрос замерян отдельно (console.log, [timing]) - Promise.all
    // ждёт САМЫЙ медленный из них, и без разбивки по запросам непонятно, кто
    // именно тормозит кнопку "Обновить".
    await Promise.all([
      timed('getChPrice', fetchData),
      timed('fetchDelta', () => market.fetchDelta()),
      timed('fetchImbalance', () => market.fetchImbalance()),
      timed('fetchQuality', () => market.fetchQuality()),
      timed('refreshOrders', () => refreshOrders()),
    ])
    console.log(`[timing] Обновить (итого): ${(performance.now() - start).toFixed(0)}ms`)
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
          label="Стратегии"
          icon="pi pi-cog"
          size="small"
          severity="secondary"
          outlined
          @click="strategiesDialogVisible = true"
        />
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

    <div class="workspace" :class="{ 'mobile-hidden': ui.mobileTab === 'orders' }">

      <div class="workspace-body">

        <div class="pairs-panel" :class="{ 'mobile-active': ui.mobileTab === 'pairs' }">
          <DataPanel />
        </div>

        <div class="trading-workspace" :class="{ 'mobile-active': ui.mobileTab === 'chart' }">

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
              label="Стакан"
              size="small"
              severity="secondary"
              :outlined="ui.activeChart !== 'depth'"
              @click="ui.activeChart = 'depth'"
            />
          </div>

          <div class="trading-workspace-content">

            <div v-show="ui.activeChart === 'price'" class="trading-workspace-slot">
              <TradingView :pair="ui.currentPair" :dark-mode="ui.darkMode" :visible="ui.activeChart === 'price'" />
            </div>

            <div v-if="ui.activeChart === 'volume'" class="trading-workspace-slot">
              <ChartVolume :pair="ui.currentPair" :dark-mode="ui.darkMode" />
            </div>

            <div v-if="ui.activeChart === 'orders'" class="trading-workspace-slot">
              <OrdersChart :pair="ui.currentPair" :dark-mode="ui.darkMode" :visible="true" />
            </div>

            <div v-if="ui.activeChart === 'depth'" class="trading-workspace-slot">
              <DepthChart :pair="ui.currentPair" :dark-mode="ui.darkMode" />
            </div>

          </div>

        </div>

        <div
          class="trade-console-panel"
          :class="{
            'trade-console-panel-collapsed': ui.tradeConsoleCollapsed,
            'mobile-active': ui.mobileTab === 'trade'
          }"
        >
          <button
            class="trade-console-toggle"
            :title="ui.tradeConsoleCollapsed ? 'Развернуть Trade Console' : 'Свернуть Trade Console'"
            @click="ui.toggleTradeConsole()"
          >
            <i :class="ui.tradeConsoleCollapsed ? 'pi pi-angle-left' : 'pi pi-angle-right'" />
          </button>
          <div v-show="!ui.tradeConsoleCollapsed" class="trade-console-content">
            <SmartTrade />
          </div>
        </div>

      </div>

    </div>

    <div class="orders-bar" :class="{ 'mobile-active': ui.mobileTab === 'orders' }">
      <div class="orders-bar-content">
        <div v-show="ui.activeOrdersTab === 'active'" class="orders-bar-slot">
          <OrdersActive />
        </div>
        <div v-show="ui.activeOrdersTab === 'history'" class="orders-bar-slot">
          <OrdersHistory />
        </div>
      </div>
    </div>

    <div class="mobile-tab-bar">
      <button
        class="mobile-tab-button"
        :class="{ active: ui.mobileTab === 'pairs' }"
        @click="ui.mobileTab = 'pairs'"
      >
        <i class="pi pi-table" />
        <span>Пары</span>
      </button>
      <button
        class="mobile-tab-button"
        :class="{ active: ui.mobileTab === 'chart' }"
        @click="ui.mobileTab = 'chart'"
      >
        <i class="pi pi-chart-line" />
        <span>График</span>
      </button>
      <button
        class="mobile-tab-button"
        :class="{ active: ui.mobileTab === 'trade' }"
        @click="ui.mobileTab = 'trade'"
      >
        <i class="pi pi-send" />
        <span>Trade</span>
      </button>
      <button
        class="mobile-tab-button"
        :class="{ active: ui.mobileTab === 'orders' }"
        @click="ui.mobileTab = 'orders'"
      >
        <i class="pi pi-history" />
        <span>Ордера</span>
      </button>
    </div>

    <StrategiesDialog v-model:visible="strategiesDialogVisible" />

  </div>
</template>

<style scoped>
.main-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;

  gap: 0.5rem;
  padding: 0.5rem;
  box-sizing: border-box;
}

.top-bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  padding: 0.5rem 1rem;
  gap: 0.75rem 1.5rem;

  border: 1px solid var(--p-content-border-color);
  border-radius: 8px;
}

.top-bar-controls {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.mobile-tab-bar {
  display: none;
}

.workspace {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.workspace-body {
  flex: 1;
  min-height: 0;
  display: flex;
  gap: 0.75rem;
  overflow: hidden;
}

.pairs-panel {
  flex: 0 0 v-bind(PRICE_TABLE_WIDTH);
  max-width: v-bind(PRICE_TABLE_WIDTH);
  min-width: 300px;

  padding: 0.75rem;

  border: 1px solid var(--p-content-border-color);
  border-radius: 8px;

  box-sizing: border-box;
  overflow: hidden;
}

.trading-workspace {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;

  padding: 0.75rem;

  border: 1px solid var(--p-content-border-color);
  border-radius: 8px;

  box-sizing: border-box;
}

.trading-workspace-toolbar {
  flex-shrink: 0;
  display: flex;
  flex-wrap: wrap;
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

.trade-console-panel {
  flex-shrink: 0;
  display: flex;

  width: 320px;

  padding: 0.5rem;

  border: 1px solid var(--p-content-border-color);
  border-radius: 8px;

  box-sizing: border-box;
  overflow: hidden;

  transition: width 0.2s ease;
}

.trade-console-panel-collapsed {
  width: 2.25rem;
  padding: 0.5rem 0;
}

.trade-console-toggle {
  flex-shrink: 0;

  width: 1.75rem;

  display: flex;
  align-items: center;
  justify-content: center;

  border: none;
  background: transparent;

  color: var(--p-text-muted-color, #999);
  cursor: pointer;
}

.trade-console-toggle:hover {
  color: var(--p-text-color);
}

.trade-console-content {
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.orders-bar {
  flex-shrink: 0;
  height: v-bind(ordersBarHeight);
  display: flex;
  flex-direction: column;

  padding: 0.5rem;

  border: 1px solid var(--p-content-border-color);
  border-radius: 8px;

  box-sizing: border-box;
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

/* ============================================================
   Мобильная раскладка: на десктопе видны все 4 области сразу
   (пары/график/trade console/ордера), на узком экране это не
   влезает - переключаем их по одной через нижний таб-бар вместо
   одновременных колонок.
   ============================================================ */
@media (max-width: 768px) {
  .main-container {
    height: 100%;
  }

  .mobile-tab-bar {
    flex-shrink: 0;
    display: flex;

    border: 1px solid var(--p-content-border-color);
    border-radius: 8px;

    overflow: hidden;
  }

  .mobile-tab-button {
    flex: 1;

    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.15rem;

    padding: 0.5rem 0.25rem;

    border: none;
    background: transparent;

    font-size: 0.7rem;
    color: var(--p-text-muted-color, #999);

    cursor: pointer;
  }

  .mobile-tab-button.active {
    color: var(--p-primary-color);
    background: color-mix(in srgb, var(--p-primary-color) 10%, transparent);
  }

  .mobile-tab-button i {
    font-size: 1.1rem;
  }

  .workspace-body {
    gap: 0;
  }

  .pairs-panel,
  .trading-workspace,
  .trade-console-panel {
    display: none;
  }

  .pairs-panel.mobile-active,
  .trading-workspace.mobile-active,
  .trade-console-panel.mobile-active {
    display: flex;
    flex-direction: column;

    width: 100%;
    max-width: 100%;
    min-width: 0;
    flex: 1 1 auto;
  }

  /* Сворачивание Trade Console имеет смысл только когда панель - одна из
     нескольких видимых колонок на десктопе. На мобильном она и так на весь
     экран только когда выбрана табом, сворачивать там нечего. */
  .trade-console-toggle {
    display: none;
  }

  .trade-console-content {
    display: block !important;
  }

  .workspace.mobile-hidden {
    display: none;
  }

  .orders-bar {
    display: none;
    height: auto;
  }

  .orders-bar.mobile-active {
    display: flex;
    flex: 1;
  }
}
</style>
