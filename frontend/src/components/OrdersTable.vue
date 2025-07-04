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
          <th>Стратегия</th>
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
          <td>{{ order.StrategyBuy }}</td>
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
import { computed, ref, onMounted, onUnmounted } from 'vue';
import emitter from '@/js/eventBus';
import { useOrdersStore } from '@/stores/orders';
import {
  change_pair,
  show_chart_orders,
  chart_frome_orders_update,
} from '@/js/main.js';

export default {

  setup() {
    const ordersStore = useOrdersStore();
    const pnl = ref(0);

    // Получаем список ордеров из хранилища
    const orderList = computed(() => 
      [...ordersStore.active].sort((a, b) => new Date(b.TimeCreated) - new Date(a.TimeCreated))
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
      ordersStore.updateOrder(order);
    }

    // Добавление нового ордера
    function handleOrderAdd(order) {
      ordersStore.addOrder(order);
    }

    // Удаление ордера
    function handleOrderRemove(order) {
      ordersStore.removeOrder(order.ID);
    }


    function handlePnlUpdate(value) {
      pnl.value = value;
    }

    async function handleClose(orderId) {
      try {
        const response = await $.ajax({
          url: 'closeDeal',
          type: 'POST',
          method: 'POST',
          cache: false,
          contentType: 'text/html; charset=utf-8',
          processData: false,
          data: orderId
        });
        
        ordersStore.setActive(response.OrdersActive || []);
        ordersStore.setHistory(response.OrdersHistory || []);
      } catch (error) {
        console.error('Ошибка при закрытии позиции', error);
      }
    }

    async function handleCloseAll() {
      const res = confirm('Вы подтверждаете закрытие всех сделок?');
      if (!res) return;
      
      try {
        const response = await $.ajax({
          url: 'closeAllDeal',
          type: 'POST',
          method: 'POST',
          cache: false,
          processData: false
        });
        
        ordersStore.setActive(response.OrdersActive || []);
        ordersStore.setHistory(response.OrdersHistory || []);
      } catch (error) {
        console.error('Ошибка при закрытии всех позиций', error);
      }
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
  width: 10%;
}

.table-trade-active th:nth-child(6),
.table-trade-active td:nth-child(6) {
  width: 10%;
}

.table-trade-active th:nth-child(7),
.table-trade-active td:nth-child(7) {
  width: 10%;
}


.btn-close-deal-all {
  border: none; /* Убирает границу кнопки */
  background: none; /* Убирает фон кнопки */
  padding: 0; /* Убирает отступы внутри кнопки */
  cursor: pointer; /* Делает курсор указателем при наведении */
}

</style>