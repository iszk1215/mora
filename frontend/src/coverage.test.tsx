import { describe, it, expect, vi, beforeEach, beforeAll, afterAll } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, useLoaderData, useParams } from 'react-router'
import { formatRevision, formatRatio, formatTime, makeRepoCoverageListPath, makeEntryPath, CoverageEntryPage, CoverageList, makeCoverageSeries, coverageToDatasets, buildCoverageClickUrl } from './coverage'
import { Coverage } from './core'

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return { ...actual, useLoaderData: vi.fn(), useParams: vi.fn() }
})

vi.mock('echarts-for-react', () => ({
  default: () => <div data-testid="echart" />,
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

const makeCoverages = (): Coverage[] => [
  {
    index: 1, hits: 90, lines: 100, revision: 'abc', revision_url: '',
    time: '2024-01-15T10:00:00Z',
    entries: [{ name: 'go', hits: 90, lines: 100 }],
  },
  {
    index: 2, hits: 50, lines: 100, revision: 'def', revision_url: '',
    time: '2024-06-15T10:00:00Z',
    entries: [{ name: 'py', hits: 50, lines: 100 }],
  },
  {
    index: 3, hits: 80, lines: 100, revision: 'ghi', revision_url: '',
    time: '2024-12-25T10:00:00Z',
    entries: [{ name: 'js', hits: 80, lines: 100 }],
  },
]

describe('CoverageList', () => {
  beforeEach(() => {
    vi.mocked(useLoaderData).mockReturnValue({
      repo: { id: 1, url: 'https://example.com/repo', namespace: 'ns', name: 'repo' },
      coverages: makeCoverages(),
    })
    vi.mocked(useParams).mockReturnValue({ repo_id: '1' })
  })

  it('renders all coverage segments', () => {
    render(<MemoryRouter><CoverageList /></MemoryRouter>)
    expect(screen.getByText(/#1/)).toBeInTheDocument()
    expect(screen.getByText(/#2/)).toBeInTheDocument()
    expect(screen.getByText(/#3/)).toBeInTheDocument()
  })

  it('renders coverage entry links', () => {
    render(<MemoryRouter><CoverageList /></MemoryRouter>)
    const links = screen.getAllByRole('link')
    const entryLinks = links.filter(l => l.getAttribute('href')?.startsWith('/repos/1/coverages/'))
    expect(entryLinks).toHaveLength(3)
    expect(entryLinks[0]).toHaveAttribute('href', '/repos/1/coverages/1/go')
    expect(entryLinks[1]).toHaveAttribute('href', '/repos/1/coverages/2/py')
    expect(entryLinks[2]).toHaveAttribute('href', '/repos/1/coverages/3/js')
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
  it('builds url from repoId, index, and series name', () => {
    expect(buildCoverageClickUrl('2', 3, 'go'))
      .toBe('/repos/2/coverages/3/go')
  })

  it('handles numeric index', () => {
    expect(buildCoverageClickUrl('42', 10, 'py'))
      .toBe('/repos/42/coverages/10/py')
  })
})
