<script setup lang="ts">
import { computed } from 'vue'
import Select from 'primevue/select'
import { useMarketStore } from '../../stores/market'
import { useUIStore } from '../../stores/ui'

const market = useMarketStore()
const ui = useUIStore()

const pairOptions = computed(() =>
  Object.keys(market.changePrices).map(p => p.replace('USDT', ''))
)

const selectedOption = computed(() => ui.currentPair.replace('USDT', ''))

function onPairChange(displayName: string) {
  ui.selectPair(displayName + 'USDT')
}

const priceTop = computed(() => {
  const stat = market.marketsStat[ui.currentPair]
  return stat?.Price != null
    ? stat.Price.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 6 })
    : ''
})

const ch24Top = computed(() => {
  const stat = market.marketsStat[ui.currentPair]
  return stat?.Ch24 != null
    ? stat.Ch24.toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' }) + '%'
    : ''
})

const volumeTop = computed(() => {
  const stat = market.marketsStat[ui.currentPair]
  return stat?.Volume != null
    ? stat.Volume.toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' })
    : ''
})

const ch24Color = computed(() => {
  const val = market.marketsStat[ui.currentPair]?.Ch24
  if (val == null) return 'inherit'
  return val > 0 ? 'var(--p-green-500)' : val < 0 ? 'var(--p-red-500)' : 'inherit'
})

// Статус подписки exchange_service на текущую пару.
// Три состояния: активна (зелёный), неактивна (красный), неизвестно
// (серый - exchange_service ещё не ответил или это не выбранная пара).
const feedStatus = computed(() => market.feedStatus[ui.currentPair])

const feedStatusColor = computed(() => {
  switch (feedStatus.value) {
    case 'Active':
      return 'var(--p-green-500)'
    case 'Inactive':
      return 'var(--p-red-500)'
    default:
      return 'var(--p-surface-400)'
  }
})

const feedStatusTitle = computed(() => {
  switch (feedStatus.value) {
    case 'Active':
      return 'Фид активен: данные по паре идут'
    case 'Inactive':
      return 'Фид неактивен: подписка на паре оборвана'
    default:
      return 'Статус фида неизвестен'
  }
})
</script>

<template>
  <div class="header-info">
    <div class="header-info-left">
      <div class="pair-selector">
        <span
          class="feed-status-dot"
          :style="{ backgroundColor: feedStatusColor }"
          :title="feedStatusTitle"
        />
        <Select
          :options="pairOptions"
          :model-value="selectedOption"
          @update:model-value="onPairChange"
          placeholder="Пара"
          :filter="true"
          :showClear="false"
          size="small"
          class="pair-select"
        />
      </div>
      <div class="stats-row">
        <div class="stat-item">
          <span class="stat-label">Цена:</span>
          <span class="stat-value">{{ priceTop }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">24часа:</span>
          <span class="stat-value" :style="{ color: ch24Color }">{{ ch24Top }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">Объем:</span>
          <span class="stat-value">{{ volumeTop }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.header-info {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  flex-wrap: wrap;
  height: auto;
}

.header-info-left {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.5rem 1.5rem;
}

.pair-selector {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.feed-status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
  transition: background-color 0.3s ease;
}

.stats-row {
  display: flex;
  gap: 1rem;
}

.stat-item {
  display: flex;
  gap: 0.25rem;
  align-items: center;
  font-size: 0.85rem;
}

.stat-label {
  white-space: nowrap;
}

.stat-value {
  font-weight: 700;
}

.pair-select {
  width: 160px;
}
</style>