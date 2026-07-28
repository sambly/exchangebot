<script setup lang="ts">
import { ref, watch, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import {
  createChart,
  CandlestickSeries,
  createSeriesMarkers,
  ColorType,
  type IChartApi,
  type ISeriesApi,
  type ISeriesMarkersPluginApi,
  type CandlestickData,
  type Time
} from 'lightweight-charts'
import { useOrdersStore } from '../../stores/orders'
import { useUIStore } from '../../stores/ui'
import Button from 'primevue/button'
import { toChartTime } from '../../utils/chartTime'

const props = defineProps<{
  pair: string
  darkMode?: boolean
  visible?: boolean
}>()

let isInitializing = false
let isDisposed = false

const store = useOrdersStore()
const ui = useUIStore()

const ordersForPair = computed(() =>
  [...store.active, ...store.history].filter(o => o.Pair === props.pair)
)

const hasOrders = computed(() => ordersForPair.value.length > 0)

const chartContainer = ref<HTMLDivElement | null>(null)
const frames = ['1m', '3m', '15m', '1h', '4h', '12h'] as const
const activeFrame = ref<string>('15m')

let chart: IChartApi | null = null
let candleSeries: ISeriesApi<'Candlestick'> | null = null
let markersPrimitive: ISeriesMarkersPluginApi<Time> | null = null
let resizeObserver: ResizeObserver | null = null
let isInitialized = false
let tooltip: HTMLDivElement | null = null
let spinnerEl: HTMLDivElement | null = null

const textColor = () => props.darkMode ? '#d1d4dc' : '#111'
const bgColor = () => props.darkMode ? '#131722' : '#ffffff'
const gridColor = () => props.darkMode ? '#2B2B43' : '#E1E3EA'

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

function waitForSize(): Promise<void> {
  return new Promise(resolve => {
    const check = () => {
      if (isDisposed) {
        resolve()
        return
      }
      const el = chartContainer.value
      if (el && el.clientWidth > 0 && el.clientHeight > 0) {
        resolve()
      } else {
        requestAnimationFrame(check)
      }
    }
    check()
  })
}

async function fetchCandles(pair: string, frame: string) {
  const res = await fetch('/trade/api/getChangeDelta', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ Pair: pair, Frame: frame })
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return await res.json()
}

// Акцентный цвет/размер для ордера, выбранного кликом по строке в
// OrdersActive/OrdersHistory (ui.selectedOrderId) - иначе среди десятков
// одинаковых зелёных/красных стрелок на графике невозможно найти именно ТУ
// сделку, ради которой сюда перешли.
const SELECTED_COLOR = '#f59e0b'

function buildMarkers(): any[] {
  const markers: any[] = []
  const selectedId = ui.selectedOrderId

  for (const order of ordersForPair.value) {
    const tOpen = toChartTime(order.TimeCreated)
    const tClose = order.Time ? toChartTime(order.Time) : null
    const isBuy = order.Side === 'BUY'
    const isClose = order.Status === 'Close'
    const isSelected = order.ID === selectedId

    markers.push({
      time: tOpen,
      position: isBuy ? 'belowBar' : 'aboveBar',
      color: isSelected ? SELECTED_COLOR : '#22c55e',
      shape: isBuy ? 'arrowUp' : 'arrowDown',
      text: `${isBuy ? 'long' : 'short'} #${order.ID}`,
      size: isSelected ? 2 : 1
    })

    if (isClose && tClose) {
      markers.push({
        time: tClose,
        position: isBuy ? 'aboveBar' : 'belowBar',
        color: isSelected ? SELECTED_COLOR : '#ef4444',
        shape: isBuy ? 'arrowDown' : 'arrowUp',
        text: `close #${order.ID}`,
        size: isSelected ? 2 : 1
      })
    }
  }

  return markers.sort((a, b) => (a.time as number) - (b.time as number))
}

function refreshMarkers() {
  if (!candleSeries) return
  const markers = buildMarkers()
  if (markersPrimitive) {
    markersPrimitive.setMarkers(markers)
  } else if (markers.length > 0) {
    markersPrimitive = createSeriesMarkers(candleSeries, markers)
  }
}

function initChart() {
  if (!chartContainer.value) return
  if (chart) {
    chart.remove()
    chart = null
    candleSeries = null
  }

  chart = createChart(chartContainer.value, {
    width: chartContainer.value.clientWidth,
    height: chartContainer.value.clientHeight,
    layout: {
      textColor: textColor(),
      background: { type: ColorType.Solid, color: bgColor() }
    },
    grid: {
      vertLines: { color: gridColor() },
      horzLines: { color: gridColor() }
    },
    crosshair: { mode: 0 },
    timeScale: {
      timeVisible: true,
      secondsVisible: false
    }
  })

  createTooltip()
}

function getTooltipBackground() {
  return props.darkMode
    ? 'rgba(19,23,34,0.95)'
    : 'rgba(255,255,255,0.95)'
}

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

  tooltip.style.background = getTooltipBackground()

  tooltip.style.color = textColor()

  tooltip.style.boxShadow = '0 2px 8px rgba(0,0,0,0.15)'

  chartContainer.value.appendChild(tooltip)
}

function attachCrosshair() {
  if (!chart || !tooltip) return

  chart.subscribeCrosshairMove(param => {
    if (!param.time) {
      tooltip!.style.display = 'none'
      return
    }

    tooltip!.style.display = 'block'

    const point = param.seriesData.get(candleSeries!) as any
    if (!point) return

    const fmt = (v: number) =>
      Number(v).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 6 })

    tooltip!.innerHTML = `
      <div style="font-weight:600;margin-bottom:6px;">
        ${props.pair}
      </div>
      <div style="color:#22c55e;margin-top:2px;">
        Open:  ${fmt(point.open)}
      </div>
      <div style="color:#22c55e;margin-top:2px;">
        High:  ${fmt(point.high)}
      </div>
      <div style="color:#ef4444;margin-top:2px;">
        Low:   ${fmt(point.low)}
      </div>
      <div style="color:#ef4444;margin-top:2px;">
        Close: ${fmt(point.close)}
      </div>
    `
  })
}

async function loadData() {
  if (!chart) return

  showSpinner()
  try {
    const raw = await fetchCandles(props.pair, activeFrame.value)
    if (!Array.isArray(raw) || raw.length === 0) return

    const candleData: CandlestickData[] = raw.map((item: any) => ({
      time: toChartTime(item.Time),
      open: item.Open,
      high: item.High,
      low: item.Low,
      close: item.Close
    }))

    if (candleSeries) {
      chart.removeSeries(candleSeries)
      candleSeries = null
      markersPrimitive = null
    }

    candleSeries = chart.addSeries(CandlestickSeries, {
      upColor: '#22c55e',
      downColor: '#ef4444',
      borderUpColor: '#22c55e',
      borderDownColor: '#ef4444',
      wickUpColor: '#22c55e',
      wickDownColor: '#ef4444'
    })

    candleSeries.setData(candleData)

    refreshMarkers()

    // scrollToRealTime вместо fitContent: последний укладывает ВСЮ историю в
    // область графика (свечи схлопываются в кашу при большом диапазоне),
    // а нам нужен вид на последние бары - как в обычном торговом терминале.
    chart.timeScale().scrollToRealTime()

    attachCrosshair()
  } catch (err) {
    console.error('loadData error:', err)
  } finally {
    hideSpinner()
  }
}

function scrollToLatest() {
  chart?.timeScale().scrollToRealTime()
}

async function initialize() {
  if (isInitializing) return
  isInitializing = true
  try {
    await nextTick()
    await waitForSize()
    if (isDisposed) return
    initChart()
    await loadData()
    isInitialized = true
  } finally {
    isInitializing = false
  }
}

function resizeChart() {
  if (!chart || !chartContainer.value) return
  chart.applyOptions({
    width: chartContainer.value.clientWidth,
    height: chartContainer.value.clientHeight
  })
}

watch(() => props.visible, async (val) => {
  if (!val) return
  isInitialized = false
  await initialize()
})

watch([() => props.pair, activeFrame], async () => {
  if (!props.visible) return
  if (!isInitialized) {
    await initialize()
  } else {
    await loadData()
  }
})

watch(() => ui.selectedOrderId, () => {
  refreshMarkers()
})

watch(() => props.darkMode, () => {
  if (!chart) return
  chart.applyOptions({
    layout: {
      textColor: textColor(),
      background: { type: ColorType.Solid, color: bgColor() }
    },
    grid: {
      vertLines: { color: gridColor() },
      horzLines: { color: gridColor() }
    }
  })
  if (tooltip) {
    tooltip.style.background = getTooltipBackground()
    tooltip.style.color = textColor()
  }
})

onMounted(() => {
  resizeObserver = new ResizeObserver(() => resizeChart())
  if (chartContainer.value) {
    resizeObserver.observe(chartContainer.value)
  }
  if (props.visible) {
    initialize()
  }
})

onBeforeUnmount(() => {
  isDisposed = true
  resizeObserver?.disconnect()
  if (chart) {
    chart.remove()
    chart = null
    candleSeries = null
    markersPrimitive = null
  }
  tooltip?.remove()
  hideSpinner()
})
</script>

<template>
  <div class="orders-chart-wrapper">
    <div class="frame-toolbar">
      <Button
        v-for="f in frames"
        :key="f"
        :label="f"
        size="small"
        severity="secondary"
        :outlined="activeFrame !== f"
        @click="activeFrame = f"
      />
      <Button
        label="К последним"
        icon="pi pi-angle-double-right"
        size="small"
        severity="secondary"
        text
        style="margin-left: auto"
        @click="scrollToLatest"
      />
    </div>
    <div ref="chartContainer" class="chart-container">
      <div v-if="!hasOrders" class="no-orders-placeholder">
        Нет ордеров
      </div>
    </div>
  </div>
</template>

<style scoped>
.orders-chart-wrapper {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-height: 0;
}

.frame-toolbar {
  flex-shrink: 0;
  display: flex;
  gap: 0.25rem;
  padding: 0.25rem 0.5rem;
}

.chart-container {
  flex: 1;
  min-height: 0;
  position: relative;
  overflow: hidden;
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

.no-orders-placeholder {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--p-text-muted-color, #999);
  font-size: 0.9rem;
  background: var(--p-content-background, #fff);
  z-index: 15;
}
</style>