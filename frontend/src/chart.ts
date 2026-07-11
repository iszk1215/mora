import { Coverage, SeriesConfig } from './core'

interface Point {
  x: string
  y: number
  index: number
}

export interface Dataset {
  data: Array<{ x: string; y: string }>
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

export function getCoverageOption() {
  return {
    grid: { left: 60, right: 20, top: 40, bottom: 60 },
    xAxis: {
      type: 'time' as const,
    },
    yAxis: {
      type: 'value' as const,
      name: 'Coverage %',
      axisLabel: {
        formatter: '{value}%',
      },
    },
    tooltip: {
      trigger: 'axis',
      valueFormatter: (value: number) => formatValue(value, '%.1f%%'),
    },
    animation: false,
    legend: {
      type: 'scroll',
      top: 0,
    },
  }
}

export function makeCoverageSeries(coverages: Coverage[]) {
  const map: { [name: string]: Point[] } = {}

  const hasMultiEntries = coverages.reduce(
    (flag: boolean, cov: Coverage) => flag || cov.entries.length > 1,
    false
  )
  if (hasMultiEntries) {
    map.total = []
  }

  for (const cov of coverages) {
    for (const e of cov.entries) {
      if (!(e.name in map)) {
        map[e.name] = []
      }
      map[e.name].push(
        { x: cov.time, y: e.lines === 0 ? 0 : e.hits * 100.0 / e.lines, index: cov.index }
      )
    }
    if (hasMultiEntries) {
      map.total.push(
        { x: cov.time, y: cov.lines === 0 ? 0 : cov.hits * 100.0 / cov.lines, index: cov.index }
      )
    }
  }

  const series = []
  for (const k in map) {
    const name = k === '_default' ? 'coverage' : k
    series.push({
      name,
      type: 'line' as const,
      data: map[k].map(p => ({ value: [p.x, p.y], index: p.index })),
    })
  }

  return series
}

export function coverageToDatasets(coverages: Coverage[]): Dataset[] {
  const map: { [name: string]: Array<{ x: string; y: string }> } = {}

  const hasMultiEntries = coverages.reduce(
    (flag: boolean, cov: Coverage) => flag || cov.entries.length > 1,
    false
  )
  if (hasMultiEntries) {
    map.total = []
  }

  for (const cov of coverages) {
    for (const e of cov.entries) {
      if (!(e.name in map)) {
        map[e.name] = []
      }
      const y = e.lines === 0 ? 0 : e.hits * 100.0 / e.lines
      map[e.name].push({ x: cov.time, y: y.toFixed(1) })
    }
    if (hasMultiEntries) {
      const y = cov.lines === 0 ? 0 : cov.hits * 100.0 / cov.lines
      map.total.push({ x: cov.time, y: y.toFixed(1) })
    }
  }

  const datasets: Dataset[] = []
  for (const k in map) {
    const label = k === '_default' ? 'coverage' : k
    datasets.push({
      label,
      data: map[k],
      seriesConfig: { value_format: '%.1f%%' },
    })
  }

  return datasets
}
