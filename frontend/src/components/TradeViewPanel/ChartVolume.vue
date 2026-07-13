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
  type ISeriesApi,
  type Time
} from 'lightweight-charts'

import {
  ref,
  watch,
  onMounted,
  onBeforeUnmount,
  nextTick
} from 'vue'

import Button from 'primevue/button'

const props = defineProps<{
  pair: string
  darkMode?: boolean
}>()

const chartContainer = ref<HTMLDivElement | null>(null)

const selectedFrame = ref('1h')

const selectedDeltas = ref<string[]>([
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
  '1d'
]

const deltaOptions = [
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

let chart: IChartApi | null = null

const seriesMap = new Map<string, ISeriesApi<'Line'>>()

let resizeObserver: ResizeObserver | null = null
let tooltip: HTMLDivElement | null = null

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

    let index = 0

    for (const [key, series] of seriesMap.entries()) {
      const point =
        param.seriesData.get(series) as any

      const value =
        point?.value !== undefined
          ? Number(point.value).toLocaleString(
              'en-US',
              {
                maximumFractionDigits: 0
              }
            )
          : '—'

      html += `
        <div style="
          color:${colors[index % colors.length]};
          margin-top:4px;
        ">
          ${key}: ${value}
        </div>
      `

      index++
    }

    tooltip!.innerHTML = html
  })
}

// ======================================================
// CREATE CHART
// ======================================================

async function createChartView() {
  if (!chartContainer.value) return

  const raw = await fetchData()

  if (!Array.isArray(raw)) return

  chartContainer.value.innerHTML = ''

  seriesMap.clear()

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

      rightPriceScale: {
        scaleMargins: {
          top: 0.1,
          bottom: 0.1
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

  for (const delta of selectedDeltas.value) {
    const series = chart.addSeries(
      LineSeries,
      {
        color:
          colors[
            colorIndex % colors.length
          ],

        lineWidth: 2
      }
    )

    const data = raw.map((item: any) => ({
      time: Math.floor(
        new Date(item.Time).getTime() / 1000
      ) as Time,

      value: Number(item[delta])
    }))

    series.setData(data)

    seriesMap.set(delta, series)

    colorIndex++
  }

  chart.timeScale().fitContent()

  attachCrosshair()
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

  if (chart) {
    chart.remove()
    chart = null
  }

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

  if (chart) {
    chart.remove()
    chart = null
  }

  tooltip?.remove()
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
</style>