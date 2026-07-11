import React from 'react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { useLoaderData } from 'react-router'
import { CoverageTrackerDetail } from './tracker_coverage'

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return { ...actual, useLoaderData: vi.fn() }
})

vi.mock('echarts-for-react', () => ({
  default: () => <div data-testid="echart" />,
}))

const mockRepo = { id: 1, namespace: 'ns', name: 'repo', url: 'http://example.com', scm_id: 1, created_at: '' }

const mockCoverages = [
  {
    index: 1,
    hits: 90,
    lines: 100,
    revision: 'abc',
    revision_url: '',
    time: '2024-01-15T10:00:00Z',
    entries: [{ name: '_default', hits: 90, lines: 100 }],
  },
]

describe('CoverageTrackerDetail', () => {
  beforeEach(() => {
    vi.mocked(useLoaderData).mockReset()
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ repo: mockRepo, coverages: mockCoverages }),
    } as Response)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders tracker name as h2 title', async () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'my-coverage-tracker', visibility: 'private', type: 'coverage', repo_id: 1, chart_config: '{}', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><CoverageTrackerDetail /></MemoryRouter>)
    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 2, name: 'my-coverage-tracker' })).toBeInTheDocument()
    })
  })

  it('shows "No coverage data" when fetch returns empty coverages', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ repo: mockRepo, coverages: [] }),
    } as Response)
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'tracker', visibility: 'private', type: 'coverage', repo_id: 1, chart_config: '{}', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><CoverageTrackerDetail /></MemoryRouter>)
    await waitFor(() => {
      expect(screen.getByText('No coverage data')).toBeInTheDocument()
    })
  })
})
