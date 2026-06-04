import { describe, it, expect } from 'vitest'
import { getCoverageOption, makeCoverageSeries } from './chart'
import type { Coverage } from './core'

describe('getCoverageOption', () => {
  it('returns an option object with grid', () => {
    const opt = getCoverageOption()
    expect(opt).toHaveProperty('grid')
  })

  it('has xAxis with time type', () => {
    const opt = getCoverageOption()
    expect(opt.xAxis.type).toBe('time')
  })

  it('has yAxis with value type and name', () => {
    const opt = getCoverageOption()
    expect(opt.yAxis.type).toBe('value')
    expect(opt.yAxis.name).toBe('Coverage %')
  })

  it('has animation disabled', () => {
    const opt = getCoverageOption()
    expect(opt.animation).toBe(false)
  })

  it('has tooltip with axis trigger', () => {
    const opt = getCoverageOption()
    expect(opt.tooltip.trigger).toBe('axis')
  })
})

describe('makeCoverageSeries', () => {
  it('returns empty array for empty coverages', () => {
    const series = makeCoverageSeries([])
    expect(series).toHaveLength(0)
  })

  it('creates single series for single-entry coverages', () => {
    const coverages: Coverage[] = [
      { index: 1, hits: 50, lines: 100, entries: [{ name: '_default', hits: 50, lines: 100 }], revision: 'abc123', revision_url: '', time: '2024-01-01T00:00:00Z' },
      { index: 2, hits: 90, lines: 100, entries: [{ name: '_default', hits: 90, lines: 100 }], revision: 'def456', revision_url: '', time: '2024-01-02T00:00:00Z' },
    ]
    const series = makeCoverageSeries(coverages)
    expect(series).toHaveLength(1)
    expect(series[0].name).toBe('coverage')
    expect(series[0].type).toBe('line')
    expect(series[0].data).toHaveLength(2)
    expect(series[0].data[0]).toEqual({ value: ['2024-01-01T00:00:00Z', 50.0], index: 1 })
    expect(series[0].data[1]).toEqual({ value: ['2024-01-02T00:00:00Z', 90.0], index: 2 })
  })

  it('creates per-entry series plus total for multi-entry coverages', () => {
    const coverages: Coverage[] = [
      {
        index: 1, hits: 150, lines: 200,
        entries: [
          { name: 'src', hits: 80, lines: 100 },
          { name: 'test', hits: 70, lines: 100 },
        ],
        revision: 'abc123', revision_url: '', time: '2024-01-01T00:00:00Z',
      },
    ]
    const series = makeCoverageSeries(coverages)
    expect(series.length).toBe(3)
    const names = series.map((s: any) => s.name)
    expect(names).toContain('src')
    expect(names).toContain('test')
    expect(names).toContain('total')
    const total = series.find((s: any) => s.name === 'total')!
    expect(total.data[0].value[1]).toBe(75.0)
  })

  it('handles coverage with no entries gracefully', () => {
    const coverages: Coverage[] = [
      { index: 1, hits: 0, lines: 0, entries: [], revision: '', revision_url: '', time: '2024-01-01T00:00:00Z' },
    ]
    const series = makeCoverageSeries(coverages)
    expect(series).toHaveLength(0)
  })
})
