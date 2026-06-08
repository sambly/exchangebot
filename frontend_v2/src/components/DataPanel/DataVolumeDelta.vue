<script setup lang="ts">
import { ref, computed, watch, onMounted, nextTick } from 'vue'
import type { DeltaFast, DeltaEntry } from '../../types'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import { useUIStore } from '../../stores/ui'
import { useFiltersStore } from '../../stores/filters'

interface VolumeData {
  pair: string
  volume: number
  volumeBuy: number
  volumeAsk: number
  trades: number
  tradesBuy: number
  tradesAsk: number
  isFavorite: boolean
}

const ui = useUIStore()
const filters = useFiltersStore()

const frames = ['1m', '3m', '15m', '1h', '4h', '1d'] as const
const activeFrame = ref<string>('15m')
const deltaFast = ref<DeltaFast>({})
const isLoading = ref(false)
const error = ref<string | null>(null)
const tableContainerRef = ref<HTMLElement | null>(null)

async function fetchDelta() {
  isLoading.value = true
  error.value = null

  try {
    const response = await fetch('/trade/api/getChDelta', {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json'
      }
    })

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    const data = await response.json()

    deltaFast.value = data.DeltaFast || {}
  } catch (err) {
    error.value =
      err instanceof Error
        ? err.message
        : 'Ошибка загрузки данных'

    console.error('Error loading delta data:', err)
  } finally {
    isLoading.value = false
  }
}

const displayData = computed(() => {
  const df = deltaFast.value
  const filterList = filters.filteredPairs
  const frame = activeFrame.value

  const result: VolumeData[] = []

  for (const pair of filterList) {
    const entry: DeltaEntry | undefined = df[pair]?.[frame]

    if (!entry) continue

    if (ui.filterMode === 'favorites' && !ui.favoritePairs.has(pair)) {
      continue
    }

    result.push({
      pair: pair.replace(/USDT$/, ''),
      volume: entry.Volume,
      volumeBuy: entry.VolumeBuy,
      volumeAsk: entry.VolumeAsk,
      trades: entry.Trades,
      tradesBuy: entry.TradesBuy,
      tradesAsk: entry.TradesAsk,
      isFavorite: ui.favoritePairs.has(pair)
    })
  }

  result.sort((a, b) => b.volume - a.volume)

  return result
})

function toggleFavorite(pairShort: string) {
  ui.toggleFavorite(pairShort + 'USDT')
}

const onRowClick = (event: any) => {
  ui.selectPair(event.data.pair + 'USDT')
}

const fmt = (val: number) => val.toFixed(2)

const getRowClass = (data: VolumeData) => ({
  'table-row-active': ui.currentPair === data.pair + 'USDT'
})

const ROW_HEIGHT = 41

async function scrollToPair() {
  await nextTick()
  const idx = displayData.value.findIndex(d => d.pair + 'USDT' === ui.currentPair)
  if (idx < 0) return
  const container = (
    tableContainerRef.value?.querySelector('.p-virtualscroller') ||
    tableContainerRef.value?.querySelector('.p-datatable-table-container')
  ) as HTMLElement | null
  if (!container) return
  const firstRow = container.querySelector('tbody tr') as HTMLElement | null
  const rowH = firstRow?.offsetHeight || ROW_HEIGHT
  const offset = idx * rowH - container.clientHeight / 2 + rowH / 2
  container.scrollTo({ top: Math.max(0, offset), behavior: 'auto' })
}

watch(() => ui.currentPair, () => scrollToPair())
watch(displayData, () => scrollToPair())

watch(() => ui.activeDataPanel, async (val) => {
  if (val !== 'volume') return
  await nextTick()
  requestAnimationFrame(() => scrollToPair())
})

onMounted(async () => {
  await fetchDelta()
  scrollToPair()
})
</script>

<template>
  <div class="volume-table-wrapper">

    <!-- Header -->
    <div class="table-header">

      <div class="filter-buttons">
        <Button
          icon="pi pi-heart"
          size="small"
          severity="secondary"
          :outlined="ui.filterMode !== 'favorites'"
          @click="ui.filterMode = ui.filterMode === 'favorites' ? 'all' : 'favorites'"
        />
        <div class="divider" />
        <Button
          label="Цена"
          severity="secondary"
          size="small"
          :outlined="ui.activeDataPanel !== 'price'"
          @click="ui.activeDataPanel = 'price'"
        />
        <Button
          label="Объем"
          severity="secondary"
          size="small"
          :outlined="ui.activeDataPanel !== 'volume'"
          @click="ui.activeDataPanel = 'volume'"
        />
      </div>

      <div class="frame-buttons">
        <Button
          v-for="f in frames"
          :key="f"
          :label="f"
          size="small"
          severity="secondary"
          :outlined="activeFrame !== f"
          @click="activeFrame = f"
        />
      </div>

    </div>

    <!-- Error -->
    <div
      v-if="error"
      class="error-msg"
    >
      {{ error }}
    </div>

    <!-- Table -->
    <div
      v-else
      ref="tableContainerRef"
      class="table-container"
    >
      <DataTable
        :value="displayData"
        :loading="isLoading"
        :scrollable="true"
        scrollHeight="flex"
        :virtualScrollerOptions="{ itemSize: ROW_HEIGHT }"
        class="volume-table"
        dataKey="pair"
        :rowClass="getRowClass"
        @row-click="onRowClick"
      >

        <!-- Favorite -->
        <Column header="" style="width: 3rem">
          <template #body="{ data }">
            <button
              class="favorite-btn"
              :class="{ active: data.isFavorite }"
              @click.stop="toggleFavorite(data.pair)"
            >
              {{ data.isFavorite ? '♥' : '♡' }}
            </button>
          </template>
        </Column>

        <!-- Pair -->
        <Column
          field="pair"
          header="Пара"
          :sortable="true"
          style="min-width: 80px"
        >
          <template #body="{ data }">
            <span class="pair-name">
              {{ data.pair }}
            </span>
          </template>
        </Column>

        <!-- Volume -->
        <Column
          field="volume"
          header="Volume"
          :sortable="true"
          style="min-width: 90px"
        >
          <template #body="{ data }">
            {{ fmt(data.volume) }}
          </template>
        </Column>

        <!-- Volume Buy -->
        <Column
          header="Volume Buy"
          style="min-width: 100px"
        >
          <template #body="{ data }">
            <span class="delta-positive">
              {{ fmt(data.volumeBuy) }}
            </span>
          </template>
        </Column>

        <!-- Volume Ask -->
        <Column
          header="Volume Ask"
          style="min-width: 100px"
        >
          <template #body="{ data }">
            <span class="delta-negative">
              {{ fmt(data.volumeAsk) }}
            </span>
          </template>
        </Column>

        <!-- Trades -->
        <Column
          field="trades"
          header="Trades"
          :sortable="true"
          style="min-width: 80px"
        >
          <template #body="{ data }">
            {{ fmt(data.trades) }}
          </template>
        </Column>

        <!-- Trades Buy -->
        <Column
          header="Trades Buy"
          style="min-width: 100px"
        >
          <template #body="{ data }">
            <span class="delta-positive">
              {{ fmt(data.tradesBuy) }}
            </span>
          </template>
        </Column>

        <!-- Trades Ask -->
        <Column
          header="Trades Ask"
          style="min-width: 100px"
        >
          <template #body="{ data }">
            <span class="delta-negative">
              {{ fmt(data.tradesAsk) }}
            </span>
          </template>
        </Column>

      </DataTable>
    </div>

  </div>
</template>

<style scoped>
.volume-table-wrapper {
  display: flex;
  flex-direction: column;

  height: 100%;

  overflow: hidden;
}

.table-header {
  flex-shrink: 0;

  display: flex;
  justify-content: space-between;
  align-items: center;

  gap: 0.5rem;

  overflow-x: auto;

  white-space: nowrap;

  -webkit-overflow-scrolling: touch;
}

.filter-buttons {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.divider {
  width: 1px;
  height: 1.25rem;
  background: var(--p-content-border-color);
  flex-shrink: 0;
}


.frame-buttons {
  display: flex;
  gap: 0.15rem;
}

.error-msg {
  flex-shrink: 0;

  margin: 1rem;
  padding: 0.75rem;

  border-radius: 6px;

  background: #fee2e2;
  color: #dc2626;
}

.table-container {
  flex: 1;

  min-height: 0;

  overflow: hidden;

  position: relative;

  padding-top: 0.5rem;
}

:deep(.volume-table) {
  height: 100%;
}

:deep(.p-datatable-table-container) {
  height: 100%;
  overflow: auto;
}

:deep(.p-datatable-thead > tr > th) {
  position: sticky;
  top: 0;
  z-index: 1;

  padding: 0.5rem 0.4rem;

  font-weight: 600;
}

:deep(.p-datatable-tbody > tr) {
  cursor: pointer;
}

:deep(.p-datatable-tbody > tr > td) {
  padding: 0.35rem 0.4rem;
}

:deep(.table-row-active > td) {
  background: color-mix(
    in srgb,
    var(--p-primary-color) 12%,
    transparent
  );
}

.favorite-btn {
  background: none;
  border: none;

  padding: 0.15rem;

  font-size: 1.1rem;

  cursor: pointer;

  transition: transform 0.2s;
}

.favorite-btn:hover {
  transform: scale(1.1);
}

.pair-name {
  font-weight: 500;
}

.delta-positive {
  color: var(--p-green-500);
}

.delta-negative {
  color: var(--p-red-500);
}
</style>