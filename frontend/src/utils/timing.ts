// Замер длительности async-вызова для диагностики "почему обновление идёт
// долго" - какой из параллельных запросов внутри Promise.all тормозит,
// не видно просто по общему времени кнопки.
export async function timed<T>(label: string, fn: () => Promise<T>): Promise<T> {
  const start = performance.now()
  try {
    return await fn()
  } finally {
    console.log(`[timing] ${label}: ${(performance.now() - start).toFixed(0)}ms`)
  }
}
