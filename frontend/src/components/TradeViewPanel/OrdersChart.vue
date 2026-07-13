<script setup lang="ts">
import { ref, watch, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import {
  createChart,
  CandlestickSeries,
  createSeriesMarkers,
  ColorType,
  type IChartApi,
  type ISeriesApi,
  type CandlestickData,
  type Time
} from 'lightweight-charts'
import { useOrdersStore } from '../../stores/orders'
import Button from 'primevue/button'

const props = defineProps<{
  pair: string
  darkMode?: boolean
  visible?: boolean
}>()

let isLoading = false

const store = useOrdersStore()

const ordersForPair = computed(() =>
  [...store.active, ...store.history].filter(o => o.Pair === props.pair)
)

const chartContainer = ref<HTMLDivElement | null>(null)
const frames = ['1m', '3m', '15m', '1h', '4h', '1d'] as const
const activeFrame = ref<string>('15m')

let chart: IChartApi | null = null
let candleSeries: ISeriesApi<'Candlestick'> | null = null
let resizeObserver: ResizeObserver | null = null
let isInitialized = false

const textColor = () => props.darkMode ? '#d1d4dc' : '#111'
const bgColor = () => props.darkMode ? '#131722' : '#ffffff'
const gridColor = () => props.darkMode ? '#2B2B43' : '#E1E3EA'

function waitForSize(): Promise<void> {
  return new Promise(resolve => {
    const check = () => {
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

function buildMarkers(): any[] {
  const markers: any[] = []

  for (const order of ordersForPair.value) {
    const tOpen = Math.floor(new Date(order.TimeCreated || 0).getTime() / 1000) as Time
    const tClose = order.Time
      ? Math.floor(new Date(order.Time).getTime() / 1000) as Time
      : null
    const isBuy = order.Side === 'BUY'
    const isClose = order.Status === 'Close'

    markers.push({
      time: tOpen,
      position: isBuy ? 'belowBar' : 'aboveBar',
      color: '#22c55e',
      shape: isBuy ? 'arrowUp' : 'arrowDown',
      text: `${isBuy ? 'long' : 'short'} #${order.ID}`
    })

    if (isClose && tClose) {
      markers.push({
        time: tClose,
        position: isBuy ? 'aboveBar' : 'belowBar',
        color: '#ef4444',
        shape: isBuy ? 'arrowDown' : 'arrowUp',
        text: `close #${order.ID}`
      })
    }
  }

  return markers.sort((a, b) => (a.time as number) - (b.time as number))
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
}

async function loadData() {
  if (!chart) return

  try {
    const raw = await fetchCandles(props.pair, activeFrame.value)
    if (!Array.isArray(raw) || raw.length === 0) return

    const candleData: CandlestickData[] = raw.map((item: any) => ({
      time: Math.floor(new Date(item.Time).getTime() / 1000) as Time,
      open: item.Open,
      high: item.High,
      low: item.Low,
      close: item.Close
    }))

    if (candleSeries) {
      chart.removeSeries(candleSeries)
      candleSeries = null
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

    const markers = buildMarkers()
    if (markers.length > 0) {
      createSeriesMarkers(candleSeries, markers)
    }

    chart.timeScale().fitContent()
  } catch (err) {
    console.error('loadData error:', err)
  }
}

async function initialize() {
  if (isLoading) return
  isLoading = true
  try {
    await nextTick()
    await waitForSize()
    initChart()
    await loadData()
    isInitialized = true
  } finally {
    isLoading = false
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
  resizeObserver?.disconnect()
  if (chart) {
    chart.remove()
    chart = null
  }
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
    </div>
    <div ref="chartContainer" class="chart-container" />
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
</style>