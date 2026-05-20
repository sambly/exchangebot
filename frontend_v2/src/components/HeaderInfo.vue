<script setup lang="ts">
</script>

<template>
  <div class="header-info">
    <div class="header-info-left">
      <div class="stats-row">
        <div class="stat-item">
          <span class="stat-label">24часа:</span>
          <span class="stat-value" id="ch24-top"></span>
        </div>
        <div class="stat-item">
          <span class="stat-label">Объем:</span>
          <span class="stat-value" id="volume-top"></span>
        </div>
      </div>
      <div class="search-row">
        <input
          class="search-input"
          list="pairslistOptions"
          id="pairs"
          placeholder="Pair to search..."
        />
        <datalist id="pairslistOptions"></datalist>
      </div>
    </div>
    <div class="header-info-right" id="filter-panel"></div>
  </div>
</template>

<style scoped>
.header-info {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 100%;
  padding: 0 1rem;
  gap: 1rem;
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
  color: var(--text-color-secondary, #6b7280);
  white-space: nowrap;
}

.stat-value {
  font-weight: 700;
  color: var(--primary-color, #3b82f6);
}

.search-row {
  display: flex;
  align-items: center;
}

.search-input {
  padding: 0.3rem 0.6rem;
  border: 1px solid var(--border-color, #d1d5db);
  border-radius: 4px;
  font-size: 0.85rem;
  width: 200px;
  outline: none;
}

.search-input:focus {
  border-color: var(--primary-color, #3b82f6);
}

.header-info-right {
  display: flex;
  align-items: center;
}
</style>