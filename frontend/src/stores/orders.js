import { defineStore } from 'pinia';

export const useOrdersStore = defineStore('orders', {
  state: () => ({
    active: [],    // активные ордера
    history: []    // история ордеров
  }),
  actions: {
    setActive(orders) {
      this.active = Array.isArray(orders) ? orders : Object.values(orders || {}).flat();
    },
    setHistory(orders) {
      this.history = Array.isArray(orders) ? orders : Object.values(orders || {}).flat();
    },
    updateOrder(order) {
      const index = this.active.findIndex(o => o.ID === order.ID);
      if (index !== -1) {
        this.active[index] = { ...this.active[index], ...order };
      }
    },
    addOrder(order) {
      if (!this.active.some(o => o.ID === order.ID)) {
        this.active.push(order);
      }
    },
    removeOrder(orderId) {
      this.active = this.active.filter(order => order.ID !== orderId);
    }
  }
});