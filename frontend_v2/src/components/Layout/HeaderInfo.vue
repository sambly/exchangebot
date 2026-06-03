<script setup lang="ts">
import { inject, computed, type Ref } from 'vue'
import type { MarketsStat, ChangePrices } from '../types'
import Select from 'primevue/select'

const marketsStat = inject<Ref<MarketsStat>>('marketsStat')!
const changePrices = inject<Ref<ChangePrices>>('changePrices')!
const currentPair = inject<Ref<string>>('currentPair')!
const selectPair = inject<(pair: string) => void>('selectPair')!

const pairOptions = computed(() => {
  const pairs = Object.keys(changePrices.value || {})
  return pairs.map(p => p.replace('USDT', ''))
})

const selectedOption = computed(() => {
  return (currentPair.value || '').replace('USDT', '')
})

function onPairChange(displayName: string) {
  selectPair(displayName + 'USDT')
}

const ch24Top = computed(() => {
  const stat = marketsStat.value?.[currentPair.value]
  return stat?.Ch24 != null
    ? stat.Ch24.toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' }) + '%'
    : ''
})

const volumeTop = computed(() => {
  const stat = marketsStat.value?.[currentPair.value]
  return stat?.Volume != null
    ? stat.Volume.toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' })
    : ''
})

const ch24Color = computed(() => {
  const val = marketsStat.value?.[currentPair.value]?.Ch24
  if (val == null) return 'inherit'
  return val > 0 ? 'var(--p-green-500)' : val < 0 ? 'var(--p-red-500)' : 'inherit'
})
</script>

<template>
  <div class="header-info">
    <div class="header-info-left">
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
      <div class="stats-row">
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
  gap: 1.5rem;
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