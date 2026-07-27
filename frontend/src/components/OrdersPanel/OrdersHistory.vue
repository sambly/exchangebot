<script setup lang="ts">
import { computed, ref } from 'vue'
import { useOrdersStore, type Order } from '../../stores/orders.ts'
import { useUIStore } from '../../stores/ui'
import { useToast } from 'primevue/usetoast'
import DataTable, { type DataTableRowClickEvent } from 'primevue/datatable'
import Column from 'primevue/column'
import Select from 'primevue/select'
import Button from 'primevue/button'
import OrdersTabSwitcher from './OrdersTabSwitcher.vue'

const store = useOrdersStore()
const ui = useUIStore()
const toast = useToast()

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

function onRowClick(event: DataTableRowClickEvent) {
  ui.selectPair((event.data as Order).Pair)
  ui.activeChart = 'orders'
}

function colorSide(side: string) {
  return side === 'BUY' ? 'var(--p-green-500)' : side === 'SELL' ? 'var(--p-red-500)' : 'inherit'
}
function colorProfit(profit?: number) {
  if (profit === undefined) return 'inherit'
  return profit > 0 ? 'var(--p-green-500)' : profit < 0 ? 'var(--p-red-500)' : 'inherit'
}

// Причина выхода: сработал план (take-profit), выбило стопом (stop-loss),
// сигнал протух (timeout) или закрыли руками.
const EXIT_REASON_LABELS: Record<string, string> = {
  'take-profit': 'Тейк',
  'stop-loss': 'Стоп',
  timeout: 'Таймаут',
  manual: 'Вручную'
}
function exitReasonLabel(reason?: string) {
  if (!reason) return '-'
  return EXIT_REASON_LABELS[reason] ?? reason
}
function colorExitReason(reason?: string) {
  if (reason === 'take-profit') return 'var(--p-green-500)'
  if (reason === 'stop-loss') return 'var(--p-red-500)'
  return 'inherit'
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

async function handleDelete(orderId: number) {
  try {
    const res = await fetch('/trade/api/deleteHistoryOrder', {
      method: 'POST',
      body: String(orderId),
    })
    if (!res.ok) throw new Error(await res.text())

    store.removeFromHistory(orderId)
    toast.add({ severity: 'success', summary: 'Сделка удалена', detail: `#${orderId}`, life: 3000 })
  } catch (err) {
    console.error('Error deleting order:', err)
    toast.add({ severity: 'error', summary: 'Ошибка', detail: 'Не удалось удалить сделку', life: 4000 })
  }
}

async function handleDeleteAll() {
  if (!confirm('Вы подтверждаете удаление ВСЕЙ истории сделок? Это действие необратимо.')) return
  try {
    const res = await fetch('/trade/api/deleteAllHistoryOrders', { method: 'POST' })
    if (!res.ok) throw new Error(await res.text())

    store.clearHistory()
    toast.add({ severity: 'success', summary: 'История сделок удалена', life: 3000 })
  } catch (err) {
    console.error('Error deleting all history:', err)
    toast.add({ severity: 'error', summary: 'Ошибка', detail: 'Не удалось удалить историю сделок', life: 4000 })
  }
}
</script>

<template>
  <div class="orders-wrapper">

    <div class="orders-header">
      <OrdersTabSwitcher v-model="ui.activeOrdersTab" />

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
      <Button
        label="Удалить все"
        icon="pi pi-trash"
        size="small"
        severity="danger"
        outlined
        @click="handleDeleteAll"
      />
      <Button
        :icon="ui.ordersBarSize === 'large' ? 'pi pi-window-minimize' : 'pi pi-window-maximize'"
        size="small"
        severity="secondary"
        text
        style="margin-left: auto"
        @click="ui.toggleOrdersBarSize()"
      />
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
      <Column field="Pair" header="Пара" :sortable="true" style="min-width: 100px">
        <template #body="{ data }">
          <div>{{ data.Pair }}</div>
          <div style="font-size:0.8em; opacity:0.7">#{{ data.ID }}</div>
        </template>
      </Column>
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
      <Column field="Profit" header="Профит" :sortable="true" style="min-width: 80px">
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
      <!-- Причина выхода: по ней видно, сработал план или выбило стопом.
           Без неё нельзя понять, работает стратегия или нет. -->
      <Column header="Выход" :sortable="true" field="ExitReason" style="min-width: 110px">
        <template #body="{ data }">
          <span :style="{ color: colorExitReason(data.ExitReason), fontWeight: 500 }">
            {{ exitReasonLabel(data.ExitReason) }}
          </span>
          <div v-if="data.Executor" style="font-size:0.8em; opacity:0.7">{{ data.Executor }}</div>
        </template>
      </Column>
      <Column header="" style="width: 60px">
        <template #body="{ data }">
          <Button icon="pi pi-trash" size="small" severity="danger" text @click.stop="handleDelete(data.ID)" />
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