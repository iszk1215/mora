import { describe, it, expect, vi, beforeEach, beforeAll, afterAll } from 'vitest'
import { render, screen } from '@testing-library/react'
import { useLoaderData, useParams } from 'react-router'
import { formatRevision, formatRatio, formatTime, CoverageEntryPage, CoverageTrackerList, makeCoverageSeries, coverageToDatasets, buildCoverageClickUrl } from './coverage'
import { Coverage } from './core'
import { useUser } from './user-context'

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return { ...actual, useLoaderData: vi.fn(), useParams: vi.fn() }
})

vi.mock('echarts-for-react', () => ({
  default: () => <div data-testid="echart" />,
}))

vi.mock('./user-context', () => ({
  useUser: vi.fn(),
}))

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

  it('returns N/A when lines is 0', () => {
    expect(formatRatio(0, 0)).toBe('N/A')
    expect(formatRatio(75, 0)).toBe('N/A')
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

describe('CoverageEntryPage', () => {
  beforeEach(() => {
    vi.mocked(useLoaderData).mockReturnValue({
      meta: {
        hits: 75,
        lines: 100,
        revision: 'abcdefghijklmnop',
        revision_url: 'https://example.com/commit/abc123',
        time: '2024-01-15T10:30:00Z',
      },
      files: [],
    })
  })

  it('renders revision link with href from meta.revision_url', () => {
    render(<CoverageEntryPage />)
    const link = screen.getByText('abcdefghij').closest('a')
    expect(link).toHaveAttribute('href', 'https://example.com/commit/abc123')
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

describe('coverageToDatasets', () => {
  it('includes index in extra for each data point', () => {
    const coverages: Coverage[] = [
      {
        index: 5, hits: 90, lines: 100, revision: 'abc', revision_url: '',
        time: '2024-01-15T10:00:00Z',
        entries: [{ name: 'go', hits: 90, lines: 100 }],
      },
      {
        index: 8, hits: 80, lines: 100, revision: 'def', revision_url: '',
        time: '2024-06-15T10:00:00Z',
        entries: [{ name: 'go', hits: 80, lines: 100 }],
      },
    ]
    const datasets = coverageToDatasets(coverages)
    expect(datasets[0].data[0].extra).toEqual({ index: 5, entryName: 'go' })
    expect(datasets[0].data[1].extra).toEqual({ index: 8, entryName: 'go' })
  })

  it('includes total series with extra.index when multiple entries exist', () => {
    const coverages: Coverage[] = [
      {
        index: 1, hits: 150, lines: 200, revision: 'abc', revision_url: '',
        time: '2024-01-15T10:00:00Z',
        entries: [
          { name: 'go', hits: 90, lines: 100 },
          { name: 'py', hits: 60, lines: 100 },
        ],
      },
    ]
    const datasets = coverageToDatasets(coverages)
    const totalDataset = datasets.find(d => d.label === 'total')
    expect(totalDataset).toBeDefined()
    expect(totalDataset!.data[0].extra).toEqual({ index: 1, entryName: 'total' })
  })
})

describe('buildCoverageClickUrl', () => {
  it('builds url from trackerId', () => {
    expect(buildCoverageClickUrl('2', '5', 3, 'go'))
      .toBe('/coverages/5/3/go')
  })

  it('handles numeric index', () => {
    expect(buildCoverageClickUrl('42', '10', 10, 'py'))
      .toBe('/coverages/10/10/py')
  })
})

const mockRepo = { id: 1, url: 'https://example.com/repo', namespace: 'test', name: 'test-repo' }

describe('CoverageTrackerList', () => {
  beforeEach(() => {
    vi.mocked(useUser).mockReturnValue({ id: 1, provider: 'local', provider_user_id: 'user1', username: 'testuser', avatar_url: '' })
    vi.mocked(useParams).mockReturnValue({ trackerId: '42' })
    vi.mocked(useLoaderData).mockReturnValue({
      trackerName: 'Test Coverage',
      repo: mockRepo,
      coverages: [],
      liked: false,
      likeCount: 0,
      trackerId: 42,
    })
  })

  it('renders the like button when user is logged in', () => {
    render(<CoverageTrackerList />)
    const button = screen.getByRole('button', { name: 'Like' })
    expect(button).toBeInTheDocument()
    expect(button).not.toBeDisabled()
  })

  it('renders the like button as liked when liked is true', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      trackerName: 'Test Coverage',
      repo: mockRepo,
      coverages: [],
      liked: true,
      likeCount: 5,
      trackerId: 42,
    })
    render(<CoverageTrackerList />)
    const button = screen.getByRole('button', { name: 'Unlike' })
    expect(button).toBeInTheDocument()
    expect(screen.getByText('5')).toBeInTheDocument()
  })

  it('disables the like button when user is not logged in', () => {
    vi.mocked(useUser).mockReturnValue(null)
    render(<CoverageTrackerList />)
    const button = screen.getByRole('button', { name: 'Like' })
    expect(button).toBeDisabled()
  })

  it('does not render like count when likeCount is 0', () => {
    render(<CoverageTrackerList />)
    expect(screen.getByRole('button', { name: 'Like' })).toBeInTheDocument()
    expect(screen.queryByText('0')).not.toBeInTheDocument()
  })
})
