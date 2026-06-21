import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { useLoaderData } from 'react-router'
import { UdmRoot, UdmChart, UdmMetricRoot, loadUdmMetrics, loadMetricItems, loadMetricValues } from './udm'

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return { ...actual, useLoaderData: vi.fn() }
})

vi.mock('echarts-for-react', () => ({
  default: () => <div data-testid="echart" />,
}))

vi.mock('react-datepicker', () => ({
  default: (props: any) => <input data-testid="datepicker" {...props} />,
}))

describe('loadUdmMetrics', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns metrics array from successful API response', async () => {
    const mockMetrics = [
      { id: 1, repod_id: 1, name: 'metric1' },
      { id: 2, repod_id: 1, name: 'metric2' },
    ]
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ repo: {}, metrics: mockMetrics }),
    } as Response)

    const result = await loadUdmMetrics(1)
    expect(result).toEqual(mockMetrics)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/repos/1/udm/metrics')
  })

  it('throws on non-ok response', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: false,
      status: 500,
    } as Response)

    await expect(loadUdmMetrics(1)).rejects.toBeDefined()
  })
})

describe('loadMetricItems', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('redirects on 403 response', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      status: 403,
      ok: false,
    } as Response)

    const params = { repo_id: '1', metric_id: '2' }
    const args = { params, request: {} as Request, url: new URL('http://localhost'), pattern: '/', context: {} }
    const result = await loadMetricItems(args)
    expect(result.status).toBe(302)
  })

  it('throws on non-ok non-403 response', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      status: 500,
      ok: false,
    } as Response)

    const params = { repo_id: '1', metric_id: '2' }
    const args = { params, request: {} as Request, url: new URL('http://localhost'), pattern: '/', context: {} }
    await expect(
      loadMetricItems(args)
    ).rejects.toBeDefined()
  })

  it('returns response on success', async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
    } as Response
    vi.mocked(globalThis.fetch).mockResolvedValue(mockResponse)

    const params = { repo_id: '1', metric_id: '2' }
    const args = { params, request: {} as Request, url: new URL('http://localhost'), pattern: '/', context: {} }
    const result = await loadMetricItems(args)
    expect(result).toBe(mockResponse)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/repos/1/udm/metrics/2/items')
  })
})

describe('loadMetricValues', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns values from successful API response', async () => {
    const mockValues = [
      { id: 1, item_id: 1, time: '2024-01-01', value: '10' },
    ]
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ item: {}, values: mockValues }),
    } as Response)

    const result = await loadMetricValues(1, 2, 3)
    expect(result.values).toEqual(mockValues)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/repos/1/udm/metrics/2/items/3/values')
  })

  it('throws on non-ok response', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: false,
      status: 500,
    } as Response)

    await expect(loadMetricValues(1, 2, 3)).rejects.toBeDefined()
  })
})

describe('UdmRoot', () => {
  beforeEach(() => {
    vi.mocked(useLoaderData).mockReset()
  })

  it('renders User Defined Metrics heading', () => {
    vi.mocked(useLoaderData).mockReturnValue({ repo: { id: 1 }, metrics: [] })
    render(<MemoryRouter><UdmRoot /></MemoryRouter>)
    expect(screen.getByText('User Defined Metrics')).toBeInTheDocument()
  })

  it('renders metric names as links', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      repo: { id: 1, url: '', namespace: '', name: '' },
      metrics: [
        { id: 1, repod_id: 1, name: 'metric-a' },
        { id: 2, repod_id: 1, name: 'metric-b' },
      ],
    })
    render(<MemoryRouter><UdmRoot /></MemoryRouter>)
    const linkA = screen.getByText('metric-a').closest('a')
    expect(linkA).toHaveAttribute('href', '/repos/1/udm/metrics/1')
    const linkB = screen.getByText('metric-b').closest('a')
    expect(linkB).toHaveAttribute('href', '/repos/1/udm/metrics/2')
  })
})

describe('UdmChart', () => {
  it('renders ECharts component', () => {
    render(<UdmChart data={{ datasets: [] }} ylabel="test" />)
    expect(screen.getByTestId('echart')).toBeInTheDocument()
  })

  it('renders with dataset series', () => {
    const datasets = [
      {
        label: 'item-1',
        data: [
          { x: '2024-01-01', y: '10' },
          { x: '2024-01-02', y: '20' },
        ],
      },
    ]
    render(<UdmChart data={{ datasets }} ylabel="test" />)
    expect(screen.getByTestId('echart')).toBeInTheDocument()
  })

  it('sets min and max when provided', () => {
    const min = new Date('2024-01-01')
    const max = new Date('2024-01-31')
    render(<UdmChart data={{ datasets: [] }} ylabel="test" min={min} max={max} />)
    const el = screen.getByTestId('echart')
    expect(el).toBeInTheDocument()
  })
})

describe('UdmMetricRoot', () => {
  beforeEach(() => {
    vi.mocked(useLoaderData).mockReset()
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ values: [] }),
    } as Response)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders datepickers', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      repo: { id: 1, url: '', namespace: '', name: '' },
      metric: { id: 1, repod_id: 1, name: 'test' },
      items: [],
    })
    render(<MemoryRouter><UdmMetricRoot /></MemoryRouter>)
    const datepickers = screen.getAllByTestId('datepicker')
    expect(datepickers).toHaveLength(2)
  })

  it('renders chart', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      repo: { id: 1, url: '', namespace: '', name: '' },
      metric: { id: 1, repod_id: 1, name: 'test' },
      items: [],
    })
    render(<MemoryRouter><UdmMetricRoot /></MemoryRouter>)
    expect(screen.getByTestId('echart')).toBeInTheDocument()
  })

  it('renders chart with metric name as ylabel', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      repo: { id: 1, url: '', namespace: '', name: '' },
      metric: { id: 1, repod_id: 1, name: 'my-metric' },
      items: [],
    })
    render(<MemoryRouter><UdmMetricRoot /></MemoryRouter>)
    expect(screen.getByTestId('echart')).toBeInTheDocument()
  })

  it('renders "From" and "To" labels', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      repo: { id: 1, url: '', namespace: '', name: '' },
      metric: { id: 1, repod_id: 1, name: 'test' },
      items: [],
    })
    render(<MemoryRouter><UdmMetricRoot /></MemoryRouter>)
    expect(screen.getByText('From')).toBeInTheDocument()
    expect(screen.getByText('To')).toBeInTheDocument()
  })
})
