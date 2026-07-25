<template>
  <div class="volume-chart-wrapper">
    <div class="toolbar">
      <div class="frames">
        <Button
          v-for="frame in frames"
          :key="frame"
          :label="frame"
          size="small"
          severity="secondary"
          :outlined="selectedFrame !== frame"
          @click="selectedFrame = frame"
        />
      </div>

      <div class="deltas">
        <label
          v-for="d in deltaOptions"
          :key="d.value"
          class="delta-item"
        >
          <input
            type="checkbox"
            :value="d.value"
            v-model="selectedDeltas"
          />

          {{ d.label }}
        </label>
      </div>
    </div>

    <div ref="chartContainer" class="chart-container"></div>
  </div>
</template>

<script setup lang="ts">
import {
  createChart,
  LineSeries,
  ColorType,
  type IChartApi,
  type ISeriesApi
} from 'lightweight-charts'

import {
  ref,
  watch,
  onMounted,
  onBeforeUnmount,
  nextTick
} from 'vue'

import Button from 'primevue/button'
import { toChartTime } from '../../utils/chartTime'

const props = defineProps<{
  pair: string
  darkMode?: boolean
}>()

const chartContainer = ref<HTMLDivElement | null>(null)

const selectedFrame = ref('1h')

const selectedDeltas = ref<string[]>([
  'Price',
  'Volume',
  'VolumeBuy',
  'VolumeAsk'
])

const frames = [
  '1m',
  '3m',
  '15m',
  '1h',
  '4h',
  '12h'
]

const deltaOptions = [
  { label: 'Price', value: 'Price' },
  { label: 'Volume', value: 'Volume' },
  { label: 'Volume Buy', value: 'VolumeBuy' },
  { label: 'Volume Ask', value: 'VolumeAsk' },
  { label: 'Trades', value: 'Trades' }
]

const colors = [
  '#2962FF',
  '#FF5733',
  '#8c7401',
  '#5733FF',
  '#FF33E9',
  '#23605f'
]

// Volume/VolumeBuy/VolumeAsk - объём в котируемой валюте (тысячи-миллионы),
// Trades* - штуки сделок (десятки-сотни). Общая линейная шкала для обоих
// была бы бессмысленна: она целиком определяется бОльшими числами объёма, и
// Trades превращался бы в плоскую линию у нуля вне зависимости от того, как
// он на самом деле меняется - поэтому у Trades своя шкала.
function scaleIdFor(delta: string): 'price' | 'delta' | 'trades' {
  if (delta === 'Price') return 'price'
  if (delta.startsWith('Trades')) return 'trades'
  return 'delta'
}

let chart: IChartApi | null = null

const seriesMap = new Map<string, ISeriesApi<'Line'>>()
// Тултип берёт цвет отсюда, а не пересчитывает свой индекс - раньше он
// считал позицию по ВСЕМ сериям (включая Price), а серии красились по
// индексу только среди дельт, и цвета в лейблах расходились с линиями.
const seriesColorMap = new Map<string, string>()

let resizeObserver: ResizeObserver | null = null
let tooltip: HTMLDivElement | null = null
let spinnerEl: HTMLDivElement | null = null

// ======================================================
// THEME
// ======================================================

function getTextColor() {
  return props.darkMode
    ? '#d1d4dc'
    : '#111111'
}

function getBackgroundColor() {
  return props.darkMode
    ? '#131722'
    : '#ffffff'
}

function getGridColor() {
  return props.darkMode
    ? '#2B2B43'
    : '#E1E3EA'
}

function getTooltipBackground() {
  return props.darkMode
    ? 'rgba(19,23,34,0.95)'
    : 'rgba(255,255,255,0.95)'
}

function getPriceColor() {
  return props.darkMode
    ? '#e0e0e0'
    : '#555555'
}

// ======================================================
// SPINNER
// ======================================================

function showSpinner() {
  if (!chartContainer.value) return
  hideSpinner()
  spinnerEl = document.createElement('div')
  spinnerEl.className = 'chart-loading-overlay chart-loading-overlay-dom'
  spinnerEl.innerHTML = `<div class="p-progressspinner" style="width:40px;height:40px"><svg class="p-progressspinner-svg" viewBox="25 25 50 50" style="animation-duration:2s"><circle class="p-progressspinner-circle" cx="50" cy="50" r="20" fill="none" stroke-width="4" stroke-miterlimit="10" style="stroke:var(--p-primary-color, #3B82F6)"/></svg></div>`
  chartContainer.value.appendChild(spinnerEl)
}

function hideSpinner() {
  spinnerEl?.remove()
  spinnerEl = null
}

// ======================================================
// API
// ======================================================

async function fetchData() {
  const response = await fetch('/trade/api/getChangeDelta', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      Pair: props.pair,
      Frame: selectedFrame.value
    })
  })

  if (!response.ok) {
    throw new Error(
      `HTTP error ${response.status}`
    )
  }

  return await response.json()
}

// ======================================================
// TOOLTIP
// ======================================================

function createTooltip() {
  if (!chartContainer.value) return

  tooltip?.remove()

  tooltip = document.createElement('div')

  tooltip.style.position = 'absolute'
  tooltip.style.top = '8px'
  tooltip.style.left = '8px'
  tooltip.style.padding = '8px 10px'
  tooltip.style.fontSize = '12px'
  tooltip.style.borderRadius = '6px'
  tooltip.style.pointerEvents = 'none'
  tooltip.style.zIndex = '10'

  tooltip.style.background =
    getTooltipBackground()

  tooltip.style.color =
    getTextColor()

  tooltip.style.boxShadow =
    '0 2px 8px rgba(0,0,0,0.15)'

  chartContainer.value.appendChild(tooltip)
}

function attachCrosshair() {
  if (!chart || !tooltip) return

  chart.subscribeCrosshairMove(param => {
    if (!param.time) return

    let html = `
      <div style="
        font-weight:600;
        margin-bottom:6px;
      ">
        ${props.pair}
      </div>
    `

    for (const [key, series] of seriesMap.entries()) {
      const point =
        param.seriesData.get(series) as any

      const color = seriesColorMap.get(key) ?? getPriceColor()

      if (key === 'Price') {
        const value = point?.value !== undefined
          ? Number(point.value).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 6 })
          : '—'

        html += `
          <div style="
            color:${color};
            margin-top:4px;
            font-weight:500;
          ">
            ${key}: ${value}
          </div>
        `
      } else {
        const value = point?.value !== undefined
          ? Number(point.value).toLocaleString('en-US', { maximumFractionDigits: 0 })
          : '—'

        html += `
          <div style="
            color:${color};
            margin-top:4px;
          ">
            ${key}: ${value}
          </div>
        `
      }
    }

    tooltip!.innerHTML = html
  })
}

// ======================================================
// CLEANUP — удаляем chart + tooltip, но сохраняем контейнер
// ======================================================

function cleanupChart() {
  if (chart) {
    chart.remove()
    chart = null
  }
  tooltip?.remove()
  tooltip = null
  seriesMap.clear()
  seriesColorMap.clear()
}

// ======================================================
// CREATE CHART
// ======================================================

async function createChartView() {
  if (!chartContainer.value) return

  showSpinner()

  try {
    const raw = await fetchData()

    if (!Array.isArray(raw)) return

    cleanupChart()

    const width =
      chartContainer.value.clientWidth

    const height =
      chartContainer.value.clientHeight

    chart = createChart(
      chartContainer.value,
      {
        width,
        height,

        layout: {
          textColor: getTextColor(),

          background: {
            type: ColorType.Solid,
            color: getBackgroundColor()
          }
        },

        grid: {
          vertLines: {
            color: getGridColor()
          },

          horzLines: {
            color: getGridColor()
          }
        },

        timeScale: {
          timeVisible: true,
          secondsVisible: false
        }
      }
    )

    createTooltip()

    let colorIndex = 0
    const scalesUsed = new Set<string>()

    for (const delta of selectedDeltas.value) {
      const isPrice = delta === 'Price'
      const fieldName = isPrice ? 'Close' : delta
      const scaleId = scaleIdFor(delta)

      const seriesColor = isPrice
        ? getPriceColor()
        : colors[colorIndex % colors.length]

      const series = chart.addSeries(
        LineSeries,
        {
          color: seriesColor,
          lineWidth: 2,
          priceScaleId: scaleId,
          lastValueVisible: true
        }
      )

      scalesUsed.add(scaleId)
      if (!isPrice) {
        colorIndex++
      }

      const data = raw.map((item: any) => ({
        time: toChartTime(item.Time),
        value: Number(item[fieldName])
      }))

      series.setData(data)

      seriesMap.set(delta, series)
      seriesColorMap.set(delta, seriesColor)
    }

    // Делим высоту графика поровну между реально используемыми шкалами,
    // сверху вниз: price, затем delta (объём), затем trades (число сделок).
    //
    // Trades раньше сидел на одной шкале с Volume ('delta') - но это разные
    // единицы (объём в котируемой валюте против штук сделок), и на одном
    // линейном масштабе Trades визуально превращался в плоскую линию у нуля,
    // хотя сами цифры менялись: масштаб просто целиком определялся Volume,
    // который на порядки больше.
    const scaleOrder = (['price', 'delta', 'trades'] as const).filter(id => scalesUsed.has(id))
    const margin = 0.02
    const slice = scaleOrder.length > 0 ? (1 - margin * 2) / scaleOrder.length : 0

    scaleOrder.forEach((scaleId, index) => {
      chart!.priceScale(scaleId).applyOptions({
        scaleMargins: {
          top: margin + slice * index,
          bottom: margin + slice * (scaleOrder.length - index - 1)
        }
      })
    })

    chart.timeScale().fitContent()

    attachCrosshair()
  } finally {
    hideSpinner()
  }
}

// ======================================================
// RESIZE
// ======================================================

function resizeChart() {
  if (!chart || !chartContainer.value)
    return

  chart.applyOptions({
    width:
      chartContainer.value.clientWidth,

    height:
      chartContainer.value.clientHeight
  })
}

// ======================================================
// REFRESH
// ======================================================

async function refreshChart() {
  await nextTick()

  cleanupChart()

  await createChartView()
}

// ======================================================
// WATCHERS
// ======================================================

watch(
  () => props.pair,
  () => {
    refreshChart()
  }
)

watch(selectedFrame, () => {
  refreshChart()
})

watch(
  selectedDeltas,
  () => {
    if (!selectedDeltas.value.length)
      return

    refreshChart()
  },
  { deep: true }
)

// Смена темы — только обновление опций, без пересоздания
watch(
  () => props.darkMode,
  () => {
    if (!chart) return
    chart.applyOptions({
      layout: {
        textColor: getTextColor(),
        background: {
          type: ColorType.Solid,
          color: getBackgroundColor()
        }
      },
      grid: {
        vertLines: { color: getGridColor() },
        horzLines: { color: getGridColor() }
      }
    })
    if (tooltip) {
      tooltip.style.background = getTooltipBackground()
      tooltip.style.color = getTextColor()
    }
  }
)

// ======================================================
// LIFECYCLE
// ======================================================

onMounted(async () => {
  await nextTick()

  await createChartView()

  resizeObserver = new ResizeObserver(
    () => {
      resizeChart()
    }
  )

  if (chartContainer.value) {
    resizeObserver.observe(
      chartContainer.value
    )
  }
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()

  cleanupChart()

  hideSpinner()
})
</script>

<style scoped>
.volume-chart-wrapper {
  display: flex;
  flex-direction: column;

  width: 100%;
  height: 100%;

  min-height: 0;
}

.toolbar {
  flex-shrink: 0;

  display: flex;
  justify-content: space-between;
  align-items: center;

  gap: 12px;

  padding: 8px;

  flex-wrap: wrap;
}

.frames {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.deltas {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;

  font-size: 12px;
}

.delta-item {
  display: flex;
  align-items: center;
  gap: 4px;

  cursor: pointer;
}

.chart-container {
  flex: 1;

  min-height: 0;

  position: relative;
}

:global(.chart-loading-overlay-dom) {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--p-content-background, transparent) 60%, transparent);
  z-index: 20;
  pointer-events: none;
}
</style>