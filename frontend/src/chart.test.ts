import { describe, it, expect } from 'vitest'
import { getChartData, makeDataset } from './chart'
import type { Coverage } from './core'

describe('getChartData', () => {
  it('returns a config object with line type', () => {
    const config = getChartData()
    expect(config.type).toBe('line')
  })

  it('has data property with datasets array', () => {
    const config = getChartData()
    expect(config.data.datasets).toEqual([])
    expect(config.data.labels).toBeNull()
  })

  it('has options with time scale on x axis', () => {
    const config = getChartData()
    expect(config.options.scales.x.type).toBe('time')
    expect(config.options.scales.y.type).toBe('linear')
  })

  it('has tooltip with label callback', () => {
    const config = getChartData()
    const cb = config.options.plugins.tooltip.callbacks.label
    const result = cb({ dataset: { data: [{ index: 1, y: 85.5 }] }, dataIndex: 0, raw: { y: 85.5 } })
    expect(result).toMatch(/85\.5%/)
  })
})

describe('makeDataset', () => {
  it('returns empty array for empty coverages', () => {
    const datasets = makeDataset([])
    expect(datasets).toHaveLength(0)
  })

  it('creates single dataset for single-entry coverages', () => {
    const coverages: Coverage[] = [
      { index: 1, hits: 50, lines: 100, entries: [{ name: '_default', hits: 50, lines: 100 }], revision: 'abc123', revision_url: '', time: '2024-01-01T00:00:00Z' },
      { index: 2, hits: 90, lines: 100, entries: [{ name: '_default', hits: 90, lines: 100 }], revision: 'def456', revision_url: '', time: '2024-01-02T00:00:00Z' },
    ]
    const datasets = makeDataset(coverages)
    expect(datasets).toHaveLength(1)
    expect(datasets[0].label).toBe('coverage')
    expect(datasets[0].data).toHaveLength(2)
    expect(datasets[0].data[0]).toEqual({ x: '2024-01-01T00:00:00Z', y: 50.0, index: 1 })
    expect(datasets[0].data[1]).toEqual({ x: '2024-01-02T00:00:00Z', y: 90.0, index: 2 })
  })

  it('creates per-entry datasets plus total for multi-entry coverages', () => {
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
    const datasets = makeDataset(coverages)
    expect(datasets.length).toBe(3)
    const labels = datasets.map((d: any) => d.label)
    expect(labels).toContain('src')
    expect(labels).toContain('test')
    expect(labels).toContain('total')
    const total = datasets.find((d: any) => d.label === 'total')
    expect(total.data[0].y).toBe(75.0)
  })

  it('handles coverage with no entries gracefully', () => {
    const coverages: Coverage[] = [
      { index: 1, hits: 0, lines: 0, entries: [], revision: '', revision_url: '', time: '2024-01-01T00:00:00Z' },
    ]
    const datasets = makeDataset(coverages)
    expect(datasets).toHaveLength(0)
  })
})
