<script setup lang="ts">
import { ref, provide } from 'vue'
import Toast from 'primevue/toast'

import MainContainer from './components/MainView.vue'
import Sidebar from './components/Layout/Sidebar.vue'

const darkMode = ref(false)
provide('darkMode', darkMode)

function toggleDarkMode() {
  darkMode.value = !darkMode.value
  document.documentElement.classList.toggle('app-dark', darkMode.value)
}
</script>

<template>
  <div class="app-layout">
    <Toast position="top-right" />
    <Sidebar :dark-mode="darkMode" @toggle-dark-mode="toggleDarkMode" />
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