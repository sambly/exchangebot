<script setup lang="ts">
import {
  ref,
  computed,
  inject,
  watch,
  onMounted,
  nextTick,
  type Ref
} from 'vue'

import type { DeltaFast, DeltaEntry } from '../../types'

import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'

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


const filteredPairs = inject<Ref<string[]>>('filteredPairs')!
const currentPair = inject<Ref<string>>('currentPair')!
const selectPair = inject<(pair: string) => void>('selectPair')!
const activeComponent = inject<Ref<string>>('activeComponent')!
const filterMode = inject<Ref<'all' | 'favorites'>>('filterMode')!

const frames = ['1m', '3m', '15m', '1h', '4h', '1d'] as const

const activeFrame = ref<string>('15m')

const deltaFast = ref<DeltaFast>({})

const isLoading = ref(false)
const error = ref<string | null>(null)

const favoritePairs = ref<Set<string>>(new Set())

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
  const filterList = filteredPairs.value
  const frame = activeFrame.value

  const result: VolumeData[] = []

  for (const pair of filterList) {
    const entry: DeltaEntry | undefined =
      df[pair]?.[frame]

    if (!entry) continue

    if (
      filterMode.value === 'favorites' &&
      !favoritePairs.value.has(pair)
    ) {
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
      isFavorite: favoritePairs.value.has(pair)
    })
  }

  result.sort((a, b) => b.volume - a.volume)

  return result
})

function loadFavorites() {
  const stored = localStorage.getItem('favoritePairs')

  if (!stored) return

  try {
    favoritePairs.value = new Set(JSON.parse(stored))
  } catch {
    favoritePairs.value = new Set()
  }
}

function saveFavorites() {
  localStorage.setItem(
    'favoritePairs',
    JSON.stringify(Array.from(favoritePairs.value))
  )
}

function toggleFavorite(pairShort: string) {
  const pairFull = pairShort + 'USDT'

  if (favoritePairs.value.has(pairFull)) {
    favoritePairs.value.delete(pairFull)
  } else {
    favoritePairs.value.add(pairFull)
  }

  saveFavorites()
}

const onRowClick = (event: any) => {
  selectPair(event.data.pair + 'USDT')
}

const fmt = (val: number) => val.toFixed(2)

const getRowClass = (data: VolumeData) => {
  return {
    'table-row-active':
      currentPair.value === data.pair + 'USDT'
  }
}

async function scrollToPair() {
  await nextTick()

  const container = tableContainerRef.value?.querySelector(
    '.p-datatable-table-container'
  ) as HTMLElement | null

  if (!container) return

  const activeRow = container.querySelector(
    '.table-row-active'
  ) as HTMLElement | null

  if (!activeRow) return

  const containerRect =
    container.getBoundingClientRect()

  const rowRect =
    activeRow.getBoundingClientRect()

  const scrollOffset =
    rowRect.top -
    containerRect.top +
    container.scrollTop -
    container.clientHeight / 2 +
    rowRect.height / 2

  container.scrollTo({
    top: scrollOffset,
    behavior: 'auto'
  })
}

watch(
  [currentPair, displayData],
  () => {
    scrollToPair()
  },
  { deep: true }
)

watch(activeComponent, async (val) => {
  if (val !== 'volume') return

  await nextTick()

  requestAnimationFrame(() => {
    scrollToPair()
  })
})

onMounted(async () => {
  loadFavorites()

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
          :outlined="filterMode !== 'favorites'"
          @click="filterMode = filterMode === 'favorites' ? 'all' : 'favorites'"
        />
        <div class="divider" />
        <Button
          label="Цена"
          severity="secondary"
          size="small"
          :outlined="activeComponent !== 'price'"
          @click="activeComponent = 'price'"
        />
        <Button
          label="Объем"
          severity="secondary"
          size="small"
          :outlined="activeComponent !== 'volume'"
          @click="activeComponent = 'volume'"
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

  transition: background-color 0.2s;
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