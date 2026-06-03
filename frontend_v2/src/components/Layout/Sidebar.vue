<script setup lang="ts">
import Button from 'primevue/button'
import GrafanaIcon from '../../assets/grafana.svg'
import JaegerIcon from '../../assets/jaeger.svg'

defineProps<{
  darkMode: boolean
}>()

const emit = defineEmits<{
  (e: 'toggle-dark-mode'): void
}>()
</script>

<template>
  <div class="sidebar">
    <div class="sidebar-buttons">
      
      <a href="/" class="sidebar-link">
        <Button icon="pi pi-home" text rounded severity="secondary" />
      </a>

      <a href="grafana" target="_blank" class="sidebar-link">
        <img :src="GrafanaIcon" alt="Grafana" class="sidebar-icon" />
      </a>

      <a href="jaeger" target="_blank" class="sidebar-link">
        <img :src="JaegerIcon" alt="Jaeger" class="sidebar-icon" />
      </a>

      <Button
        :icon="darkMode ? 'pi pi-moon' : 'pi pi-sun'"
        text
        rounded
        severity="secondary"
        @click="emit('toggle-dark-mode')"
      />
    </div>
  </div>
</template>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 48px;
  padding-top: 0.5rem;
  flex-shrink: 0;
}

.sidebar-buttons {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}

.sidebar-link {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 50%;
  transition: background 0.2s;
}

.sidebar-link:hover {
  background: var(--p-content-border-color);
}

.sidebar-icon {
  width: 1.1rem;
  height: 1.1rem;
}
</style>