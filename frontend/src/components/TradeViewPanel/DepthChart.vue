<script setup lang="ts">
import {
  createChart,
  AreaSeries,
  LineType,
  ColorType,
  type IChartApi,
  type ISeriesApi,
  type Time
} from 'lightweight-charts'
import { ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'

const props = defineProps<{
  pair: string
  darkMode?: boolean
}>()

interface DepthLevel {
  Price: number
  Quantity: number
}
interface DepthResponse {
  Ready: boolean
  Bids: DepthLevel[] | null
  Asks: DepthLevel[] | null
}

// У lightweight-charts нет оси "по цене" - ось X всегда Time. Стандартный
// трюк для cumulative depth chart: кодируем цену числом и подсовываем вместо
// time, а подписи/тултип декодируем обратно форматтерами (formatPrice).
// Масштаб 1e8 покрывает и цены в доли цента (мемкоины), и BTC-диапазон без
// потери значащих цифр, и не вылезает за пределы safe integer.
const PRICE_SCALE = 1e8

function encodePrice(price: number): Time {
  return Math.round(price * PRICE_SCALE) as Time
}
function decodePrice(time: Time): number {
  return (time as number) / PRICE_SCALE
}
function formatPrice(time: Time): string {
  return decodePrice(time).toLocaleString('en-US', { maximumFractionDigits: 8 })
}

const chartContainer = ref<HTMLDivElement | null>(null)
const hasData = ref(false)

let chart: IChartApi | null = null
let bidSeries: ISeriesApi<'Area'> | null = null
let askSeries: ISeriesApi<'Area'> | null = null
let pollTimer: ReturnType<typeof setInterval> | null = null
let tooltip: HTMLDivElement | null = null
let spinnerEl: HTMLDivElement | null = null
let resizeObserver: ResizeObserver | null = null

// Нет пуша обновлений стакана в веб-сокет (см. обсуждение) - опрашиваем сами.
// Секунда - тот же порядок троттлинга, что и у пуша ордеров (pushInterval в
// orderService.go), более частый опрос ради графика глубины смысла не имеет.
const POLL_INTERVAL_MS = 1000

function getTextColor() {
  return props.darkMode ? '#d1d4dc' : '#111111'
}
function getBackgroundColor() {
  return props.darkMode ? '#131722' : '#ffffff'
}
function getGridColor() {
  return props.darkMode ? '#2B2B43' : '#E1E3EA'
}
function getTooltipBackground() {
  return props.darkMode ? 'rgba(19,23,34,0.95)' : 'rgba(255,255,255,0.95)'
}

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
  tooltip.style.color = getTextColor()
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

    const bidPoint = bidSeries ? (param.seriesData.get(bidSeries) as any) : undefined
    const askPoint = askSeries ? (param.seriesData.get(askSeries) as any) : undefined

    let html = `<div style="font-weight:600;margin-bottom:6px;">${props.pair}</div>`
    html += `<div>Цена: ${formatPrice(param.time as Time)}</div>`
    if (bidPoint?.value !== undefined) {
      html += `<div style="color:#22c55e;margin-top:4px;">Bid, накоплено: ${Number(bidPoint.value).toLocaleString('en-US', { maximumFractionDigits: 4 })}</div>`
    }
    if (askPoint?.value !== undefined) {
      html += `<div style="color:#ef4444;margin-top:4px;">Ask, накоплено: ${Number(askPoint.value).toLocaleString('en-US', { maximumFractionDigits: 4 })}</div>`
    }
    tooltip!.innerHTML = html
  })
}

// Бэкенд отдаёт bids отсортированными по убыванию цены (лучший бид первый).
// Копим объём от спреда вглубь, а для графика разворачиваем в порядок
// возрастания цены - lightweight-charts требует строго возрастающий time.
function cumulativeBids(levels: DepthLevel[]) {
  let cum = 0
  const points = levels.map(l => {
    cum += l.Quantity
    return { time: encodePrice(l.Price), value: cum }
  })
  return points.reverse()
}

// asks уже приходят по возрастанию цены (лучший аск первый) - тот же порядок,
// что нужен графику, разворачивать не нужно.
function cumulativeAsks(levels: DepthLevel[]) {
  let cum = 0
  return levels.map(l => {
    cum += l.Quantity
    return { time: encodePrice(l.Price), value: cum }
  })
}

async function fetchDepth(): Promise<DepthResponse> {
  const res = await fetch('/trade/api/getDepth', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ Pair: props.pair })
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return await res.json()
}

function ensureChart() {
  if (chart || !chartContainer.value) return

  chart = createChart(chartContainer.value, {
    width: chartContainer.value.clientWidth,
    height: chartContainer.value.clientHeight,
    layout: {
      textColor: getTextColor(),
      background: { type: ColorType.Solid, color: getBackgroundColor() }
    },
    grid: {
      vertLines: { color: getGridColor() },
      horzLines: { color: getGridColor() }
    },
    localization: {
      timeFormatter: (time: Time) => formatPrice(time)
    },
    timeScale: {
      tickMarkFormatter: (time: Time) => formatPrice(time)
    }
  })

  bidSeries = chart.addSeries(AreaSeries, {
    lineColor: '#22c55e',
    topColor: 'rgba(34, 197, 94, 0.3)',
    bottomColor: 'rgba(34, 197, 94, 0)',
    lineType: LineType.WithSteps,
    lineWidth: 2,
    lastValueVisible: false,
    priceLineVisible: false
  })

  askSeries = chart.addSeries(AreaSeries, {
    lineColor: '#ef4444',
    topColor: 'rgba(239, 68, 68, 0.3)',
    bottomColor: 'rgba(239, 68, 68, 0)',
    lineType: LineType.WithSteps,
    lineWidth: 2,
    lastValueVisible: false,
    priceLineVisible: false
  })

  createTooltip()
  attachCrosshair()
}

async function refresh() {
  if (!chartContainer.value) return
  try {
    const data = await fetchDepth()
    const wasEmpty = !hasData.value
    hasData.value = !!data.Ready && !!((data.Bids?.length ?? 0) || (data.Asks?.length ?? 0))

    if (!hasData.value) return

    ensureChart()
    if (!bidSeries || !askSeries || !chart) return

    bidSeries.setData(cumulativeBids(data.Bids || []))
    askSeries.setData(cumulativeAsks(data.Asks || []))

    if (wasEmpty) {
      chart.timeScale().fitContent()
    }
  } catch (err) {
    console.error('Error loading depth:', err)
  }
}

async function initialLoad() {
  showSpinner()
  try {
    await refresh()
  } finally {
    hideSpinner()
  }
}

function resizeChart() {
  if (!chart || !chartContainer.value) return
  chart.applyOptions({
    width: chartContainer.value.clientWidth,
    height: chartContainer.value.clientHeight
  })
}

function cleanupChart() {
  if (chart) {
    chart.remove()
    chart = null
    bidSeries = null
    askSeries = null
  }
  tooltip?.remove()
  tooltip = null
}

watch(() => props.pair, () => {
  cleanupChart()
  hasData.value = false
  initialLoad()
})

watch(() => props.darkMode, () => {
  if (!chart) return
  chart.applyOptions({
    layout: {
      textColor: getTextColor(),
      background: { type: ColorType.Solid, color: getBackgroundColor() }
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
})

onMounted(async () => {
  await nextTick()
  await initialLoad()

  resizeObserver = new ResizeObserver(() => resizeChart())
  if (chartContainer.value) {
    resizeObserver.observe(chartContainer.value)
  }

  pollTimer = setInterval(refresh, POLL_INTERVAL_MS)
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
  resizeObserver?.disconnect()
  cleanupChart()
  hideSpinner()
})
</script>

<template>
  <div class="depth-chart-wrapper">
    <div ref="chartContainer" class="chart-container">
      <div v-if="!hasData" class="no-depth-placeholder">
        Глубина не отслеживается для пары {{ pair }} (настраивается в config.yaml: depth.pairs)
      </div>
    </div>
  </div>
</template>

<style scoped>
.depth-chart-wrapper {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-height: 0;
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

.no-depth-placeholder {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 1rem;
  color: var(--p-text-muted-color, #999);
  font-size: 0.9rem;
  background: var(--p-content-background, #fff);
  z-index: 15;
}
</style>
