<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import DataTable, { type DataTableRowClickEvent } from 'primevue/datatable'
import Column from 'primevue/column'
import { useMarketStore } from '../../stores/market'
import { useUIStore } from '../../stores/ui'
import { useFiltersStore } from '../../stores/filters'
import { useScrollToPair, ROW_HEIGHT } from '../../composables/useScrollToPair'
import { compareByField } from '../../utils/tableSort'
import DataPanelToolbar from './DataPanelToolbar.vue'

interface ImbalanceRow {
  pair: string
  imbalance: number
  zScore: number
  ready: boolean
  isFavorite: boolean
}

const market = useMarketStore()
const ui = useUIStore()
const filters = useFiltersStore()

const tableContainerRef = ref<HTMLElement | null>(null)

// |z| >= порог - заметное отклонение имбаланса от типичного поведения ИМЕННО
// этой пары (см. internal/depth.GetImbalanceZScore) - тот же порог и та же
// логика подсветки, что и в DepthChart.vue.
const Z_THRESHOLD = 2

const rows = computed(() => {
  const result: ImbalanceRow[] = []
  for (const pair of filters.filteredPairs) {
    const entry = market.imbalance[pair]
    result.push({
      pair: pair.replace(/USDT$/, ''),
      imbalance: entry?.Imbalance ?? 0,
      zScore: entry?.ZScore ?? 0,
      ready: entry?.Ready ?? false,
      isFavorite: ui.favoritePairs.has(pair)
    })
  }
  return result
})

const displayedRows = computed(() => {
  if (ui.filterMode === 'favorites') {
    return rows.value.filter(r => ui.favoritePairs.has(r.pair + 'USDT'))
  }
  return rows.value
})

// Сортировка ведётся здесь (а не отдана целиком PrimeVue), чтобы orderedPairs
// ниже точно совпадал с порядком отрисованных строк - иначе useScrollToPair
// центрирует не ту строку после клика по заголовку столбца (см. тот же
// паттерн в DataChangePrice.vue/DataVolumeDelta.vue).
const sortField = ref<string | undefined>(undefined)
const sortOrder = ref<number>(1)

const sortedRows = computed(() => {
  if (!sortField.value || !sortOrder.value) return displayedRows.value
  const field = sortField.value
  const order = sortOrder.value
  return [...displayedRows.value].sort((a, b) => compareByField(a, b, field, order))
})

function toggleFavorite(pairShort: string) {
  ui.toggleFavorite(pairShort + 'USDT')
}

const onRowClick = (event: DataTableRowClickEvent) => {
  ui.selectPair((event.data as ImbalanceRow).pair + 'USDT')
}

const getRowClass = (data: ImbalanceRow) => ({
  'table-row-active': ui.currentPair === data.pair + 'USDT'
})

function imbalanceColor(row: ImbalanceRow) {
  if (!row.ready || Math.abs(row.zScore) < Z_THRESHOLD) return 'inherit'
  return row.imbalance > 0 ? 'var(--p-green-500)' : 'var(--p-red-500)'
}

function fmt(value: number) {
  return (value > 0 ? '+' : '') + value.toLocaleString('ru', { maximumFractionDigits: 2, minimumFractionDigits: 2 })
}

const orderedPairs = computed(() => sortedRows.value.map(r => r.pair + 'USDT'))

useScrollToPair(tableContainerRef, orderedPairs, 'imbalance')

onMounted(async () => {
  if (!Object.keys(market.imbalance).length) {
    await market.fetchImbalance()
  }
})
</script>

<template>
  <div class="imbalance-table-wrapper">

    <div class="table-header">
      <DataPanelToolbar />
    </div>

    <div ref="tableContainerRef" class="table-container">
      <DataTable
        :value="sortedRows"
        v-model:sortField="sortField"
        v-model:sortOrder="sortOrder"
        :scrollable="true"
        scrollHeight="flex"
        :virtualScrollerOptions="{ itemSize: ROW_HEIGHT }"
        class="imbalance-table"
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

        <!-- Imbalance -->
        <Column
          field="imbalance"
          :sortable="true"
          style="min-width: 100px"
        >
          <template #header>
            <span class="header-hint" title="Соотношение объёма топ-20 уровней стакана: (биды − аски) / (биды + аски). От -1 (в топе только продавцы) до +1 (только покупатели). Это про выставленные заявки, а не про исполненные сделки.">
              Имбаланс
            </span>
          </template>
          <template #body="{ data }">
            <span :style="{ color: imbalanceColor(data) }">
              {{ data.ready ? fmt(data.imbalance) : '—' }}
            </span>
          </template>
        </Column>

        <!-- Z-score -->
        <Column
          field="zScore"
          :sortable="true"
          style="min-width: 100px"
        >
          <template #header>
            <span class="header-hint" title="Насколько текущий имбаланс необычен ИМЕННО ДЛЯ ЭТОЙ ПАРЫ относительно её собственной истории за ~15 минут (робастная медиана + MAD, та же математика, что у детектора аномалий). |z| >= 2 - заметное отклонение от нормы, подсвечивается цветом.">
              z-score
            </span>
          </template>
          <template #body="{ data }">
            <span :style="{ color: imbalanceColor(data) }" :title="!data.ready ? 'Стакан по паре не отслеживается или ещё не набрал историю' : ''">
              {{ data.ready ? fmt(data.zScore) : '—' }}
            </span>
          </template>
        </Column>

      </DataTable>
    </div>

  </div>
</template>

<style scoped>
.imbalance-table-wrapper {
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

.table-container {
  flex: 1;

  min-height: 0;

  overflow: hidden;

  position: relative;

  padding-top: 0.5rem;
}

:deep(.imbalance-table) {
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
</style>
