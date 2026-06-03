<!-- DataPanel.vue -->
<script setup lang="ts">
import { ref,provide } from 'vue'
import DataChangePrice from '../DataPanel/DataChangePrice.vue'
import DataVolumeDelta from '../DataPanel/DataVolumeDelta.vue'

const activeComponent = ref<'price' | 'volume' | 'other'>('price')
const filterMode = ref<'all' | 'favorites'>('all')
provide('activeComponent', activeComponent)
provide('filterMode', filterMode)

</script>

<template>

    <div v-show="activeComponent === 'price'" class="panel-content">
      <DataChangePrice />
    </div>
    <div v-show="activeComponent === 'volume'" class="panel-content">
      <DataVolumeDelta />
    </div>

</template>

<style scoped>

.panel-content {
  height: 100%;
  width: 100%;
  overflow: hidden;
}
</style>