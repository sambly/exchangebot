<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import DataTable, { type DataTableRowClickEvent } from 'primevue/datatable'
import Column from 'primevue/column'
import { useMarketStore } from '../../stores/market'
import { useUIStore } from '../../stores/ui'
import { useFiltersStore } from '../../stores/filters'
import { useScrollToPair, ROW_HEIGHT } from '../../composables/useScrollToPair'
import { compareByField } from '../../utils/tableSort'
import type { PriceLevelsEntry } from '../../types'
import DataPanelToolbar from './DataPanelToolbar.vue'

interface ImbalanceRow {
  pair: string
  imbalance: number
  zScore: number
  ready: boolean
  isFavorite: boolean
  hasQuality: boolean
  score: number
  side: 'BUY' | 'SELL' | null
  hasLevels: boolean
  levelsSide: 'BUY' | 'SELL' | null
  levelsScore: number
  hasRegime: boolean
  regime: number
}

const market = useMarketStore()
const ui = useUIStore()
const filters = useFiltersStore()

const tableContainerRef = ref<HTMLElement | null>(null)

// |z| >= порог - заметное отклонение имбаланса от типичного поведения ИМЕННО
// этой пары (см. internal/depth.GetImbalanceZScore) - тот же порог и та же
// логика подсветки, что и в DepthChart.vue.
const Z_THRESHOLD = 2

// Score/Side (Quality) не зависят от периода (см. Go-комментарий у
// entrysetup.Quality - соотношение дистанций, деление на волатильность
// периода сократилось бы), поэтому для отображения годится запись ЛЮБОГО
// периода - берём первую, какая нашлась. PriceLevels/VolatilityRegime от
// периода зависят по-настоящему, но у этой таблицы нет своего выбора периода -
// тем же способом берём первый доступный, просто как разумный дефолт.
function firstEntry<T>(byPeriod: Record<string, T> | undefined): T | undefined {
  if (!byPeriod) return undefined
  const key = Object.keys(byPeriod)[0]
  return key ? byPeriod[key] : undefined
}

// PriceLevels не даёт Score/Side готовыми (в Go-структуре только
// Support/Resistance с их DistancePercent) - тот же принцип "ближе - в стоп,
// дальше - в тейк", что и у Quality, здесь просто применён на стороне
// отображения.
function levelsScore(levels: PriceLevelsEntry | undefined) {
  if (!levels?.HasSupport || !levels?.HasResistance) return null
  const support = levels.Support.DistancePercent
  const resistance = levels.Resistance.DistancePercent
  if (support <= 0 || resistance <= 0) return null

  const side: 'BUY' | 'SELL' = support <= resistance ? 'BUY' : 'SELL'
  const near = Math.min(support, resistance)
  const far = Math.max(support, resistance)
  return { side, score: far / near }
}

const rows = computed(() => {
  const result: ImbalanceRow[] = []
  for (const pair of filters.filteredPairs) {
    const entry = market.imbalance[pair]
    const quality = firstEntry(market.quality[pair])
    const levels = levelsScore(firstEntry(market.priceLevels[pair]))
    const regime = firstEntry(market.volatilityRegime[pair])

    result.push({
      pair: pair.replace(/USDT$/, ''),
      imbalance: entry?.Imbalance ?? 0,
      zScore: entry?.ZScore ?? 0,
      ready: entry?.Ready ?? false,
      isFavorite: ui.favoritePairs.has(pair),
      hasQuality: quality !== undefined,
      score: quality?.Score ?? 0,
      side: quality?.Side ?? null,
      hasLevels: levels !== null,
      levelsSide: levels?.side ?? null,
      levelsScore: levels?.score ?? 0,
      hasRegime: regime !== undefined,
      regime: regime ?? 0
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

function scoreColor(row: ImbalanceRow) {
  if (!row.hasQuality) return 'inherit'
  return row.side === 'BUY' ? 'var(--p-green-500)' : 'var(--p-red-500)'
}

function levelsColor(row: ImbalanceRow) {
  if (!row.hasLevels) return 'inherit'
  return row.levelsSide === 'BUY' ? 'var(--p-green-500)' : 'var(--p-red-500)'
}

// Сжатие (regime < REGIME_SQUEEZE) подсвечиваем отдельно от обычных значений:
// это не "хорошо/плохо", а "прямо сейчас интереснее посмотреть" - возможный
// разгон после затишья.
const REGIME_SQUEEZE = 0.5

function regimeColor(row: ImbalanceRow) {
  if (!row.hasRegime) return 'inherit'
  return row.regime < REGIME_SQUEEZE ? 'var(--p-blue-500)' : 'inherit'
}

const orderedPairs = computed(() => sortedRows.value.map(r => r.pair + 'USDT'))

useScrollToPair(tableContainerRef, orderedPairs, 'imbalance')

// В отличие от MarketsStat/ChangePrices (пуш через /trade/ws), у имбаланса и
// Quality/PriceLevels/Regime нет вебсокет-канала - только REST. Раньше эти
// данные забирались один раз при первом монтировании и потом замораживались
// навсегда (DataPanel.vue держит все вкладки смонтированными через v-show,
// так что повторного onMounted при переключении вкладок не происходит).
// Опрашиваем сами, пока вкладка "Имбаланс" реально видна - и не молотим API
// впустую, пока пользователь смотрит другую вкладку.
const IMBALANCE_POLL_INTERVAL_MS = 10000

let pollTimer: ReturnType<typeof setInterval> | null = null

async function refreshImbalanceData() {
  await Promise.all([market.fetchImbalance(), market.fetchQuality()])
}

function startPolling() {
  if (pollTimer) return
  pollTimer = setInterval(refreshImbalanceData, IMBALANCE_POLL_INTERVAL_MS)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

watch(() => ui.activeDataPanel, panel => {
  if (panel === 'imbalance') {
    refreshImbalanceData()
    startPolling()
  } else {
    stopPolling()
  }
})

onMounted(() => {
  if (ui.activeDataPanel === 'imbalance') {
    refreshImbalanceData()
    startPolling()
  }
})

onBeforeUnmount(stopPolling)
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

        <!-- Entry setup -->
        <Column
          field="score"
          :sortable="true"
          style="min-width: 110px"
        >
          <template #header>
            <span class="header-hint" title="Удобство входа по структуре стакана: во сколько раз дальняя стена (потенциальный тейк) дальше ближней (потенциальный стоп). Сторона (BUY/SELL) - какая стена ближе. Не рекомендация входить, а сырое наблюдение по текущему стакану.">
              Стены
            </span>
          </template>
          <template #body="{ data }">
            <span
              :style="{ color: scoreColor(data) }"
              :title="!data.hasQuality ? 'Обе стены (поддержка и сопротивление) сейчас не найдены' : ''"
            >
              {{ data.hasQuality ? `${data.side} ×${data.score.toFixed(2)}` : '—' }}
            </span>
          </template>
        </Column>

        <!-- Price levels (candle-based S/R) -->
        <Column
          field="levelsScore"
          :sortable="true"
          style="min-width: 110px"
        >
          <template #header>
            <span class="header-hint" title="То же соотношение, что и «Стены», но по разворотным точкам НА ГРАФИКЕ ЦЕНЫ (swing high/low за последние ~100 свечей), а не по стакану. Устойчивее к спуфингу (уровень уже состоялся), но запаздывает - свежий уровень пакет пока не увидит.">
              Уровни
            </span>
          </template>
          <template #body="{ data }">
            <span
              :style="{ color: levelsColor(data) }"
              :title="!data.hasLevels ? 'Обе разворотные точки (поддержка и сопротивление) сейчас не найдены' : ''"
            >
              {{ data.hasLevels ? `${data.levelsSide} ×${data.levelsScore.toFixed(2)}` : '—' }}
            </span>
          </template>
        </Column>

        <!-- Volatility regime -->
        <Column
          field="regime"
          :sortable="true"
          style="min-width: 100px"
        >
          <template #header>
            <span class="header-hint" title="Отношение недавней волатильности пары к типичной за тот же период. Меньше 1 - сжатие (двигалась меньше обычного, часто предшествует выносу), больше 1 - расширение (уже разогналась). Не рекомендация, только наблюдение по волатильности.">
              Режим
            </span>
          </template>
          <template #body="{ data }">
            <span
              :style="{ color: regimeColor(data) }"
              :title="!data.hasRegime ? 'Недостаточно истории для оценки режима' : ''"
            >
              {{ data.hasRegime ? `×${data.regime.toFixed(2)}` : '—' }}
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
