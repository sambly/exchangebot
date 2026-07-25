<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import type { DeltaEntry } from '../../types'
import DataTable, { type DataTableRowClickEvent } from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import { useUIStore } from '../../stores/ui'
import { useMarketStore } from '../../stores/market'
import { useFiltersStore } from '../../stores/filters'
import { useScrollToPair, ROW_HEIGHT } from '../../composables/useScrollToPair'
import { compareByField } from '../../utils/tableSort'
import DataPanelToolbar from './DataPanelToolbar.vue'

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
const market = useMarketStore()
const filters = useFiltersStore()

const frames = ['1m', '3m', '15m', '1h', '4h', '12h'] as const
const activeFrame = ref<string>('15m')
const tableContainerRef = ref<HTMLElement | null>(null)

const displayData = computed(() => {
  const df = market.deltaFast
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

  return result
})

// Сортировка ведётся здесь (а не отдана целиком PrimeVue), чтобы orderedPairs
// ниже точно совпадал с порядком отрисованных строк - иначе useScrollToPair
// центрирует не ту строку после клика по заголовку столбца. Пока пользователь
// не кликнул по заголовку - сохраняем прежний дефолт: по убыванию объёма.
const sortField = ref<string | undefined>(undefined)
const sortOrder = ref<number>(1)

const sortedData = computed(() => {
  const list = [...displayData.value]
  if (sortField.value && sortOrder.value) {
    const field = sortField.value
    const order = sortOrder.value
    return list.sort((a, b) => compareByField(a, b, field, order))
  }
  return list.sort((a, b) => b.volume - a.volume)
})

function toggleFavorite(pairShort: string) {
  ui.toggleFavorite(pairShort + 'USDT')
}

const onRowClick = (event: DataTableRowClickEvent) => {
  ui.selectPair((event.data as VolumeData).pair + 'USDT')
}

const fmt = (val: number) => val.toFixed(2)

const getRowClass = (data: VolumeData) => ({
  'table-row-active': ui.currentPair === data.pair + 'USDT'
})

const orderedPairs = computed(() => sortedData.value.map(d => d.pair + 'USDT'))

const { scrollToPair } = useScrollToPair(tableContainerRef, orderedPairs, 'volume')

onMounted(async () => {
  if (!Object.keys(market.deltaFast).length) {
    await market.fetchDelta()
  }
  scrollToPair()
})
</script>

<template>
  <div class="volume-table-wrapper">

    <!-- Header -->
    <div class="table-header">

      <DataPanelToolbar />

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
      v-if="market.deltaError"
      class="error-msg"
    >
      {{ market.deltaError }}
    </div>

    <!-- Table -->
    <div
      v-else
      ref="tableContainerRef"
      class="table-container"
    >
      <DataTable
        :value="sortedData"
        v-model:sortField="sortField"
        v-model:sortOrder="sortOrder"
        :loading="market.isDeltaLoading"
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
          :sortable="true"
          style="min-width: 80px"
        >
          <template #header>
            <span title="Торговая пара">Пара</span>
          </template>
          <template #body="{ data }">
            <span class="pair-name">
              {{ data.pair }}
            </span>
          </template>
        </Column>

        <!-- Volume -->
        <Column
          field="volume"
          :sortable="true"
          style="min-width: 90px"
        >
          <template #header>
            <span class="header-hint" title="Изменение суммарного объёма сделок (%) за последнюю половину выбранного окна относительно предыдущей половины. Это дельта, а не абсолютный объём: большое число здесь не значит 'ликвидная пара', а значит 'объём сильно вырос/упал только что'.">
              Volume
            </span>
          </template>
          <template #body="{ data }">
            {{ fmt(data.volume) }}
          </template>
        </Column>

        <!-- Volume Buy -->
        <Column style="min-width: 100px">
          <template #header>
            <span class="header-hint" title="То же самое, что Volume, но только по объёму тейкер-покупок (сделки, инициированные покупателем).">
              Volume Buy
            </span>
          </template>
          <template #body="{ data }">
            <span class="delta-positive">
              {{ fmt(data.volumeBuy) }}
            </span>
          </template>
        </Column>

        <!-- Volume Ask -->
        <Column style="min-width: 100px">
          <template #header>
            <span class="header-hint" title="То же самое, что Volume, но только по объёму тейкер-продаж (сделки, инициированные продавцом).">
              Volume Ask
            </span>
          </template>
          <template #body="{ data }">
            <span class="delta-negative">
              {{ fmt(data.volumeAsk) }}
            </span>
          </template>
        </Column>

        <!-- Trades -->
        <Column
          field="trades"
          :sortable="true"
          style="min-width: 80px"
        >
          <template #header>
            <span class="header-hint" title="Изменение числа сделок (%) за последнюю половину выбранного окна относительно предыдущей половины - как Volume, но по количеству транзакций, а не по объёму.">
              Trades
            </span>
          </template>
          <template #body="{ data }">
            {{ fmt(data.trades) }}
          </template>
        </Column>

        <!-- Trades Buy -->
        <Column style="min-width: 100px">
          <template #header>
            <span class="header-hint" title="То же самое, что Trades, но только по числу тейкер-покупок.">
              Trades Buy
            </span>
          </template>
          <template #body="{ data }">
            <span class="delta-positive">
              {{ fmt(data.tradesBuy) }}
            </span>
          </template>
        </Column>

        <!-- Trades Ask -->
        <Column style="min-width: 100px">
          <template #header>
            <span class="header-hint" title="То же самое, что Trades, но только по числу тейкер-продаж.">
              Trades Ask
            </span>
          </template>
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

.header-hint {
  cursor: help;
  border-bottom: 1px dotted var(--p-text-muted-color, #999);
}

.delta-positive {
  color: var(--p-green-500);
}

.delta-negative {
  color: var(--p-red-500);
}
</style>