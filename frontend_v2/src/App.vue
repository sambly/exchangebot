<script setup lang="ts">
import { watch } from 'vue'
import Toast from 'primevue/toast'

import MainContainer from './components/MainView.vue'
import Sidebar from './components/Layout/Sidebar.vue'
import { useUIStore } from './stores/ui'

const ui = useUIStore()

watch(
  () => ui.darkMode,
  (enabled) => document.documentElement.classList.toggle('app-dark', enabled),
  { immediate: true },
)
</script>

<template>
  <div class="app-layout">
    <Toast position="top-right" />
    <Sidebar />
    <main class="main-container-wrapper">
      <MainContainer />
    </main>
  </div>
</template>

<style scoped>
.app-layout {
  display: flex;
  width: 100%;
  min-height: 100vh;
}

.main-container-wrapper {
  flex: 1;
  min-width: 0;
  overflow: visible;
}
</style>