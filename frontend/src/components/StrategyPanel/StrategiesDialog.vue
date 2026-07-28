<script setup lang="ts">
import { ref, watch, defineAsyncComponent, type Component } from 'vue'
import Dialog from 'primevue/dialog'
import { useToast } from 'primevue/usetoast'
import type { StrategyStatus } from '../../types'
import DefaultStrategyPanel from './DefaultStrategyPanel.vue'

const visible = defineModel<boolean>('visible', { required: true })

const toast = useToast()
const strategies = ref<StrategyStatus[]>([])
const loading = ref(false)

// Реестр веб-панелей по IDName стратегии - собственная реализация пакета
// (internal/strategy/<idName>), если она есть, иначе DefaultStrategyPanel
// (универсальные тумблеры по HasEnable/HasNotifications). Тот же принцип,
// что у Strategy.GetTelegramMenu() на бэкенде, просто со стороны фронта:
// новый пакет со своей панелью просто добавляет сюда одну строку.
const panelRegistry: Record<string, Component> = {
  anomaly: defineAsyncComponent(() => import('./anomaly/AnomalyPanel.vue')),
  base: defineAsyncComponent(() => import('./base/BasePanel.vue')),
}

const panelFor = (idName: string): Component => panelRegistry[idName] ?? DefaultStrategyPanel

const loadStrategies = async () => {
  loading.value = true
  try {
    const res = await fetch('/trade/api/getStrategiesStatus', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    })
    if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`)
    strategies.value = await res.json()
  } catch (err) {
    console.error('Error loading strategies:', err)
    toast.add({ severity: 'error', summary: 'Ошибка', detail: 'Не удалось загрузить список стратегий', life: 4000 })
  } finally {
    loading.value = false
  }
}

// Список должен быть свежим при каждом открытии - состояние тумблеров
// меняется и из Telegram, а не только отсюда.
watch(visible, (v) => {
  if (v) loadStrategies()
})

const toggleEnable = async (row: StrategyStatus, enabled: boolean) => {
  const prev = row.Enabled
  row.Enabled = enabled
  try {
    const res = await fetch('/trade/api/toggleStrategy', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ IDName: row.IDName, Enabled: enabled }),
    })
    if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`)
  } catch (err) {
    console.error('Error toggling strategy:', err)
    row.Enabled = prev
    toast.add({ severity: 'error', summary: 'Ошибка', detail: `Не удалось переключить стратегию ${row.Name}`, life: 4000 })
  }
}

const toggleNotifications = async (row: StrategyStatus, enabled: boolean) => {
  const prev = row.NotificationsEnabled
  row.NotificationsEnabled = enabled
  try {
    const res = await fetch('/trade/api/toggleStrategyNotifications', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ IDName: row.IDName, Enabled: enabled }),
    })
    if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`)
  } catch (err) {
    console.error('Error toggling strategy notifications:', err)
    row.NotificationsEnabled = prev
    toast.add({ severity: 'error', summary: 'Ошибка', detail: `Не удалось переключить уведомления ${row.Name}`, life: 4000 })
  }
}
</script>

<template>
  <Dialog v-model:visible="visible" header="Стратегии" modal :style="{ width: '28rem' }">
    <div v-if="loading" class="strategies-empty">Загрузка...</div>
    <div v-else-if="strategies.length === 0" class="strategies-empty">Нет доступных стратегий</div>
    <div v-else class="strategies-list">
      <component
        :is="panelFor(row.IDName)"
        v-for="row in strategies"
        :key="row.IDName"
        :row="row"
        @toggle-enable="(v: boolean) => toggleEnable(row, v)"
        @toggle-notifications="(v: boolean) => toggleNotifications(row, v)"
      />
    </div>
  </Dialog>
</template>

<style scoped>
.strategies-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.strategies-empty {
  color: var(--p-text-muted-color);
  padding: 1rem 0;
}
</style>
