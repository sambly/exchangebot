<script lang="ts" setup>
import MainContainer from './components/MainContainer.vue'
</script>

<template>
  <div class="app-layout">
    <div class="sidebar"></div>
    <div class="main-container-wrapper">
      <MainContainer />
    </div>
  </div>
</template>

<style>
html, body, #app {
  margin: 0;
  padding: 0;
  height: 100vh;
  overflow: hidden;
}
</style>

<style scoped>
.app-layout {
  display: flex;
  height: 100vh;
  overflow: hidden;
}

.sidebar {
  flex-shrink: 0;
  width: 48px;
  border-right: 1px solid var(--border-color, #e5e7eb);
  background: var(--surface-0, #ffffff);
}

.main-container-wrapper {
  flex: 1;
  min-width: 0;
  overflow: hidden;
}
</style>