<script setup lang="ts">
import Checkbox from 'primevue/checkbox'
import type { StrategyStatus } from '../../types'

// Fallback-панель для стратегий без своего компонента в реестре (см.
// StrategiesDialog.vue) - универсальный ряд тумблеров по HasEnable/
// HasNotifications, без ничего специфичного для конкретного пакета.
defineProps<{ row: StrategyStatus }>()

const emit = defineEmits<{
  'toggle-enable': [boolean]
  'toggle-notifications': [boolean]
}>()
</script>

<template>
  <div class="strategy-panel">
    <div class="strategy-panel-name">{{ row.Name }}</div>

    <label v-if="row.HasEnable" class="strategy-row">
      <Checkbox
        :model-value="row.Enabled"
        :binary="true"
        size="small"
        @update:model-value="(v) => emit('toggle-enable', !!v)"
      />
      <span>Стратегия</span>
    </label>

    <label v-if="row.HasNotifications" class="strategy-row">
      <Checkbox
        :model-value="row.NotificationsEnabled"
        :binary="true"
        size="small"
        @update:model-value="(v) => emit('toggle-notifications', !!v)"
      />
      <span>Уведомления</span>
    </label>
  </div>
</template>

<style scoped>
.strategy-panel {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.strategy-panel-name {
  font-weight: 600;
}

.strategy-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
}
</style>
