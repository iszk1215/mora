import { describe, it, expect } from 'vitest'
import { formatValue, makeCoverageSeries } from './chart'
import { Coverage } from './core'

describe('formatValue', () => {
  it('returns default format when fmt is undefined', () => {
    expect(formatValue(42, undefined)).toBe('42')
    expect(formatValue(42.5, undefined)).toBe('42.5')
    expect(formatValue(42.567, undefined)).toBe('42.6')
  })

  it('returns default format when fmt is empty string', () => {
    expect(formatValue(42, '')).toBe('42')
    expect(formatValue(42.5, '')).toBe('42.5')
  })

  it('formats with .Nf pattern', () => {
    expect(formatValue(42.567, '%.1f')).toBe('42.6')
    expect(formatValue(42.567, '%.2f')).toBe('42.57')
    expect(formatValue(42.567, '%.3f')).toBe('42.567')
    expect(formatValue(42.567, '%.0f')).toBe('43')
  })

  it('formats with %d pattern', () => {
    expect(formatValue(42.567, '%d')).toBe('43')
    expect(formatValue(42.4, '%d')).toBe('42')
  })

  it('includes literal % via %%', () => {
    expect(formatValue(42.567, '%.1f%%')).toBe('42.6%')
    expect(formatValue(42.567, 'Value: %.2f%%')).toBe('Value: 42.57%')
  })

  it('handles integer values', () => {
    expect(formatValue(0, '%.2f')).toBe('0.00')
    expect(formatValue(100, '%.1f')).toBe('100.0')
  })
})

describe('makeCoverageSeries', () => {
  it('returns 0 instead of Infinity when lines is 0', () => {
    const coverages: Coverage[] = [{
      index: 1,
      time: '2024-01-01T00:00:00Z',
      hits: 0,
      lines: 0,
      revision: '',
      revision_url: '',
      entries: [{ name: 'src/main.go', hits: 0, lines: 0 }],
    }]
    const series = makeCoverageSeries(coverages)
    expect(series[0].data[0].value[1]).toBe(0)
  })
})
