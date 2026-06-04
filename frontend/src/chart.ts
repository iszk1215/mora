import { Coverage } from './core'

interface Point {
  x: string
  y: number
  index: number
}

export function getCoverageOption() {
  return {
    grid: { left: 60, right: 20, top: 20, bottom: 40 },
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
      trigger: 'axis' as const,
    },
    animation: false,
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
        { x: cov.time, y: e.hits * 100.0 / e.lines, index: cov.index }
      )
    }
    if (hasMultiEntries) {
      map.total.push(
        { x: cov.time, y: cov.hits * 100.0 / cov.lines, index: cov.index }
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
