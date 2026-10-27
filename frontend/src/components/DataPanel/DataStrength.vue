<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import DataTable, { type DataTableRowClickEvent } from 'primevue/datatable'
import Column from 'primevue/column'
import Checkbox from 'primevue/checkbox'
import InputNumber from 'primevue/inputnumber'
import Button from 'primevue/button'
import { useMarketStore } from '../../stores/market'
import { useUIStore } from '../../stores/ui'
import { useFiltersStore } from '../../stores/filters'
import { useScrollToPair, ROW_HEIGHT } from '../../composables/useScrollToPair'
import { compareByField } from '../../utils/tableSort'
import type { StrengthEntry } from '../../types'
import DataPanelToolbar from './DataPanelToolbar.vue'

type Side = 'BUY' | 'SELL'

// Компоненты композита. regime - не голосующий сигнал (у сжатия волатильности
// нет стороны), а множитель силы остальных - см. applyComposite ниже.
//
// «Стены» здесь больше нет (strategy_v2, см. ARCHITECTURE.md §10.3):
// направленный сигнал по резервному объёму на уровне стакана легко
// подделывается спуфингом, а confirm-стрик защищает только от однотактного
// шума, не от целенаправленной подмены ордера. Имбаланс/Buy-активность
// заменены на OFI/VPIN - те же роли, но событийная/объёмная методология
// вместо снапшота по времени (см. Go-комментарий у entrysetup.StrengthComponents).
const COMPONENTS = [
  { key: 'ofi', label: 'OFI', hint: 'Order Flow Imbalance - событийный поток заявок на лучших уровнях стакана (Cont-Kukanov-Stoikov), z-score относительно истории пары.' },
  { key: 'levels', label: 'Уровни', hint: 'Асимметрия уровней поддержки/сопротивления по разворотным точкам на графике цены (swing high/low).' },
  { key: 'vpin', label: 'VPIN', hint: 'Доля покупок в объёмном бакете (VPIN-подобный, Easley/López de Prado/O\'Hara) - z-score относительно истории пары.' },
  { key: 'relstrength', label: 'Отн. сила', hint: 'Кросс-секционный сигнал: изменение цены пары относительно МЕДИАНЫ изменения по всему рынку за тот же период - отличает идиосинкратическую аномалию пары от общего движения рынка (например, всё падает вслед за BTC).' },
  { key: 'regime', label: 'Сжатие волатильности', hint: 'Множитель, не голос: усиливает композит, когда пара двигалась в последнее время заметно меньше обычного (сжатие часто предшествует выносу).' },
] as const

type ComponentKey = typeof COMPONENTS[number]['key']

const enabled = reactive<Record<ComponentKey, boolean>>({
  ofi: true,
  levels: true,
  vpin: true,
  relstrength: true,
  regime: true,
})

interface StrengthRow {
  pair: string
  isFavorite: boolean
  hasComposite: boolean
  composite: number
  side: Side | null
  agree: number
  total: number
  hasOFI: boolean
  ofiZ: number
  ofiConfirmed: boolean
  hasLevels: boolean
  levelsSide: Side | null
  levelsScore: number
  hasRegime: boolean
  regime: number
  hasVPIN: boolean
  vpinZ: number
  hasRelativeStrength: boolean
  relativeStrengthZ: number
  relativeStrengthConfirmed: boolean
}

const market = useMarketStore()
const ui = useUIStore()
const filters = useFiltersStore()

const tableContainerRef = ref<HTMLElement | null>(null)

// Один период на всю таблицу, кнопками сверху - тот же паттерн, что у
// activeFrame в DataVolumeDelta.vue. Список периодов берём из уже
// загруженных данных (GetAllStrengthComponents отдаёт одинаковый набор
// периодов для каждой пары), а не хардкодим - на бэкенде периоды настраиваются.
const availablePeriods = computed(() => {
  for (const pair in market.strength) {
    return Object.keys(market.strength[pair])
  }
  return []
})
const activePeriod = ref<string>('')

watch(availablePeriods, periods => {
  if (activePeriod.value && periods.includes(activePeriod.value)) return
  activePeriod.value = periods.includes('15m') ? '15m' : (periods[0] ?? '')
}, { immediate: true })

// topN=0/null - без ограничения, показываем все отфильтрованные пары.
const topN = ref<number | null>(0)

function signOf(side: Side): number {
  return side === 'BUY' ? 1 : -1
}

// CONTRIBUTION_CLAMP - потолок |вклада| одного компонента в композит.
//
// Разбор боевых логов (см. ARCHITECTURE.md §10) показал: OFI на реальных
// (не вырожденных, не баг) резких всплесках объёма честно даёт z в сотни -
// это не ошибка калибровки, а особенность единиц измерения (сырой объём
// базового актива), просто плохо ложащаяся на привычную "z-score = сигмы"
// интуицию. Без потолка один такой тик полностью перевешивает "Потенциал"
// для пары - средние ±2..5 остальных компонентов тонут в одном выбросе.
// Клип - только для СРЕДНЕГО: сырое значение в своей колонке показывается
// как есть, обрезается только вклад в композит.
const CONTRIBUTION_CLAMP = 10

function clampContribution(value: number): number {
  return Math.max(-CONTRIBUTION_CLAMP, Math.min(CONTRIBUTION_CLAMP, value))
}

// LevelsScore - "во сколько раз дальше" (>=1, без верхней границы). Логарифм
// переводит это в шкалу, сравнимую с z-score остальных компонентов: score=1
// (нет реальной асимметрии) даёт 0, score=e даёт 1 - тот же порядок, что и
// заметный z-score.
function ratioContribution(side: Side, score: number): number {
  return signOf(side) * Math.log(Math.max(score, 1))
}

function buildRow(pair: string, entry: StrengthEntry | undefined): StrengthRow {
  const contributions: number[] = []

  // OFI и VPIN - окна, коммитящиеся раз в некоторое время (а не мгновенный
  // снимок), но сторона всё равно может смениться на первом же новом окне. В
  // композит идут, только если сторона уже подтвердилась (см.
  // Go-комментарий у depth.GetOFIConfirmedSide/prices.GetVPINConfirmedSide) -
  // иначе "Потенциал" мигал бы на каждую смену окна вместо того, чтобы
  // отражать реально держащийся перекос. Сами значения в колонках при этом
  // показываются всегда - это только фильтр для композита, не сокрытие данных.
  // Каждый вклад клипуется CONTRIBUTION_CLAMP-ом - см. комментарий там же.
  if (enabled.ofi && entry?.HasOFI && entry.OFIConfirmed) {
    contributions.push(clampContribution(entry.OFIZScore))
  }
  // VPIN - без confirm-gate, в отличие от OFI/RelativeStrength: разбор логов
  // (см. ARCHITECTURE.md §10) показал, что streak-подтверждение здесь не
  // срабатывает НИКОГДА (0 раз за 43ч на 5 парах) - бакет коммитится раз в
  // 20-55 минут, а требовать 3 подряд экстремальных бакета - это часы
  // непрерывного перекоса, чего на практике не бывает. Сам бакет уже
  // интеграл ~20 минут ордерфлоу (усреднён), в отличие от сырого тика OFI -
  // доп. подтверждение через ещё несколько таких интегралов избыточно.
  if (enabled.vpin && entry?.HasVPIN) {
    contributions.push(clampContribution(entry.VPINZScore))
  }
  if (enabled.relstrength && entry?.HasRelativeStrength && entry.RelativeStrengthConfirmed) {
    contributions.push(clampContribution(entry.RelativeStrengthZScore))
  }
  if (enabled.levels && entry?.HasLevels) {
    contributions.push(clampContribution(ratioContribution(entry.LevelsSide, entry.LevelsScore)))
  }

  let composite = contributions.length
    ? contributions.reduce((sum, c) => sum + c, 0) / contributions.length
    : 0

  // Сжатие волатильности - множитель, а не ещё один голос: усиливает то, что
  // уже показали остальные компоненты, но само по себе стороны не задаёт.
  if (enabled.regime && entry?.HasRegime && entry.Regime < 1 && composite !== 0) {
    const boost = Math.min(2 - entry.Regime, 1.5)
    composite *= boost
  }

  const side: Side | null = composite > 0 ? 'BUY' : composite < 0 ? 'SELL' : null
  const agree = side ? contributions.filter(c => Math.sign(c) === Math.sign(composite)).length : 0

  return {
    pair: pair.replace(/USDT$/, ''),
    isFavorite: ui.favoritePairs.has(pair),
    hasComposite: contributions.length > 0,
    composite,
    side,
    agree,
    total: contributions.length,
    hasOFI: entry?.HasOFI ?? false,
    ofiZ: entry?.OFIZScore ?? 0,
    ofiConfirmed: entry?.OFIConfirmed ?? false,
    hasLevels: entry?.HasLevels ?? false,
    levelsSide: entry?.HasLevels ? entry.LevelsSide : null,
    levelsScore: entry?.LevelsScore ?? 0,
    hasRegime: entry?.HasRegime ?? false,
    regime: entry?.Regime ?? 0,
    hasVPIN: entry?.HasVPIN ?? false,
    vpinZ: entry?.VPINZScore ?? 0,
    hasRelativeStrength: entry?.HasRelativeStrength ?? false,
    relativeStrengthZ: entry?.RelativeStrengthZScore ?? 0,
    relativeStrengthConfirmed: entry?.RelativeStrengthConfirmed ?? false,
  }
}

const rows = computed(() => {
  const period = activePeriod.value
  const result: StrengthRow[] = []
  for (const pair of filters.filteredPairs) {
    result.push(buildRow(pair, market.strength[pair]?.[period]))
  }
  return result
})

const displayedRows = computed(() => {
  if (ui.filterMode === 'favorites') {
    return rows.value.filter(r => ui.favoritePairs.has(r.pair + 'USDT'))
  }
  return rows.value
})

const sortField = ref<string | undefined>(undefined)
const sortOrder = ref<number>(1)

const sortedRows = computed(() => {
  const list = [...displayedRows.value]
  if (sortField.value && sortOrder.value) {
    const field = sortField.value
    const order = sortOrder.value
    return list.sort((a, b) => compareByField(a, b, field, order))
  }
  // По умолчанию - по убыванию |композита|: самые выраженные сетапы сверху,
  // независимо от стороны (BUY/SELL).
  return list.sort((a, b) => Math.abs(b.composite) - Math.abs(a.composite))
})

const topRows = computed(() => {
  if (!topN.value || topN.value <= 0) return sortedRows.value
  return sortedRows.value.slice(0, topN.value)
})

function toggleFavorite(pairShort: string) {
  ui.toggleFavorite(pairShort + 'USDT')
}

const onRowClick = (event: DataTableRowClickEvent) => {
  ui.selectPair((event.data as StrengthRow).pair + 'USDT')
}

const getRowClass = (data: StrengthRow) => ({
  'table-row-active': ui.currentPair === data.pair + 'USDT'
})

function fmt(value: number) {
  return (value > 0 ? '+' : '') + value.toLocaleString('ru', { maximumFractionDigits: 2, minimumFractionDigits: 2 })
}

function sideColor(side: Side | null) {
  if (!side) return 'inherit'
  return side === 'BUY' ? 'var(--p-green-500)' : 'var(--p-red-500)'
}

// liveSideColor - для колонок "Имбаланс"/"Стены": значение показывается
// всегда, но красится только когда сторона подтвердилась (см. buildRow) -
// пока подтверждения нет, это может быть секундный дёрг стакана, а не
// реальный сигнал, и приглушённый цвет об этом сообщает без скрытия числа.
function liveSideColor(has: boolean, confirmed: boolean, side: Side | null) {
  if (!has) return 'inherit'
  if (!confirmed) return 'var(--p-text-muted-color, #999)'
  return sideColor(side)
}

const REGIME_SQUEEZE = 0.5

function regimeColor(row: StrengthRow) {
  if (!row.hasRegime) return 'inherit'
  return row.regime < REGIME_SQUEEZE ? 'var(--p-blue-500)' : 'inherit'
}

const orderedPairs = computed(() => topRows.value.map(r => r.pair + 'USDT'))

useScrollToPair(tableContainerRef, orderedPairs, 'strength')

// Тот же принцип, что и у DataImbalance.vue: нет вебсокет-канала для этих
// данных, опрашиваем сами, пока вкладка реально видна.
const STRENGTH_POLL_INTERVAL_MS = 10000

let pollTimer: ReturnType<typeof setInterval> | null = null

function startPolling() {
  if (pollTimer) return
  pollTimer = setInterval(() => market.fetchStrength(), STRENGTH_POLL_INTERVAL_MS)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

watch(() => ui.activeDataPanel, panel => {
  if (panel === 'strength') {
    market.fetchStrength()
    startPolling()
  } else {
    stopPolling()
  }
})

onMounted(() => {
  if (ui.activeDataPanel === 'strength') {
    market.fetchStrength()
    startPolling()
  }
})

onBeforeUnmount(stopPolling)
</script>

<template>
  <div class="strength-table-wrapper">

    <div class="table-header">
      <DataPanelToolbar />
    </div>

    <div class="settings-bar">

      <div class="settings-row">
        <div class="period-buttons">
          <Button
            v-for="p in availablePeriods"
            :key="p"
            :label="p"
            size="small"
            severity="secondary"
            :outlined="activePeriod !== p"
            @click="activePeriod = p"
          />
        </div>

        <div class="divider" />

        <div class="topn-control">
          <span>Топ</span>
          <InputNumber
            v-model="topN"
            :min="0"
            :max="500"
            :useGrouping="false"
            size="small"
            placeholder="все"
          />
        </div>
      </div>

      <div class="settings-row">
        <div class="component-toggles">
          <label
            v-for="c in COMPONENTS"
            :key="c.key"
            class="toggle-label"
            :title="c.hint"
          >
            <Checkbox v-model="enabled[c.key]" :binary="true" size="small" />
            <span>{{ c.label }}</span>
          </label>
        </div>
      </div>

    </div>

    <div ref="tableContainerRef" class="table-container">
      <DataTable
        :value="topRows"
        v-model:sortField="sortField"
        v-model:sortOrder="sortOrder"
        :scrollable="true"
        scrollHeight="flex"
        :virtualScrollerOptions="{ itemSize: ROW_HEIGHT }"
        class="strength-table"
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
            <span class="pair-name">{{ data.pair }}</span>
          </template>
        </Column>

        <!-- Composite -->
        <Column
          field="composite"
          :sortable="true"
          style="min-width: 130px"
        >
          <template #header>
            <span class="header-hint" title="Средний вклад отмеченных чекбоксами компонентов (OFI/VPIN/уровни/отн. сила), усиленный при сжатии волатильности. Знак - BUY/SELL, величина - насколько выражен сетап. Не рекомендация входить, сводная оценка по структуре рынка прямо сейчас.">
              Потенциал
            </span>
          </template>
          <template #body="{ data }">
            <span :style="{ color: sideColor(data.side) }">
              {{ data.hasComposite ? `${data.side} ${fmt(data.composite)}` : '—' }}
            </span>
          </template>
        </Column>

        <!-- Agreement -->
        <Column
          field="agree"
          :sortable="true"
          style="min-width: 90px"
        >
          <template #header>
            <span class="header-hint" title="Сколько из отмеченных компонентов согласны со стороной композита, из скольких отмеченных компонентов сейчас доступны данные. Больше - надёжнее: несколько независимых наблюдений совпали, а не одно случайное.">
              Согласие
            </span>
          </template>
          <template #body="{ data }">
            {{ data.total ? `${data.agree}/${data.total}` : '—' }}
          </template>
        </Column>

        <!-- OFI -->
        <Column
          field="ofiZ"
          :sortable="true"
          style="min-width: 100px"
        >
          <template #header>
            <span class="header-hint" title="Order Flow Imbalance, z-score (событийный поток заявок на лучших уровнях стакана). Серым - сторона ещё не подтвердилась (держится меньше нескольких окон подряд) и в «Потенциал» пока не засчитывается.">OFI</span>
          </template>
          <template #body="{ data }">
            <span
              :style="{ color: liveSideColor(data.hasOFI, data.ofiConfirmed, data.ofiZ > 0 ? 'BUY' : 'SELL') }"
              :title="data.hasOFI && !data.ofiConfirmed ? 'Сторона ещё не подтвердилась - в композит «Потенциал» пока не входит' : ''"
            >
              {{ data.hasOFI ? fmt(data.ofiZ) : '—' }}
            </span>
          </template>
        </Column>

        <!-- Levels -->
        <Column
          field="levelsScore"
          :sortable="true"
          style="min-width: 100px"
        >
          <template #header>
            <span class="header-hint" title="Свечные уровни поддержки/сопротивления: сторона + во сколько раз дальний уровень дальше ближнего.">Уровни</span>
          </template>
          <template #body="{ data }">
            <span :style="{ color: sideColor(data.levelsSide) }">
              {{ data.hasLevels ? `${data.levelsSide} ×${data.levelsScore.toFixed(2)}` : '—' }}
            </span>
          </template>
        </Column>

        <!-- VPIN -->
        <Column
          field="vpinZ"
          :sortable="true"
          style="min-width: 100px"
        >
          <template #header>
            <span class="header-hint" title="VPIN-подобный: доля покупок в объёмном (не временном) бакете, z-score относительно истории пары. Голосует в «Потенциал» всегда, без доп. подтверждения - см. Go-комментарий в buildRow.">VPIN</span>
          </template>
          <template #body="{ data }">
            <span :style="{ color: sideColor(data.hasVPIN ? (data.vpinZ > 0 ? 'BUY' : 'SELL') : null) }">
              {{ data.hasVPIN ? fmt(data.vpinZ) : '—' }}
            </span>
          </template>
        </Column>

        <!-- Relative strength -->
        <Column
          field="relativeStrengthZ"
          :sortable="true"
          style="min-width: 100px"
        >
          <template #header>
            <span class="header-hint" title="Кросс-секционная относительная сила: изменение цены пары минус медианное изменение по всему рынку за тот же период, z-score относительно истории пары. Серым - сторона ещё не подтвердилась и в «Потенциал» пока не засчитывается.">Отн. сила</span>
          </template>
          <template #body="{ data }">
            <span
              :style="{ color: liveSideColor(data.hasRelativeStrength, data.relativeStrengthConfirmed, data.relativeStrengthZ > 0 ? 'BUY' : 'SELL') }"
              :title="data.hasRelativeStrength && !data.relativeStrengthConfirmed ? 'Сторона ещё не подтвердилась - в композит «Потенциал» пока не входит' : ''"
            >
              {{ data.hasRelativeStrength ? fmt(data.relativeStrengthZ) : '—' }}
            </span>
          </template>
        </Column>

        <!-- Regime -->
        <Column
          field="regime"
          :sortable="true"
          style="min-width: 90px"
        >
          <template #header>
            <span class="header-hint" title="Сжатие/расширение волатильности относительно нормы (см. таблицу «Имбаланс»). Множитель композита, не голос.">Режим</span>
          </template>
          <template #body="{ data }">
            <span :style="{ color: regimeColor(data) }">
              {{ data.hasRegime ? `×${data.regime.toFixed(2)}` : '—' }}
            </span>
          </template>
        </Column>

      </DataTable>
    </div>

  </div>
</template>

<style scoped>
.strength-table-wrapper {
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

.settings-bar {
  flex-shrink: 0;

  display: flex;
  flex-direction: column;

  gap: 0.4rem;

  padding: 0.4rem 0;

  overflow-x: auto;

  -webkit-overflow-scrolling: touch;
}

.settings-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;

  gap: 0.6rem;

  white-space: nowrap;
}

.period-buttons {
  display: flex;
  gap: 0.15rem;
}

.divider {
  width: 1px;
  height: 1.25rem;
  background: var(--p-content-border-color);
  flex-shrink: 0;
}

.component-toggles {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.toggle-label {
  display: flex;
  align-items: center;
  gap: 0.3rem;

  font-size: 0.78rem;

  cursor: help;
}

.topn-control {
  display: flex;
  align-items: center;
  gap: 0.4rem;

  font-size: 0.85rem;
}

.table-container {
  flex: 1;

  min-height: 0;

  overflow: hidden;

  position: relative;

  padding-top: 0.25rem;
}

:deep(.strength-table) {
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
