<template>
  <div>
    <!-- Верхняя панель -->
    <div class="d-flex mt-3">
      <div class="d-flex gap-1 ms-3">
        Сделок
        <div class="text-primary" style="font-weight: bold;">{{ orderList.length }}</div>
      </div>
      <div class="d-flex gap-1 ms-3">
        PNL
        <div :style="{ color: colorProfit(pnl), fontWeight: 'bold' }">{{ orderList.length === 0 ? '0' : formatProfit(pnl) }}</div>
      </div>
      <div class="d-flex ms-auto" style="margin-right: 5rem !important;">
        <button class="btn-close-deal-all" @click="handleCloseAll">
          <img src="/src/svg/btn_close_all.svg" alt="clDealAll" title="Закрыть все сделки" />
        </button>
      </div>
    </div>

    <!-- Таблица -->
    <table class="table table-hover table-borderless table-light table-trade-active">
      <thead>
        <tr>
          <th>Тип</th>
          <th>Пара</th>
          <th>Цена</th>
          <th>Профит</th>
          <th>Дата</th>
          <th>Закрыть</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="order in orderList"
          :key="order.ID"
          class="order-active"
          @click="handleRowClick(order.Pair)"
        >
          <td :style="{ color: colorSide(order.Side) }">{{ order.Side }}</td>
          <td>{{ order.Pair }}</td>
          <td>{{ order.PriceCreated || '-' }}</td>
          <td :style="{ color: colorProfit(order.Profit) }">
            {{ formatProfit(order.Profit) }}
          </td>
          <td>{{ formatTime(order.TimeCreated) }}</td>
          <td>
            <button class="btn-close" type="button" @click.stop="handleClose(order.ID)"></button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
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
    const pnl = ref(0);

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

    onMounted(() => {
      emitter.on('order:update', handleOrderUpdate);
      emitter.on('order:add', handleOrderAdd);
      emitter.on('order:remove', handleOrderRemove);
      emitter.on('pnl:update', handlePnlUpdate);
    });

    onUnmounted(() => {
      emitter.off('order:update', handleOrderUpdate);
      emitter.off('order:add', handleOrderAdd);
      emitter.off('order:remove', handleOrderRemove);
      emitter.off('pnl:update', handlePnlUpdate);
    });

    // Обновление ордера
    function handleOrderUpdate(order) {
      if (orderMap.has(order.ID)) {
        const old = orderMap.get(order.ID);
        orderMap.set(order.ID, { ...old, ...order });
      }
    }

    // Добавление нового ордера
    function handleOrderAdd(order) {
      if (!orderMap.has(order.ID)) {
        orderMap.set(order.ID, order);
      }
    }

    // Удаление ордера по ID
    function handleOrderRemove(order) {
      if (orderMap.has(order.ID)) {
        orderMap.delete(order.ID);
      }
    }


    function handlePnlUpdate(value) {
      pnl.value = value;
    }

    function handleClose(orderId) {
      $.ajax({
        url: 'closeDeal',
        type: 'POST',
        method: 'POST',
        cache: false,
        contentType: 'text/html; charset=utf-8',
        processData: false,
        data: orderId,
        success: (orders) => {
          fillOrderMap(orders.OrdersActive);
          // TODO здесь эта функция кажись не вызывается 
          window.forming_orders_history?.(orders.OrdersHistory);
        },
        error: (response) => {
          console.error('Ошибка при закрытии позиции', response);
        },
      });
    }

    function handleCloseAll() {

      let res =confirm('Вы подтверждаете закрытие всех сделок?');
      if (res!=true) return;
      $.ajax({
          url: 'closeAllDeal',
          type: 'POST',
          method: 'POST',
          cache: false,
          processData: false,
          success: function (orders) {
            fillOrderMap(orders.OrdersActive);
            window.forming_orders_history?.(orders.OrdersHistory);
          },
          error: function (response) {
            // TODO здесь эта функция кажись не вызывается 
            console.error('Ошибка при закрытии всех позиций', response);
          },
      });
    }

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
      pnl,
      handleClose,
      handleCloseAll,
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

.table-trade-active {
  table-layout: fixed;
  width: 100%;
}

.table-trade-active tbody{
  max-height: 300px;
}  

.table-trade-active th:nth-child(1),
.table-trade-active td:nth-child(1) {
  width: 10%;
}

.table-trade-active th:nth-child(2),
.table-trade-active td:nth-child(2) {
  width: 20%;
}

.table-trade-active th:nth-child(3),
.table-trade-active td:nth-child(3) {
  width: 20%;
}

.table-trade-active th:nth-child(4),
.table-trade-active td:nth-child(4) {
  width: 20%;
}

.table-trade-active th:nth-child(5),
.table-trade-active td:nth-child(5) {
  width: 20%;
}

.table-trade-active th:nth-child(6),
.table-trade-active td:nth-child(6) {
  width: 10%;
}

.btn-close-deal-all {
  border: none; /* Убирает границу кнопки */
  background: none; /* Убирает фон кнопки */
  padding: 0; /* Убирает отступы внутри кнопки */
  cursor: pointer; /* Делает курсор указателем при наведении */
}

</style>