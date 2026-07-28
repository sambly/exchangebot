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

/* Sidebar на мобильных превращается из вертикальной колонки в горизонтальную
   полосу сверху (см. Sidebar.vue) - переключаем сам app-layout в колонку и
   фиксируем высоту, иначе получим 100vh сайдбара + 100vh контента = скролл
   всей страницы вместо скролла внутри MainView. */
@media (max-width: 768px) {
  .app-layout {
    flex-direction: column;
    height: 100vh;
    min-height: 0;
    overflow: hidden;
  }

  .main-container-wrapper {
    min-height: 0;
    overflow: hidden;
  }
}
</style>