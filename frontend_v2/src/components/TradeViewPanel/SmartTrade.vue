<template>
  <div class="smart-trade">

    <!-- Header: пара + таймфреймы -->
    <div class="trade-header">
      <span class="current-pair">{{ currentPair }}</span>
      <div class="btn-group">
        <Button
          v-for="tf in timeframes"
          :key="tf"
          :label="tf"
          severity="secondary"
          size="small"
          :outlined="activeFrame !== tf"
          @click="activeFrame = tf"
        />
      </div>
    </div>

    <Divider />

    <!-- Strategy -->
    <div class="field">
      <label class="field-label">Стратегия</label>
      <Select
        v-model="selectedStrategy"
        :options="strategyOptions"
        option-label="label"
        option-value="value"
        placeholder="Выберите стратегию"
        fluid
        size="small"
      />
    </div>

    <!-- Comment -->
    <div class="field">
      <label class="field-label">Комментарий</label>
      <Textarea
        v-model="comment"
        :rows="3"
        placeholder="Введите комментарий..."
        fluid
        style="resize: none; font-size: 0.875rem;"
      />
    </div>

    <!-- Deal Buttons -->
    <div class="deal-buttons">
      <Button
        label="Покупка"
        severity="success"
        class="deal-button"
        :loading="loadingBuy"
        :disabled="loadingSell"
        @click="openDeal('BUY')"
      />
      <Button
        label="Продажа"
        severity="danger"
        class="deal-button"
        :loading="loadingSell"
        :disabled="loadingBuy"
        @click="openDeal('SELL')"
      />
    </div>

  </div>
</template>

<script setup lang="ts">
import { ref, inject, onMounted, type Ref } from 'vue'
import Button from 'primevue/button'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import Divider from 'primevue/divider'
import { useOrdersStore, type Order } from '../../stores/orders.ts'

const currentPair = inject<Ref<string>>('currentPair')!

const timeframes = ['1m', '15m', '1h', '4h', '1d']
const activeFrame = ref('15m')

interface StrategyOption {
  label: string
  value: string
}

const strategyOptions = ref<StrategyOption[]>([])
const selectedStrategy = ref<string | null>(null)

const loadStrategies = async () => {
  try {
    const res = await fetch('/trade/api/getStrategies', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    if (!res.ok) return
    const data = await res.json()
    const raw: Record<string, { description: string }> = data.OptionStrategy || {}
    strategyOptions.value = Object.entries(raw).map(([key, val]) => ({
      label: key,
      value: key,
      description: val.description
    }))
    if (strategyOptions.value.length) {
      selectedStrategy.value = strategyOptions.value[0].value
    }
  } catch (err) {
    console.error('Failed to load strategies:', err)
  }
}

const comment = ref('')
const ordersStore = useOrdersStore()
const loadingBuy = ref(false)
const loadingSell = ref(false)

const openDeal = async (sideType: 'BUY' | 'SELL') => {
  if (!selectedStrategy.value) return
  const loading = sideType === 'BUY' ? loadingBuy : loadingSell
  loading.value = true
  try {
    const res = await fetch('/trade/api/openDeal', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        pair: currentPair.value,
        sideType,
        frame: activeFrame.value,
        strategy: selectedStrategy.value,
        comment: comment.value
      })
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const data = await res.json()

    // Сервер возвращает массив или объект — добавляем/обновляем точечно
    const orders: Order[] = Array.isArray(data) ? data : Object.values(data).flat() as Order[]
    for (const order of orders) {
      ordersStore.addOrder(order)
    }
  } catch (err) {
    console.error('openDeal error:', err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadStrategies()
})
</script>

<style scoped>
.smart-trade {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1rem;
  width: 100%;
  max-width: 480px;   /* не растягивается на весь экран */
  box-sizing: border-box;
}

.trade-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.current-pair {
  font-size: 1rem;
  font-weight: 600;
  letter-spacing: 0.03em;
  color: var(--p-text-color);
}

.btn-group {
  display: flex;
  gap: 0.25rem;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.field-label {
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--p-text-muted-color);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.deal-buttons {
  display: flex;
  gap: 0.75rem;
  padding-top: 0.25rem;
}

.deal-button {
  flex: 1;
}
</style>