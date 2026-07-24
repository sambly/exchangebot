import type { Time } from 'lightweight-charts'

// lightweight-charts всегда рендерит числовой Time через getUTC*() - то есть
// на оси и в тултипе всегда показывается UTC, часовой пояс браузера
// игнорируется. Чтобы пользователь видел своё локальное время, а не UTC,
// сдвигаем эпоху на локальное смещение ещё до setData()/markers.
export function toChartTime(value: string | number | Date | undefined | null): Time {
  const date = new Date(value ?? 0)
  const utcSeconds = Math.floor(date.getTime() / 1000)
  const offsetSeconds = date.getTimezoneOffset() * 60
  return (utcSeconds - offsetSeconds) as Time
}
