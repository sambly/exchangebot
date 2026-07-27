<!-- DataPanel.vue -->
<script setup lang="ts">
import DataChangePrice from '../DataPanel/DataChangePrice.vue'
import DataVolumeDelta from '../DataPanel/DataVolumeDelta.vue'
import DataImbalance from '../DataPanel/DataImbalance.vue'
import DataStrength from '../DataPanel/DataStrength.vue'
import { useUIStore } from '../../stores/ui'

const ui = useUIStore()
</script>

<template>

    <div v-show="ui.activeDataPanel === 'price'" class="panel-content">
      <DataChangePrice />
    </div>
    <div v-show="ui.activeDataPanel === 'volume'" class="panel-content">
      <DataVolumeDelta />
    </div>
    <div v-show="ui.activeDataPanel === 'imbalance'" class="panel-content">
      <DataImbalance />
    </div>
    <div v-show="ui.activeDataPanel === 'strength'" class="panel-content">
      <DataStrength />
    </div>

</template>

<style scoped>

.panel-content {
  height: 100%;
  width: 100%;
  overflow: hidden;
}
</style>