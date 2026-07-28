<script setup lang="ts">
import { computed } from 'vue'
import { useOrdersStore, type Order } from '../../stores/orders.ts'
import { useUIStore } from '../../stores/ui'
import { useToast } from 'primevue/usetoast'
import DataTable, { type DataTableRowClickEvent } from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import OrdersTabSwitcher from './OrdersTabSwitcher.vue'

const store = useOrdersStore()
const ui = useUIStore()
const toast = useToast()

const orderList = computed(() => store.sortedActive)
const pnl = computed(() => orderList.value.reduce((sum, o) => sum + (o.Profit || 0), 0))

function colorSide(side: string) {
  return side === 'BUY' ? 'var(--p-green-500)' : side === 'SELL' ? 'var(--p-red-500)' : 'inherit'
}

function colorProfit(profit?: number) {
  if (profit === undefined || profit === null) return 'inherit'
  return profit > 0 ? 'var(--p-green-500)' : profit < 0 ? 'var(--p-red-500)' : 'inherit'
}

function formatProfit(profit?: number) {
  return profit !== undefined
    ? profit.toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' })
    : '-'
}

function formatTime(timestamp?: string) {
  if (!timestamp) return '-'
  const d = new Date(timestamp)
  return isNaN(d.getTime()) ? '-' : d.toLocaleString('en-GB')
}

function onRowClick(event: DataTableRowClickEvent) {
  const order = event.data as Order
  ui.selectPair(order.Pair)
  ui.selectedOrderId = order.ID
  ui.activeChart = 'orders'
}

async function handleClose(orderId: number) {
  try {
    const res = await fetch('/trade/api/closeDeal', {
      method: 'POST',
      body: String(orderId),
    })
    const data = await res.json()

    // Убираем из активных точечно
    store.removeOrder(orderId)

    if (data.OrdersHistory) {
      const historyOrders = Array.isArray(data.OrdersHistory)
        ? (data.OrdersHistory as Order[])
        : (Object.values(data.OrdersHistory).flat() as Order[])
      store.addToHistory(historyOrders)
    }
    toast.add({ severity: 'success', summary: 'Сделка закрыта', detail: `#${orderId}`, life: 3000 })
  } catch (err) {
    console.error('Error closing order:', err)
    toast.add({ severity: 'error', summary: 'Ошибка', detail: 'Не удалось закрыть сделку', life: 4000 })
  }
}

async function handleCloseAll() {
  if (!confirm('Вы подтверждаете закрытие всех сделок?')) return
  try {
    const res = await fetch('/trade/api/closeAllDeal', { method: 'POST' })
    const data = await res.json()
    // Здесь полный сброс допустим — закрываются ВСЕ
    if (data.OrdersActive) store.setActive([])
    if (data.OrdersHistory) store.setHistory(
      Object.values(data.OrdersHistory).flat() as Order[]
    )
  } catch (err) {
    console.error('Error closing all orders:', err)
  }
}
</script>

<template>
  <div class="orders-wrapper">

    <div class="orders-header">
      <OrdersTabSwitcher v-model="ui.activeOrdersTab" />

      <div class="divider" />

      <div class="stats">
        <span class="stat">Сделок: <strong>{{ orderList.length }}</strong></span>
        <span class="stat">PNL: <strong :style="{ color: colorProfit(pnl) }">{{ formatProfit(pnl) }}</strong></span>
      </div>
   
      <Button
        label="Закрыть все"
        icon="pi pi-times"
        size="small"
        severity="secondary"
        outlined
        @click="handleCloseAll"
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
      :value="orderList"
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
      <Column header="Цена" style="min-width: 80px">
        <template #body="{ data }">{{ data.PriceCreated || '-' }}</template>
      </Column>
      <Column field="Profit" header="Профит" :sortable="true" style="min-width: 80px">
        <template #body="{ data }">
          <span :style="{ color: colorProfit(data.Profit) }">{{ formatProfit(data.Profit) }}</span>
        </template>
      </Column>
      <Column field="StrategyBuy" header="Стратегия" style="min-width: 100px" />
      <Column header="Выход" style="min-width: 100px">
        <template #body="{ data }">
          <span v-if="data.StrategySell" :title="'Позицию отслеживает: ' + data.StrategySell">
            {{ data.StrategySell }}
          </span>
          <span
            v-else
            style="color: var(--p-orange-500)"
            title="Стратегия выхода не подключена: позиция была открыта вручную из веба, либо приложение перезапускалось после её открытия. Тейк/стоп/таймаут по ней автоматически не сработают — закрыть можно только вручную."
          >
            не отслеживается
          </span>
        </template>
      </Column>
      <Column header="Дата" style="min-width: 140px">
        <template #body="{ data }">{{ formatTime(data.TimeCreated) }}</template>
      </Column>
      <Column header="" style="width: 60px">
        <template #body="{ data }">
          <Button icon="pi pi-times" size="small" severity="secondary" text @click.stop="handleClose(data.ID)" />
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

  overflow-x: auto;
  white-space: nowrap;
  -webkit-overflow-scrolling: touch;
}

.orders-header > * {
  flex-shrink: 0;
}

.divider {
  width: 1px;
  height: 20px;
  background: var(--p-content-border-color);
  flex-shrink: 0;
}

.stats {
  display: flex;
  gap: 1rem;
  font-size: 0.85rem;
}

.stat strong {
  margin-left: 0.25rem;
}

:deep(.orders-table) { height: 100%; }
:deep(.p-datatable-wrapper) { height: 100%; overflow: auto; }
:deep(.p-datatable-thead > tr > th) { padding: 0.5rem 0.4rem; font-weight: 600; position: sticky; top: 0; z-index: 1; }
:deep(.p-datatable-tbody > tr) { cursor: pointer; transition: background-color 0.2s; }
:deep(.p-datatable-tbody > tr > td) { padding: 0.35rem 0.4rem; }
</style>