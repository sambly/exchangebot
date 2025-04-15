<template>
      <!-- Таблица -->
      <table id="table-trade-history" class="table table-hover table-borderless table-light">
        <thead>
          <tr>
            <th>Тип</th>
            <th>Пара</th>
            <th>Цена</th>
            <th>Дата</th>           
            <th>Профит</th>
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
          </tr>
        </tbody>
      </table>
  </template>


<script>
import { reactive, computed, watch, ref, onMounted, onUnmounted } from 'vue';
import emitter from '@/js/eventBus';
import {
  change_pair,
  show_chart_orders,
  chart_frome_orders_update,
} from '@/js/main.js';

export default {
  props: ['orders'],

  setup(props) {
    const orderMap = reactive(new Map());

    // Обновляем Map при изменении props.orders
    function fillOrderMap(orders) {
      orderMap.clear();

      const source = Array.isArray(orders)
        ? orders
        : Object.values(orders || {}).flat();

      for (const order of source) {
        orderMap.set(order.ID, order);
      }
    }

    // Следим за props.orders и сразу вызываем при старте
    watch(
      () => props.orders,
      (newOrders) => {
        fillOrderMap(newOrders);
      },
      { immediate: true }
    );

    // computed для списка ордеров (приведен к массиву)
    const orderList = computed(() =>
      Array.from(orderMap.values()).sort((a, b) => new Date(b.TimeCreated) - new Date(a.TimeCreated))
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
