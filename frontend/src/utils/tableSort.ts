// Реплицирует то, как PrimeVue DataTable сравнивает значения при клике по
// сортируемому столбцу - нужно, чтобы наш собственный порядок строк (для
// useScrollToPair) совпадал с тем, что реально отрисовано в DOM после сортировки.
export function compareByField<T>(
  a: T,
  b: T,
  field: string,
  order: number,
): number {
  const av = (a as Record<string, unknown>)[field]
  const bv = (b as Record<string, unknown>)[field]

  if (av === bv) return 0
  if (av == null) return order
  if (bv == null) return -order

  if (typeof av === 'string' && typeof bv === 'string') {
    return order * av.localeCompare(bv)
  }

  return order * ((av as number) - (bv as number))
}
