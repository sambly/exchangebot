<template>
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
            v-for="order in orderList"
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
  </template>


<script>
import { computed } from 'vue';
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
        new Date(b.TimeCreated) - new Date(a.TimeCreated))
    );


    function handleRowClick(pair) {
      change_pair?.(pair);
      show_chart_orders?.();
      chart_frome_orders_update?.(orderList.value).catch(console.error);
    }

    function colorProfit(profit) {
      if (typeof profit !== 'number') return '#000';
      return profit > 0 ? 'green' : profit < 0 ? 'red' : '#aaa';
    }

    function formatProfit(profit) {
      return typeof profit === 'number'
        ? profit.toLocaleString('ru', {
            maximumFractionDigits: 2,
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

    return {
      orderList,
      handleRowClick,
      formatProfit,
      formatTime,
      colorProfit,
      colorSide,
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