<template>
  <div>
    <!-- Верхняя панель с фильтрами -->
    <div class="d-flex align-items-center mt-3 mb-2 gap-3">
      <div class="d-flex gap-1 ms-3">
        Сделок
        <div class="text-primary" style="font-weight: bold;">{{ totalOrders.toLocaleString('ru') }}</div>
      </div>
      <div class="d-flex gap-1 ms-3">
        PNL
        <div :style="{ color: colorProfit(totalProfit), fontWeight: 'bold' }"> {{ formatProfit(totalProfit) }}</div>
      </div>
      <!-- Новый фильтр по паре -->
      <label class="ms-3">Пара:</label>
      <select v-model="selectedPair" class="form-select w-auto">
        <option value="">Все</option>
        <option v-for="p in pairOptions" :key="p" :value="p">{{ p }}</option>
      </select>
      <label class="ms-3">Интервал:</label>
      <select v-model="selectedInterval" class="form-select w-auto">
        <option value="all">За всё время</option>
        <option value="1d">За 1 день</option>
        <option value="7d">За 7 дней</option>
        <option value="30d">За 30 дней</option>
      </select>
      <label class="ms-3">Стратегия покупки:</label>
      <select v-model="selectedStrategyBuy" class="form-select w-auto">
        <option value="">Все</option>
        <option v-for="s in strategyBuyOptions" :key="s" :value="s">{{ s }}</option>
      </select>
      <label class="ms-3">Стратегия продажи:</label>
      <select v-model="selectedStrategySell" class="form-select w-auto">
        <option value="">Все</option>
        <option v-for="s in strategySellOptions" :key="s" :value="s">{{ s }}</option>
      </select>
      <!-- Кнопка сброса фильтров -->
      <button class="btn btn-outline-secondary ms-3" @click="resetFilters">Сбросить фильтры</button>
    </div>
 
    <!-- Таблица -->
    <table class="table table-hover table-borderless table-light table-trade-history">
      <thead>
        <tr>
          <th>Тип</th>
          <th>Пара</th>
          <th>Цена</th>
          <th>Дата</th>           
          <th>Профит</th>
          <th>Стратегия</th>
        </tr>
      </thead>
      <tbody class="pt-2">
        <tr
          v-for="order in filteredOrders"
          :key="order.ID"
          class="order-history d-flex align-items-center"
          @click="handleRowClick(order.Pair)"
        >
          <td :style="{ color: colorSide(order.Side) }">{{ order.Side }}</td>
          <td>{{ order.Pair }}</td>
          <td>
            <div>{{ order.PriceCreated || '-' }}</div>
            <div>{{ order.Price || '-' }}</div>
          </td>
          <td>
            <div>{{ formatTime(order.TimeCreated) }}</div>
            <div>{{formatTime(order.Time) }}</div>
          </td>
          <td :style="{ color: colorProfit(order.Profit) }">
            {{ formatProfit(order.Profit) }}
          </td>
          <td>
            <div>{{ order.StrategyBuy }}</div>
            <div>{{order.StrategySell }}</div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>


<script>
import { computed, ref } from 'vue';
import { useOrdersStore } from '@/stores/orders';
import {
  change_pair,
  show_chart_orders,
  chart_frome_orders_update,
} from '@/js/main.js';

export default {
  setup() {
    const store = useOrdersStore();
    // Получаем отсортированный список из хранилища
    const orderList = computed(() => 
      [...store.history].sort((a, b) => 
        new Date(b.Time) - new Date(a.Time))
    );

    // Фильтры
    const selectedInterval = ref('all');
    const selectedStrategyBuy = ref('');
    const selectedStrategySell = ref('');
    const selectedPair = ref('');

    // Уникальные значения стратегий для выпадающих списков
    const strategyBuyOptions = computed(() => {
      const set = new Set(orderList.value.map(o => o.StrategyBuy).filter(Boolean));
      return Array.from(set);
    });
    const strategySellOptions = computed(() => {
      const set = new Set(orderList.value.map(o => o.StrategySell).filter(Boolean));
      return Array.from(set);
    });
    const pairOptions = computed(() => {
      const set = new Set(orderList.value.map(o => o.Pair).filter(Boolean));
      return Array.from(set);
    });

    // Фильтрованные ордера по выбранным фильтрам
    const filteredOrders = computed(() => {
      let filtered = orderList.value;
      // Фильтр по паре
      if (selectedPair.value) {
        filtered = filtered.filter(order => order.Pair === selectedPair.value);
      }
      // Фильтр по интервалу
      if (selectedInterval.value !== 'all') {
        const now = Date.now();
        let ms = 0;
        if (selectedInterval.value === '1d') ms = 24 * 60 * 60 * 1000;
        if (selectedInterval.value === '7d') ms = 7 * 24 * 60 * 60 * 1000;
        if (selectedInterval.value === '30d') ms = 30 * 24 * 60 * 60 * 1000;
        filtered = filtered.filter(order => {
          const t = new Date(order.Time).getTime();
          return !isNaN(t) && (now - t) <= ms;
        });
      }
      // Фильтр по стратегии покупки
      if (selectedStrategyBuy.value) {
        filtered = filtered.filter(order => order.StrategyBuy === selectedStrategyBuy.value);
      }
      // Фильтр по стратегии продажи
      if (selectedStrategySell.value) {
        filtered = filtered.filter(order => order.StrategySell === selectedStrategySell.value);
      }
      return filtered;
    });

    // Общее количество сделок (по фильтру)
    const totalOrders = computed(() => filteredOrders.value.length);

    // Общий профит по всем сделкам (по фильтру)
    const totalProfit = computed(() =>
      filteredOrders.value.reduce((sum, order) => {
        return typeof order.Profit === 'number' ? sum + order.Profit : sum;
      }, 0)
    );

    function handleRowClick(pair) {
      change_pair?.(pair);
      show_chart_orders?.();
      chart_frome_orders_update?.(filteredOrders.value).catch(console.error);
    }

    function colorProfit(profit) {
      if (typeof profit !== 'number') return '#000';
      return profit > 0 ? 'green' : profit < 0 ? 'red' : '#aaa';
    }

    function formatProfit(profit) {
      return typeof profit === 'number'
        ? profit.toLocaleString('ru', {
            maximumFractionDigits: 2,
            minimumFractionDigits: 2,
            notation: 'compact',
          })
        : '-';
    }

    function formatTime(timestamp) {
      if (!timestamp) return '-';
      const date = new Date(timestamp);
      return isNaN(date.getTime()) ? '-' : date.toLocaleString('en-GB');
    }

    function colorSide(side) {
      return side === 'BUY' ? 'green' : side === 'SELL' ? 'red' : '#000';
    }

    // Сброс фильтров
    function resetFilters() {
      selectedPair.value = '';
      selectedInterval.value = 'all';
      selectedStrategyBuy.value = '';
      selectedStrategySell.value = '';
    }

    return {
      filteredOrders,
      handleRowClick,
      formatProfit,
      formatTime,
      colorProfit,
      colorSide,
      totalOrders,
      totalProfit,
      selectedInterval,
      selectedStrategyBuy,
      selectedStrategySell,
      strategyBuyOptions,
      strategySellOptions,
      selectedPair,
      pairOptions,
      resetFilters,
    };
  },
};

</script>


<style scoped>

.table-trade-history {
  table-layout: fixed;
  width: 100%;
}

.table-trade-history tbody{
  max-height: 300px;
}  


.table-trade-history th:nth-child(1),
.table-trade-history td:nth-child(1) {
  width: 20%;
}

.table-trade-history th:nth-child(2),
.table-trade-history td:nth-child(2) {
  width: 20%;
}

.table-trade-history th:nth-child(3),
.table-trade-history td:nth-child(3) {
  width: 20%;
}

.table-trade-history th:nth-child(4),
.table-trade-history td:nth-child(4) {
  width: 20%;
}

.table-trade-history th:nth-child(5),
.table-trade-history td:nth-child(5) {
  width: 10%;
}

.table-trade-history th:nth-child(6),
.table-trade-history td:nth-child(6) {
  width: 10%;
}

</style>