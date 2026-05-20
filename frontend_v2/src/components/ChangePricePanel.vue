<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'

interface ChangePriceData {
  ChangePercent: number
  Price: number
}

interface ChangePrices {
  [pair: string]: {
    '1m': ChangePriceData
    '3m': ChangePriceData
    '15m': ChangePriceData
    '1h': ChangePriceData
    '4h': ChangePriceData
    '1d': ChangePriceData
  }
}

interface PriceData {
  pair: string
  volume: number
  ch24: number
  '1m': number
  '3m': number
  '15m': number
  '1h': number
  '4h': number
  '1d': number
  isFavorite: boolean
}

const filterMode = ref<'all' | 'favorites'>('all')
const pairs = ref<PriceData[]>([])
const favoritePairs = ref<Set<string>>(new Set())
const isLoading = ref(false)
const error = ref<string | null>(null)

const TIME_PERIODS = ['1m', '3m', '15m', '1h', '4h', '1d'] as const

const emit = defineEmits<{
  (e: 'select-pair', pair: string): void
}>()

const fetchChangePrices = async () => {
  const response = await fetch('/trade/api/getChPrice', {
    method: 'GET',
    headers: { 'Content-Type': 'application/json' }
  })
  if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`)
  return await response.json()
}

const loadPricesData = async () => {
  isLoading.value = true
  error.value = null
  try {
    const data = await fetchChangePrices()
    const marketsStat = data.MarketsStat
    const changePrices = data.ChangePrices
    const newPairs: PriceData[] = []
    for (const pair in changePrices) {
      newPairs.push({
        pair: pair.replace('USDT', ''),
        volume: marketsStat[pair]?.Volume || 0,
        ch24: marketsStat[pair]?.Ch24 || 0,
        '1m': changePrices[pair]['1m']?.ChangePercent || 0,
        '3m': changePrices[pair]['3m']?.ChangePercent || 0,
        '15m': changePrices[pair]['15m']?.ChangePercent || 0,
        '1h': changePrices[pair]['1h']?.ChangePercent || 0,
        '4h': changePrices[pair]['4h']?.ChangePercent || 0,
        '1d': changePrices[pair]['1d']?.ChangePercent || 0,
        isFavorite: favoritePairs.value.has(pair)
      })
    }
    pairs.value = newPairs
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Ошибка загрузки данных'
    console.error('Error loading price data:', err)
  } finally {
    isLoading.value = false
  }
}

const loadFavorites = () => {
  const stored = localStorage.getItem('favoritePairs')
  if (stored) {
    try { favoritePairs.value = new Set(JSON.parse(stored)) } catch { favoritePairs.value = new Set() }
  }
}
const saveFavorites = () => {
  localStorage.setItem('favoritePairs', JSON.stringify(Array.from(favoritePairs.value)))
}

const toggleFavorite = (pair: string) => {
  const pairFull = pair + 'USDT'
  if (favoritePairs.value.has(pairFull)) favoritePairs.value.delete(pairFull)
  else favoritePairs.value.add(pairFull)
  const idx = pairs.value.findIndex(p => p.pair === pair)
  if (idx !== -1) pairs.value[idx].isFavorite = favoritePairs.value.has(pairFull)
  saveFavorites()
}

const filteredPairs = computed(() => {
  if (filterMode.value === 'favorites') return pairs.value.filter(p => favoritePairs.value.has(p.pair + 'USDT'))
  return pairs.value
})

const getChangeClass = (value: number) => value > 0 ? 'positive' : value < 0 ? 'negative' : 'neutral'

const formatVolume = (volume: number) => {
  if (volume >= 1_000_000_000) return (volume / 1_000_000_000).toFixed(2) + 'B'
  if (volume >= 1_000_000) return (volume / 1_000_000).toFixed(2) + 'M'
  if (volume >= 1_000) return (volume / 1_000).toFixed(2) + 'K'
  return volume.toString()
}

const onRowClick = (event: any) => {
  emit('select-pair', event.data.pair + 'USDT')
}

const refresh = () => loadPricesData()
defineExpose({ refresh })

onMounted(() => {
  loadFavorites()
  loadPricesData()
})
</script>

<template>
  <div class="price-table-wrapper">
    <div class="table-header">
      <div class="filter-buttons">
        <button :class="['filter-btn', { active: filterMode === 'all' }]" @click="filterMode = 'all'">Все пары</button>
        <button :class="['filter-btn', { active: filterMode === 'favorites' }]" @click="filterMode = 'favorites'">♥ Избранные</button>
      </div>
    </div>

    <div v-if="error" class="error-message">
      {{ error }}
      <button @click="refresh" class="retry-btn">Повторить</button>
    </div>

    <div v-else class="table-container">
      <DataTable
        :value="filteredPairs"
        :loading="isLoading"
        :scrollable="true"
        scrollHeight="flex"
        class="price-table"
        dataKey="pair"
        @row-click="onRowClick"
      >
        <Column header="" style="width: 3rem">
          <template #body="{ data }">
            <button @click="toggleFavorite(data.pair)" class="favorite-btn" :class="{ active: data.isFavorite }">
              {{ data.isFavorite ? '♥' : '♡' }}
            </button>
          </template>
        </Column>

        <Column field="pair" header="Пара" :sortable="true" style="min-width: 80px">
          <template #body="{ data }"><span class="pair-name">{{ data.pair }}</span></template>
        </Column>

        <Column field="volume" header="Объем 24h" :sortable="true" style="min-width: 100px">
          <template #body="{ data }">{{ formatVolume(data.volume) }}</template>
        </Column>

        <Column field="ch24" header="24h %" :sortable="true" style="min-width: 80px">
          <template #body="{ data }">
            <span :class="['change-value', getChangeClass(data.ch24)]">{{ data.ch24 > 0 ? '+' : '' }}{{ data.ch24.toFixed(2) }}%</span>
          </template>
        </Column>

        <Column v-for="period in TIME_PERIODS" :key="period" :field="period" :header="period" :sortable="true" style="min-width: 70px">
          <template #body="{ data }">
            <span :class="['change-value', getChangeClass(data[period])]">
              {{ data[period] > 0 ? '+' : '' }}{{ data[period].toFixed(2) }}%
            </span>
          </template>
        </Column>
      </DataTable>
    </div>
  </div>
</template>

<style scoped>
.price-table-wrapper {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: white;
  overflow: hidden;
}

.table-header {
  flex-shrink: 0;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--border-color, #e5e7eb);
  display: flex;
  justify-content: flex-end;
}

.filter-buttons {
  display: flex;
  gap: 0.25rem;
}

.filter-btn {
  padding: 0.3rem 0.6rem;
  border: 1px solid var(--border-color, #d1d5db);
  background: white;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.8rem;
  transition: all 0.2s;
}
.filter-btn:hover { background: var(--surface-50, #f3f4f6); }
.filter-btn.active {
  background: var(--primary-color, #3b82f6);
  color: white;
  border-color: var(--primary-color, #3b82f6);
}

.error-message {
  flex-shrink: 0;
  margin: 1rem;
  padding: 0.75rem;
  background: #fee2e2;
  color: #dc2626;
  border-radius: 6px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.retry-btn { padding: 0.25rem 0.75rem; background: #dc2626; color: white; border: none; border-radius: 4px; cursor: pointer; }

.table-container {
  flex: 1;
  overflow: hidden;
  position: relative;
}

:deep(.price-table) { height: 100%; }
:deep(.p-datatable-wrapper) { height: 100%; overflow: auto; }
:deep(.p-datatable-thead > tr > th) {
  background-color: #f9fafb;
  padding: 0.5rem 0.4rem;
  font-weight: 600;
  color: var(--text-color, #374151);
  border-bottom: 2px solid var(--border-color, #e5e7eb);
  position: sticky;
  top: 0;
}
:deep(.p-datatable-tbody > tr) { cursor: pointer; transition: background-color 0.2s; }
:deep(.p-datatable-tbody > tr:hover) { background-color: var(--surface-50, #f3f4f6); }
:deep(.p-datatable-tbody > tr > td) { padding: 0.35rem 0.4rem; border-bottom: 1px solid var(--border-color, #e5e7eb); }

.favorite-btn { background: none; border: none; font-size: 1.1rem; cursor: pointer; padding: 0.15rem; transition: transform 0.2s; }
.favorite-btn:hover { transform: scale(1.1); }
.favorite-btn.active { color: #ef4444; }

.pair-name { font-weight: 500; }

.change-value {
  font-weight: 500;
  padding: 0.2rem 0.4rem;
  border-radius: 4px;
  display: inline-block;
}
.change-value.positive { color: #10b981; }
.change-value.negative { color: #ef4444; }
.change-value.neutral { color: #6b7280; }
</style>