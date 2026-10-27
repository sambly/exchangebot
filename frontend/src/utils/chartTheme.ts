// themeBackground - фон канваса графиков lightweight-charts, взятый из того
// же токена темы PrimeVue (--p-content-background), что и DataTable/Card -
// раньше здесь была зашита в код навигационно-синяя #131722 (фирменный тёмный
// цвет TradingView), не связанная с темой приложения: обе тёмные, но разных
// оттенков, отсюда видимый разнобой фона между таблицами/карточками и
// графиками.
//
// Не трогает TradingViewChart.vue (встроенный виджет TradingView, а не
// lightweight-charts) - там #131722 остаётся: это встроенный виджет со своей
// ВНУТРЕННЕЙ тёмной темой (theme: 'Dark'), которую мы не красим, а только
// подгоняем toolbar/заставку под то, что сам виджет всё равно покажет.
//
// fallback - на случай, если переменная ещё не резолвится (SSR/до монтирования
// темы) - те же значения, что у content.background в Aura-пресете (light:
// surface.0 = #ffffff, dark: surface.900 = #18181b), а не белый/чёрный "куда
// придётся".
export function themeBackground(darkMode: boolean): string {
  if (typeof document !== 'undefined') {
    const value = getComputedStyle(document.documentElement).getPropertyValue('--p-content-background').trim()
    if (value) return value
  }
  return darkMode ? '#18181b' : '#ffffff'
}
