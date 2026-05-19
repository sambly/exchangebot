<template>
  <div class="filter-panel">
    <div class="row g-2 align-items-end">
      <!-- Фильтр: Объем -->
      <div class="col-auto">
        <div class="input-group input-group-sm">
          <span class="input-group-text">Объем</span>
          <input
            type="number"
            class="form-control"
            :value="store.volume.min"
            @input="store.volume.min = $event.target.value || null"
            placeholder="min"
          />
          <input
            type="number"
            class="form-control"
            :value="store.volume.max"
            @input="store.volume.max = $event.target.value || null"
            placeholder="max"
          />
        </div>
      </div>

      <!-- Фильтры по периодам изменения цены -->
      <template v-for="period in periodKeys" :key="period">
        <div class="col-auto">
          <div class="input-group input-group-sm">
            <span class="input-group-text">{{ period }}</span>
            <input
              type="number"
              class="form-control"
              :value="store.periods[period].min"
              @input="store.periods[period].min = $event.target.value || null"
              placeholder="min"
            />
            <input
              type="number"
              class="form-control"
              :value="store.periods[period].max"
              @input="store.periods[period].max = $event.target.value || null"
              placeholder="max"
            />
          </div>
        </div>
      </template>

      <!-- Кнопки -->
      <div class="col-auto">
        <div class="d-flex gap-1">
          <button type="button" class="btn btn-sm btn-secondary" @click="handleApply">Применить</button>
          <button type="button" class="btn btn-sm btn-outline-secondary" @click="handleReset">Сбросить</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { useFiltersStore } from '@/stores/filters';

export default {
  setup() {
    const store = useFiltersStore();
    const periodKeys = ['1m', '3m', '15m', '1h', '4h', '1d'];

    function handleApply() {
      store.applyFilters();
      // Вызываем глобальное событие для перерисовки таблиц
      window.dispatchEvent(new CustomEvent('filters:applied'));
    }

    function handleReset() {
      store.resetFilters();
      window.dispatchEvent(new CustomEvent('filters:applied'));
    }

    return {
      store,
      periodKeys,
      handleApply,
      handleReset,
    };
  },
};
</script>

<style scoped>
.filter-panel {
  padding: 0.5rem 0;
}

.input-group-text {
  min-width: 45px;
  justify-content: center;
}

.form-control {
  max-width: 80px;
}

.btn-sm {
  white-space: nowrap;
}
</style>