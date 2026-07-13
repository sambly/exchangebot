<script setup lang="ts">
import { ref, computed } from 'vue'
import InputNumber from 'primevue/inputnumber'
import Button from 'primevue/button'
import Divider from 'primevue/divider'
import Card from 'primevue/card'
import Popover from 'primevue/popover'
import { storeToRefs } from 'pinia'
import { useFiltersStore } from '../../stores/filters'

const filtersStore = useFiltersStore()
const { volumeFilter, periodFilters } = storeToRefs(filtersStore)
const { periods, resetFilters, saveFilters } = filtersStore

const popover = ref<InstanceType<typeof Popover>>()

const applyFilters = () => {
  saveFilters()
  popover.value?.hide()
}

const handleReset = () => {
  resetFilters()
  popover.value?.hide()
}

const activeFiltersCount = computed(() => {
  let count = 0
  if (volumeFilter.value.min !== null) count++
  if (volumeFilter.value.max !== null) count++
  for (const p of periods) {
    if (periodFilters.value[p].min !== null) count++
    if (periodFilters.value[p].max !== null) count++
  }
  return count
})

</script>

<template>
  <div class="filter-panel">
    <Button
      label="Фильтры"
      icon="pi pi-sliders-h"
      size="small"
      outlined
      :badge="activeFiltersCount > 0 ? String(activeFiltersCount) : undefined"
      badgeSeverity="primary"      
      @click="popover?.toggle($event)"
    />
    
    <Popover 
      ref="popover"
      :dismissable="true"
      :showCloseIcon="true"
    >
      <Card class="filters-card">
        <template #content>
          <div class="filter-content">
            <!-- Volume -->
            <div class="filter-block">
              <div class="block-title">Объем</div>
              <div class="filter-row">
                <div class="filter-field">
                  <label>Min</label>
                  <InputNumber
                    v-model="volumeFilter.min"
                    placeholder="0"
                    size="small"
                  />
                </div>
                <div class="filter-field">
                  <label>Max</label>
                  <InputNumber
                    v-model="volumeFilter.max"
                    placeholder="∞"
                    size="small"
                  />
                </div>
              </div>
            </div>

            <Divider />

            <!-- Changes -->
            <div class="filter-block">
              <div class="block-title">Изменения %</div>
              <div class="periods-grid">
                <div v-for="p in periods" :key="p" class="period-filter">
                  <label>{{ p }}</label>
                  <div class="period-inputs">
                    <InputNumber
                      v-model="periodFilters[p].min"
                      placeholder="min"
                      size="small"
                    />
                    <span>-</span>
                    <InputNumber
                      v-model="periodFilters[p].max"
                      placeholder="max"
                      size="small"
                    />
                  </div>
                </div>
              </div>
            </div>

            <Divider />

            <!-- Actions -->
            <div class="filter-actions">
              <Button
                label="Сброс"
                icon="pi pi-refresh"
                severity="secondary"
                outlined
                size="small"
                @click="handleReset"
              />
              <Button
                label="Применить"
                icon="pi pi-check"
                size="small"
                @click="applyFilters"
              />
            </div>
          </div>
        </template>
      </Card>
    </Popover>
  </div>
</template>

<style scoped>
.filter-panel {
  display: inline-block;
}

.filters-card {
  border: none;
  box-shadow: none;
  padding: 0;
  min-width: 380px;
  max-width: 600px;
}

.filter-content {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.filter-block {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.block-title {
  font-size: 0.9rem;
  font-weight: 600;
}

.filter-row {
  display: flex;
  gap: 1rem;
}

.filter-field {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 120px;
}

.filter-field label {
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--text-color-secondary);
}

.periods-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(140px, 1fr));
  gap: 1rem;
}

.period-filter {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 0;
}

.period-filter label {
  font-size: 0.75rem;
  font-weight: 600;
  text-align: center;
}

.period-inputs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.period-inputs span {
  color: var(--text-color-secondary);
  font-size: 0.875rem;
  flex-shrink: 0;
}

.period-inputs :deep(.p-inputnumber) {
  flex: 1;
  min-width: 0;
}

.period-inputs :deep(.p-inputnumber input) {
  width: 100%;
  min-width: 0;
}

.filter-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
}

/* Стили для Popover */
:deep(.p-popover) {
  margin-top: 0.5rem;
}

:deep(.p-popover-content) {
  padding: 0;
}

:deep(.p-card) {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

/* Адаптация для маленьких экранов */
@media (max-width: 640px) {
  .periods-grid {
    grid-template-columns: 1fr;
    gap: 0.75rem;
  }
  
  .filters-card {
    min-width: 280px;
    max-width: 90vw;
  }
  
  .filter-row {
    flex-direction: column;
    gap: 0.5rem;
  }
  
  .filter-field {
    min-width: auto;
  }
}
</style>