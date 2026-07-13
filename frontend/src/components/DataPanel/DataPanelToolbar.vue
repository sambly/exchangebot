<script setup lang="ts">
import Button from 'primevue/button'
import { useUIStore } from '../../stores/ui'

const ui = useUIStore()
</script>

<template>
  <div class="filter-buttons">
    <Button
      icon="pi pi-heart"
      size="small"
      severity="secondary"
      :outlined="ui.filterMode !== 'favorites'"
      @click="ui.filterMode = ui.filterMode === 'favorites' ? 'all' : 'favorites'"
    />
    <div class="divider" />
    <Button
      label="Цена"
      severity="secondary"
      size="small"
      :outlined="ui.activeDataPanel !== 'price'"
      @click="ui.activeDataPanel = 'price'"
    />
    <Button
      label="Объем"
      severity="secondary"
      size="small"
      :outlined="ui.activeDataPanel !== 'volume'"
      @click="ui.activeDataPanel = 'volume'"
    />
  </div>
</template>

<style scoped>
.filter-buttons {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.divider {
  width: 1px;
  height: 1.25rem;
  background: var(--p-content-border-color);
  flex-shrink: 0;
}
</style>
