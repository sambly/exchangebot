<template>
  <div ref="chartContainer" class="tradingview-container"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'

const props = defineProps<{
  pair: string;
  darkMode?: boolean;
  visible?: boolean;
}>()

const chartContainer = ref<HTMLDivElement | null>(null)
let widget: any = null
let isWidgetReady = false

function loadTV(): Promise<void> {
  return new Promise((resolve, reject) => {
    if ((window as any).TradingView) {
      resolve()
      return
    }
    const existingScript = document.getElementById('tradingview-script')
    if (existingScript) {
      existingScript.addEventListener('load', () => resolve())
      existingScript.addEventListener('error', (e) => reject(e))
      return
    }
    const script = document.createElement('script')
    script.id = 'tradingview-script'
    script.src = 'https://s3.tradingview.com/tv.js'
    script.async = true
    script.onload = () => resolve()
    script.onerror = (e) => reject(e)
    document.head.appendChild(script)
  })
}

function getTheme() {
  return props.darkMode ? 'dark' : 'light'
}

async function createWidget() {
  if (!chartContainer.value) return

  if (!chartContainer.value.id) {
    chartContainer.value.id = 'tv_' + Math.random().toString(36).slice(2, 9)
  }

  const width = chartContainer.value.clientWidth
  const height = chartContainer.value.clientHeight
  if (width === 0 || height === 0) return

  // Если виджет уже есть — удаляем
  if (widget) {
    try { widget.remove() } catch (e) {}
    widget = null
    isWidgetReady = false
  }

  widget = new (window as any).TradingView.widget({
    container_id: chartContainer.value.id,
    symbol: `BINANCE:${props.pair}`,
    interval: '15',
    timezone: 'Europe/Moscow',
    theme: getTheme(),
    style: '1',
    locale: 'ru',
    toolbar_bg: props.darkMode ? '#131722' : '#f1f3f6',
    enable_publishing: false,
    allow_symbol_change: true,
    width: width,
    height: height,
    loading_screen: {
      backgroundColor: props.darkMode ? '#131722' : '#ffffff'
    },
    autosize: true,
    // onChartReady передаём прямо в конфиг — это правильный способ
    charts_storage_api_version: '1.1',
  })

  // Правильный способ дождаться готовности
  if (typeof widget.onChartReady === 'function') {
    widget.onChartReady(() => {
      isWidgetReady = true
    })
  } else {
    // Fallback — polling
    const check = setInterval(() => {
      if (widget && typeof widget.onChartReady === 'function') {
        clearInterval(check)
        widget.onChartReady(() => {
          isWidgetReady = true
        })
      }
    }, 100)
  }
}

// Смена пары — без пересоздания
watch(() => props.pair, (newPair) => {
  if (widget && isWidgetReady) {
    widget.setSymbol(`BINANCE:${newPair}`, () => {
      console.log('Symbol changed to', newPair)
    })
  } else {
    // Если виджет ещё не готов или его нет — пересоздаём
    createWidget()
  }
})

// Компонент живёт на v-show (не v-if, как остальные графики) - контейнер
// прячется через display:none, но сам он не размонтируется, и watch(pair)
// выше исправно вызывает setSymbol() даже пока вкладка скрыта. На практике
// смена символа на скрытом (0×0) контейнере не применяется - виджет так и
// показывает старую пару, пока его не пересоздать. Полумера с повторным
// setSymbol() на возврате видимости не помогла - поэтому здесь жёстче:
// при возврате видимости виджет пересоздаётся заново, уже с актуальным
// props.pair. nextTick - чтобы дождаться, пока v-show реально уберёт
// display:none и контейнер получит ненулевые размеры (иначе createWidget
// молча выходит по проверке width/height === 0).
watch(() => props.visible, async (visible) => {
  if (!visible) return
  await nextTick()
  createWidget()
})

// Смена темы — виджет встраиваемый (tv.js), changeTheme() у него не работает
// надёжно (toolbar_bg/loading_screen всё равно заданы только при создании),
// поэтому пересоздаём виджет целиком с актуальной темой
watch(() => props.darkMode, () => {
  createWidget()
})

onBeforeUnmount(() => {
  if (widget) {
    try { widget.remove() } catch (e) {}
    widget = null
  }
  isWidgetReady = false
})

onMounted(async () => {
  await loadTV()
  setTimeout(() => createWidget(), 100)
})
</script>

<style scoped>
.tradingview-container {
  width: 100%;
  height: 100%;
  position: relative;
}
</style>