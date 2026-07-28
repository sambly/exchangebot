<script setup lang="ts">
import Checkbox from 'primevue/checkbox'
import type { StrategyStatus } from '../../../types'

// Собственная веб-панель пакета base (см. internal/strategy/base) -
// зарегистрирована в StrategiesDialog.vue по IDName.
defineProps<{ row: StrategyStatus }>()

const emit = defineEmits<{
  'toggle-enable': [boolean]
  'toggle-notifications': [boolean]
}>()
</script>

<template>
  <div class="strategy-panel">
    <div class="strategy-panel-name">{{ row.Name }}</div>
    <div class="strategy-panel-hint">Базовый детектор роста/падения цены по порогу</div>

    <label v-if="row.HasEnable" class="strategy-row">
      <Checkbox
        :model-value="row.Enabled"
        :binary="true"
        size="small"
        @update:model-value="(v) => emit('toggle-enable', !!v)"
      />
      <span>Детектор включён</span>
    </label>

    <label v-if="row.HasNotifications" class="strategy-row">
      <Checkbox
        :model-value="row.NotificationsEnabled"
        :binary="true"
        size="small"
        @update:model-value="(v) => emit('toggle-notifications', !!v)"
      />
      <span>Уведомления в Telegram</span>
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

.strategy-panel-hint {
  font-size: 0.8rem;
  color: var(--p-text-muted-color);
}

.strategy-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
}
</style>
