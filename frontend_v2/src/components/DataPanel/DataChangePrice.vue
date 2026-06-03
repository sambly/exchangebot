<script setup lang="ts">
import { ref, computed, inject, watch, nextTick, type Ref } from 'vue'
import type { MarketsStat, ChangePrices } from '../../types'

import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'

interface PriceData {
  pair: string
  volume: number
  ch24: number
  '1m': number
  '3m': number
  '15m': number
  '1h': number
  '4h': number
  '1d': number
  isFavorite: boolean
}

// Inject
const marketsStat = inject<Ref<MarketsStat>>('marketsStat')!
const changePrices = inject<Ref<ChangePrices>>('changePrices')!
const filteredPairs = inject<Ref<string[]>>('filteredPairs')!
const currentPair = inject<Ref<string>>('currentPair')!
const selectPair = inject<(pair: string) => void>('selectPair')!
const activeComponent = inject<Ref<string>>('activeComponent')!
const filterMode = inject<Ref<'all' | 'favorites'>>('filterMode')!

const favoritePairs = ref<Set<string>>(new Set())

const tableContainerRef = ref<HTMLElement | null>(null)

const TIME_PERIODS = ['1m', '3m', '15m', '1h', '4h', '1d'] as const

// Построение списка пар
const pairs = computed(() => {
  const ms = marketsStat.value || {}
  const cp = changePrices.value || {}
  const filterList = filteredPairs.value

  const result: PriceData[] = []

  for (const pair of filterList) {
    result.push({
      pair: pair.replace(/USDT$/, ''),
      volume: ms[pair]?.Volume || 0,
      ch24: ms[pair]?.Ch24 || 0,
      '1m': cp[pair]?.['1m']?.ChangePercent || 0,
      '3m': cp[pair]?.['3m']?.ChangePercent || 0,
      '15m': cp[pair]?.['15m']?.ChangePercent || 0,
      '1h': cp[pair]?.['1h']?.ChangePercent || 0,
      '4h': cp[pair]?.['4h']?.ChangePercent || 0,
      '1d': cp[pair]?.['1d']?.ChangePercent || 0,
      isFavorite: favoritePairs.value.has(pair)
    })
  }

  return result
})

// Отображаемый список
const displayedPairs = computed(() => {
  if (filterMode.value === 'favorites') {
    return pairs.value.filter(p =>
      favoritePairs.value.has(p.pair + 'USDT')
    )
  }

  return pairs.value
})

// Загрузка избранного
function loadFavorites() {
  const stored = localStorage.getItem('favoritePairs')

  if (!stored) return

  try {
    favoritePairs.value = new Set(JSON.parse(stored))
  } catch {
    favoritePairs.value = new Set()
  }
}

// Сохранение избранного
function saveFavorites() {
  localStorage.setItem(
    'favoritePairs',
    JSON.stringify(Array.from(favoritePairs.value))
  )
}

// Toggle favorite
function toggleFavorite(pairShort: string) {
  const pairFull = pairShort + 'USDT'

  if (favoritePairs.value.has(pairFull)) {
    favoritePairs.value.delete(pairFull)
  } else {
    favoritePairs.value.add(pairFull)
  }

  saveFavorites()
}

// Форматирование объема
const formatVolume = (volume: number) => {
  if (volume >= 1_000_000_000) {
    return (volume / 1_000_000_000).toFixed(2) + 'B'
  }

  if (volume >= 1_000_000) {
    return (volume / 1_000_000).toFixed(2) + 'M'
  }

  if (volume >= 1_000) {
    return (volume / 1_000).toFixed(2) + 'K'
  }

  return volume.toString()
}

// Цвет изменения
const getChangeClass = (value: number) => {
  if (value > 0) return 'positive'
  if (value < 0) return 'negative'
  return 'neutral'
}

// Клик по строке
const onRowClick = (event: any) => {
  selectPair(event.data.pair + 'USDT')
}

// Active row class
const getRowClass = (data: PriceData) => {
  return {
    'table-row-active':
      currentPair.value === data.pair + 'USDT'
  }
}

// Скролл к активной строке
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

  const containerRect = container.getBoundingClientRect()
  const rowRect = activeRow.getBoundingClientRect()

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

// Watch active pair
watch(
  [currentPair, displayedPairs],
  () => {
    scrollToPair()
  },
  { deep: true }
)

watch(activeComponent, async (val) => {
  if (val !== 'price') return

  await nextTick()

  requestAnimationFrame(() => {
    scrollToPair()
  })
})

// Init
loadFavorites()
</script>

<template>
  <div class="price-table-wrapper">

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
    </div>

    <div
      v-if="!changePrices || Object.keys(changePrices).length === 0"
      class="loading-msg"
    >
      Загрузка данных...
    </div>

    <div
      v-else
      ref="tableContainerRef"
      class="table-container"
    >
      <DataTable
        :value="displayedPairs"
        :scrollable="true"
        scrollHeight="flex"
        class="price-table"
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
          header="24h V"
          :sortable="true"
          style="min-width: 100px"
        >
          <template #body="{ data }">
            {{ formatVolume(data.volume) }}
          </template>
        </Column>

        <!-- 24h -->
        <Column
          field="ch24"
          header="24h%"
          :sortable="true"
          style="min-width: 100px"
        >
          <template #body="{ data }">
            <span
              :class="[
                'change-value',
                getChangeClass(data.ch24)
              ]"
            >
              {{ data.ch24 > 0 ? '+' : '' }}{{ data.ch24.toFixed(2) }}
            </span>
          </template>
        </Column>

        <!-- Periods -->
        <Column
          v-for="period in TIME_PERIODS"
          :key="period"
          :field="period"
          :header="`${period}%`"
          :sortable="true"
          style="min-width: 100px"
        >
          <template #body="{ data }">
            <span
              :class="[
                'change-value',
                getChangeClass(data[period])
              ]"
            >
              {{ data[period] > 0 ? '+' : '' }}{{ data[period].toFixed(2) }}
            </span>
          </template>
        </Column>

      </DataTable>
    </div>

  </div>
</template>

<style scoped>
.price-table-wrapper {
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


.loading-msg {
  flex: 1;

  display: flex;
  align-items: center;
  justify-content: center;

  font-size: 0.9rem;
}

.table-container {
  flex: 1;

  min-height: 0;

  overflow: hidden;

  position: relative;

  padding-top: 0.5rem;
  
}

:deep(.price-table) {
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

.change-value {
  display: inline-block;

  padding: 0.2rem 0.4rem;

  border-radius: 4px;

  font-weight: 500;
}

.change-value.positive {
  color: var(--p-green-500);
}

.change-value.negative {
  color: var(--p-red-500);
}

.change-value.neutral {
  color: var(--p-text-muted-color);
}
</style>