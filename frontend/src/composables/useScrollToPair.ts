import { nextTick, watch, type Ref } from 'vue'
import { useUIStore, type ActiveDataPanel } from '../stores/ui'

export const ROW_HEIGHT = 41

/**
 * Держит строку выбранной пары в поле зрения виртуального скроллера.
 * orderedPairs — полные имена пар в том порядке, в котором они отрисованы.
 */
export function useScrollToPair(
  containerRef: Ref<HTMLElement | null>,
  orderedPairs: Ref<string[]>,
  panel: ActiveDataPanel,
) {
  const ui = useUIStore()

  async function scrollToPair() {
    await nextTick()

    const idx = orderedPairs.value.indexOf(ui.currentPair)
    if (idx < 0) return

    const container = (
      containerRef.value?.querySelector('.p-virtualscroller') ||
      containerRef.value?.querySelector('.p-datatable-table-container')
    ) as HTMLElement | null
    if (!container) return

    const firstRow = container.querySelector('tbody tr') as HTMLElement | null
    const rowH = firstRow?.offsetHeight || ROW_HEIGHT
    const offset = idx * rowH - container.clientHeight / 2 + rowH / 2

    container.scrollTo({ top: Math.max(0, offset), behavior: 'auto' })
  }

  watch(() => ui.currentPair, () => scrollToPair())
  watch(orderedPairs, () => scrollToPair())

  // Скроллить можно только когда панель видима — иначе высота контейнера равна нулю.
  watch(() => ui.activeDataPanel, async (val) => {
    if (val !== panel) return
    await nextTick()
    requestAnimationFrame(() => scrollToPair())
  })

  return { scrollToPair }
}
