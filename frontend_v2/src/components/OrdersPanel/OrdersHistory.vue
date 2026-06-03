<script setup lang="ts">
import { computed, ref, inject, type Ref } from 'vue'
import { useOrdersStore } from '../../stores/orders.ts'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Select from 'primevue/select'
import Button from 'primevue/button'
import OrdersTabSwitcher from './OrdersTabSwitcher.vue'


const store = useOrdersStore()

const selectPair = inject<(pair: string) => void>('selectPair')!
const activeOrdersTab = inject<Ref<'active' | 'history'>>('activeOrdersTab')!
const activeChart = inject<Ref<'price' | 'volume' | 'trade-smart' | 'orders'>>('activeChart')!

const selectedPair = ref('')
const selectedInterval = ref('all')
const selectedStrategyBuy = ref('')
const selectedStrategySell = ref('')

const orderList = computed(() => store.sortedHistory)

const strategyBuyOptions = computed(() => {
  const s = new Set(orderList.value.map(o => o.StrategyBuy).filter(Boolean))
  return Array.from(s).sort()
})
const strategySellOptions = computed(() => {
  const s = new Set(orderList.value.map(o => o.StrategySell).filter(Boolean))
  return Array.from(s).sort()
})
const pairOptions = computed(() => {
  const s = new Set(orderList.value.map(o => o.Pair).filter(Boolean))
  return Array.from(s).sort()
})

const filteredOrders = computed(() => {
  let f = orderList.value
  if (selectedPair.value) f = f.filter(o => o.Pair === selectedPair.value)
  if (selectedInterval.value !== 'all') {
    const now = Date.now()
    const ms: Record<string, number> = { '1d': 86400000, '7d': 604800000, '30d': 2592000000 }
    const limit = ms[selectedInterval.value]
    f = f.filter(o => {
      const t = new Date(o.Time || 0).getTime()
      return !isNaN(t) && now - t <= limit
    })
  }
  if (selectedStrategyBuy.value) f = f.filter(o => o.StrategyBuy === selectedStrategyBuy.value)
  if (selectedStrategySell.value) f = f.filter(o => o.StrategySell === selectedStrategySell.value)
  return f
})

const totalOrders = computed(() => filteredOrders.value.length)
const totalProfit = computed(() => filteredOrders.value.reduce((s, o) => s + (o.Profit || 0), 0))

function resetFilters() {
  selectedPair.value = ''
  selectedInterval.value = 'all'
  selectedStrategyBuy.value = ''
  selectedStrategySell.value = ''
}

function onRowClick(event: any) {
  selectPair(event.data.Pair)
  activeChart.value = 'orders'
}

function colorSide(side: string) {
  return side === 'BUY' ? 'var(--p-green-500)' : side === 'SELL' ? 'var(--p-red-500)' : 'inherit'
}
function colorProfit(profit?: number) {
  if (profit === undefined) return 'inherit'
  return profit > 0 ? 'var(--p-green-500)' : profit < 0 ? 'var(--p-red-500)' : 'inherit'
}
function formatProfit(profit?: number) {
  return profit !== undefined
    ? profit.toLocaleString('ru', { maximumFractionDigits: 2, minimumFractionDigits: 2, notation: 'compact' })
    : '-'
}
function formatTime(timestamp?: string) {
  if (!timestamp) return '-'
  const d = new Date(timestamp)
  return isNaN(d.getTime()) ? '-' : d.toLocaleString('en-GB')
}
</script>

<template>
  <div class="orders-wrapper">

    <div class="orders-header">
      <OrdersTabSwitcher v-model="activeOrdersTab" />

      <div class="divider" />

      <div class="stats">
        <span class="stat">Сделок: <strong>{{ totalOrders.toLocaleString('ru') }}</strong></span>
        <span class="stat">PNL: <strong :style="{ color: colorProfit(totalProfit) }">{{ formatProfit(totalProfit) }}</strong></span>
      </div>

      <div class="divider" />

      <div class="filters-group">
        <Select v-model="selectedPair" :options="pairOptions" placeholder="Пара" :filter="true" size="small" class="filter-select" />
        <Select v-model="selectedInterval" :options="['all', '1d', '7d', '30d']" placeholder="Интервал" size="small" class="filter-select-small">
          <template #value="{ value }">{{ value === 'all' ? 'За всё время' : `За ${value}` }}</template>
        </Select>
        <Select v-model="selectedStrategyBuy" :options="strategyBuyOptions" placeholder="Стратегия покупки" :filter="true" size="small" class="filter-select" />
        <Select v-model="selectedStrategySell" :options="strategySellOptions" placeholder="Стратегия продажи" :filter="true" size="small" class="filter-select" />
        <Button label="Сброс" size="small" severity="secondary" text @click="resetFilters" />
      </div>  
    </div>

    <DataTable
      :value="filteredOrders"
      :scrollable="true"
      scrollHeight="flex"
      class="orders-table"
      dataKey="ID"
      @row-click="onRowClick"
    >
      <Column header="Тип" style="min-width: 60px">
        <template #body="{ data }">
          <span :style="{ color: colorSide(data.Side), fontWeight: 600 }">{{ data.Side }}</span>
        </template>
      </Column>
      <Column field="Pair" header="Пара" :sortable="true" style="min-width: 100px" />
      <Column header="Цена" style="min-width: 100px">
        <template #body="{ data }">
          <div>{{ data.PriceCreated || '-' }}</div>
          <div style="font-size:0.8em; opacity:0.7">{{ data.Price || '-' }}</div>
        </template>
      </Column>
      <Column header="Дата" style="min-width: 140px">
        <template #body="{ data }">
          <div>{{ formatTime(data.TimeCreated) }}</div>
          <div style="font-size:0.8em; opacity:0.7">{{ formatTime(data.Time) }}</div>
        </template>
      </Column>
      <Column header="Профит" :sortable="true" style="min-width: 80px">
        <template #body="{ data }">
          <span :style="{ color: colorProfit(data.Profit) }">{{ formatProfit(data.Profit) }}</span>
        </template>
      </Column>
      <Column header="Стратегия" style="min-width: 120px">
        <template #body="{ data }">
          <div>{{ data.StrategyBuy }}</div>
          <div style="font-size:0.8em; opacity:0.7">{{ data.StrategySell }}</div>
        </template>
      </Column>
    </DataTable>
  </div>
</template>

<style scoped>
.orders-wrapper {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.orders-header {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 1rem;
}

.divider {
  width: 1px;
  height: 20px;
  background: var(--p-content-border-color);
  flex-shrink: 0;
}

.filters-group {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.stats {
  display: flex;
  gap: 1rem;
  font-size: 0.85rem;
}
.stat strong { margin-left: 0.25rem; }

.filter-select { width: 220px; }
.filter-select-small { width: 160px; }

:deep(.orders-table) { height: 100%; }
:deep(.p-datatable-wrapper) { height: 100%; overflow: auto; }
:deep(.p-datatable-thead > tr > th) { padding: 0.5rem 0.4rem; font-weight: 600; position: sticky; top: 0; z-index: 1; }
:deep(.p-datatable-tbody > tr) { cursor: pointer; transition: background-color 0.2s; }
:deep(.p-datatable-tbody > tr > td) { padding: 0.35rem 0.4rem; }
</style>