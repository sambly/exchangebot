<script setup lang="ts">
import ChangePricePanel from './ChangePricePanel.vue'
import HeaderInfo from './HeaderInfo.vue'

// ========== НАСТРОЙКА РАЗМЕРОВ КОМПОНЕНТОВ ==========
const PRICE_TABLE_WIDTH = '30%'
// =================================================

const handleSelectPair = (pair: string) => {
  localStorage.setItem('currentPair', pair)
}
</script>

<template>
  <div class="main-container">
    <div class="top-block">
      <HeaderInfo />
    </div>
    <div class="middle-block">
      <div class="change-price-panel">
        <ChangePricePanel @select-pair="handleSelectPair" />
      </div>
      <div class="right-panel">
        <slot />
      </div>
    </div>
    <div class="bottom-block"></div>
  </div>
</template>

<style scoped>
.main-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

.top-block {
  flex-shrink: 0;
  min-height: 48px;
  border-bottom: 1px solid var(--border-color, #e5e7eb);
}

.middle-block {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.bottom-block {
  flex-shrink: 0;
  height: 48px;
  border-top: 1px solid var(--border-color, #e5e7eb);
}

.change-price-panel {
  flex: 0 0 v-bind(PRICE_TABLE_WIDTH);
  max-width: v-bind(PRICE_TABLE_WIDTH);
  height: 100%;
  overflow: hidden;
  border-right: 1px solid var(--border-color, #e5e7eb);
}

.right-panel {
  flex: 1;
  overflow: auto;
}
</style>