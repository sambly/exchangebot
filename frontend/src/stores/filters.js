import { defineStore } from 'pinia';

const STORAGE_KEY = 'filterPairs';
const FILTER_VALUES_KEY = 'filterValues';

function loadFromStorage() {
  try {
    const saved = localStorage.getItem(FILTER_VALUES_KEY);
    return saved ? JSON.parse(saved) : null;
  } catch {
    return null;
  }
}

function getDefaultState() {
  return {
    volume: { min: null, max: null },
    ch1d: { min: null, max: null },
    // Периоды изменения цены
    periods: {
      '1m': { min: null, max: null },
      '3m': { min: null, max: null },
      '15m': { min: null, max: null },
      '1h': { min: null, max: null },
      '4h': { min: null, max: null },
      '1d': { min: null, max: null },
    },
    filteredPairs: [],
  };
}

export const useFiltersStore = defineStore('filters', {
  state: () => {
    const saved = loadFromStorage();
    if (saved) {
      return {
        volume: saved.volume || { min: null, max: null },
        ch1d: saved.ch1d || { min: null, max: null },
        periods: {
          '1m': saved.periods?.['1m'] || { min: null, max: null },
          '3m': saved.periods?.['3m'] || { min: null, max: null },
          '15m': saved.periods?.['15m'] || { min: null, max: null },
          '1h': saved.periods?.['1h'] || { min: null, max: null },
          '4h': saved.periods?.['4h'] || { min: null, max: null },
          '1d': saved.periods?.['1d'] || { min: null, max: null },
        },
        filteredPairs: [],
      };
    }
    return getDefaultState();
  },

  actions: {
    applyFilters() {
      this.saveToLocalStorage();
    },

    resetFilters() {
      const defaults = getDefaultState();
      this.volume = { ...defaults.volume };
      this.ch1d = { ...defaults.ch1d };
      this.periods = {
        '1m': { ...defaults.periods['1m'] },
        '3m': { ...defaults.periods['3m'] },
        '15m': { ...defaults.periods['15m'] },
        '1h': { ...defaults.periods['1h'] },
        '4h': { ...defaults.periods['4h'] },
        '1d': { ...defaults.periods['1d'] },
      };
      this.filteredPairs = [];
      this.saveToLocalStorage();
    },

    saveToLocalStorage() {
      localStorage.setItem(FILTER_VALUES_KEY, JSON.stringify({
        volume: this.volume,
        ch1d: this.ch1d,
        periods: this.periods,
      }));
    },

    /**
     * Фильтрует пары на основе текущих значений фильтров
     * @param {Object} marketsStat - MarketsStat[pair].Volume
     * @param {Object} changePrices - changePrices[pair][period].ChangePercent
     * @returns {string[]} отфильтрованный список пар
     */
    filterPairs(marketsStat, changePrices) {
      const heads = ['1m', '3m', '15m', '1h', '4h', '1d'];
      const pairs = [];

      for (const pair in changePrices) {
        const volume = marketsStat[pair]?.Volume;
        let pass = true;

        // Фильтр по объему
        const vol = this.volume;
        if (vol.min !== null && vol.min !== '' && volume <= Number(vol.min)) pass = false;
        if (vol.max !== null && vol.max !== '' && volume >= Number(vol.max)) pass = false;

        // Фильтр по периодам изменения цены
        if (pass) {
          for (const head of heads) {
            const periodFilter = this.periods[head];
            if (!periodFilter) continue;
            const chValue = changePrices[pair]?.[head]?.ChangePercent;
            if (chValue === undefined) continue;

            if (periodFilter.min !== null && periodFilter.min !== '' && chValue <= Number(periodFilter.min)) {
              pass = false;
              break;
            }
            if (periodFilter.max !== null && periodFilter.max !== '' && chValue >= Number(periodFilter.max)) {
              pass = false;
              break;
            }
          }
        }

        if (pass) {
          pairs.push(pair);
        }
      }

      // Сохраняем в localStorage для совместимости со старым кодом
      localStorage.setItem(STORAGE_KEY, JSON.stringify(pairs));
      this.filteredPairs = pairs;

      return pairs;
    },
  },
});