import { describe, it, expect, vi } from 'vitest'
import { formatRevision, formatRatio, formatTime, makeRepoCoverageListPath, makeEntryPath } from './coverage'

describe('formatRevision', () => {
  it('returns first 10 characters of revision', () => {
    expect(formatRevision('abcdefghijklm')).toBe('abcdefghij')
  })

  it('returns full string if shorter than 10', () => {
    expect(formatRevision('abc')).toBe('abc')
  })

  it('handles empty string', () => {
    expect(formatRevision('')).toBe('')
  })
})

describe('formatRatio', () => {
  it('formats as percentage with one decimal', () => {
    expect(formatRatio(75, 100)).toBe('75.0')
  })

  it('rounds to one decimal place', () => {
    expect(formatRatio(1, 3)).toBe('33.3')
  })

  it('handles full coverage', () => {
    expect(formatRatio(100, 100)).toBe('100.0')
  })

  it('handles zero coverage', () => {
    expect(formatRatio(0, 100)).toBe('0.0')
  })
})

describe('formatTime', () => {
  beforeAll(() => {
    vi.setSystemTime(new Date('2024-06-15T12:00:00Z'))
  })

  afterAll(() => {
    vi.useRealTimers()
  })

  it('formats ISO time string to locale string', () => {
    const result = formatTime('2024-01-15T10:30:00Z')
    expect(result).toContain('2024')
    expect(result).toContain('15')
  })
})

describe('makeRepoCoverageListPath', () => {
  it('builds path from repo_id', () => {
    const params = { repo_id: '42' }
    expect(makeRepoCoverageListPath(params)).toBe('repos/42/coverages')
  })
})

describe('makeEntryPath', () => {
  it('builds path from repo_id, index, and entry', () => {
    const params = { repo_id: '42', index: '3', entry: 'src' }
    expect(makeEntryPath(params)).toBe('repos/42/coverages/3/src')
  })
})
