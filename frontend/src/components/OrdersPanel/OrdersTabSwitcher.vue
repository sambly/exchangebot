<script setup lang="ts">
import Button from 'primevue/button'

defineProps<{
  modelValue: 'active' | 'history'
}>()

const emit = defineEmits<{
  'update:modelValue': ['active' | 'history']
}>()
</script>

<template>
  <div class="orders-tab-switcher">
    <Button
      label="Активные"
      size="small"
      severity="secondary"
      :outlined="modelValue !== 'active'"
      @click="emit('update:modelValue', 'active')"
    />
    <Button
      label="История"
      size="small"
      severity="secondary"
      :outlined="modelValue !== 'history'"
      @click="emit('update:modelValue', 'history')"
    />
  </div>
</template>

<style scoped>
.orders-tab-switcher {
  display: flex;
  gap: 0.25rem;
  flex-shrink: 0;
}
</style>