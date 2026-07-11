import { SeriesConfig } from './core'

export interface Dataset {
  data: Array<{ x: string; y: string; extra?: Record<string, any> }>
  label: string
  seriesConfig?: SeriesConfig
}

export function formatValue(value: number, fmt: string | undefined): string {
  if (fmt === undefined || fmt === '') {
    return Number.isInteger(value) ? String(value) : value.toFixed(1)
  }
  return fmt.replace(/%(?:\.(\d+))?([df])/g, (_match, precision, type) => {
    if (type === 'd') return String(Math.round(value))
    const p = precision !== undefined ? parseInt(precision) : 6
    return value.toFixed(p)
  }).replace(/%%/g, '%')
}
