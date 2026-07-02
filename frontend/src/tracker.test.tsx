import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { useLoaderData } from 'react-router'
import {
  TrackerCreate,
  TrackerView,
  TrackerDetailView,
  TrackerDetailEdit,
  loadTrackerList,
  loadTrackerDetail,
  patchTracker,
} from './tracker'

const mockNavigate = vi.fn()
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return { ...actual, useLoaderData: vi.fn(), useNavigate: () => mockNavigate }
})

vi.mock('echarts-for-react', () => ({
  default: () => <div data-testid="echart" />,
}))

vi.mock('react-datepicker', () => ({
  default: (props: any) => <input data-testid="datepicker" {...props} />,
}))

describe('loadTrackerList', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns paginated trackers from successful API response', async () => {
    const mockResponse = {
      trackers: [
        { id: 1, name: 'tracker-a', visibility: 'private', role: 'owner', liked: false },
        { id: 2, name: 'tracker-b', visibility: 'public', role: '', liked: true },
      ],
      total: 2,
      page: 1,
      per_page: 12,
    }
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    } as Response)

    const result = await loadTrackerList()
    expect(result).toEqual(mockResponse)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/tracker?page=1&per_page=12')
  })

  it('throws on non-ok response', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: false,
      status: 500,
    } as Response)

    await expect(loadTrackerList()).rejects.toBeDefined()
  })
})

describe('loadTrackerDetail', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns tracker detail from API response', async () => {
    const mockResponse = {
      tracker: { id: 1, name: 'test', visibility: 'private', role: 'owner', liked: false },
      series: [{ id: 1, tracker_id: 1, name: 's1', data_type: 'float' }],
    }
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    } as Response)

    const params = { trackerId: '1' }
    const args = { params, request: {} as Request, url: new URL('http://localhost'), pattern: '/', context: {} }
    const result = await loadTrackerDetail(args)
    expect(result).toEqual(mockResponse)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/tracker/1/series')
  })

  it('throws on missing trackerId', async () => {
    const params = {}
    const args = { params, request: {} as Request, url: new URL('http://localhost'), pattern: '/', context: {} }
    await expect(loadTrackerDetail(args)).rejects.toThrow('trackerId is required')
  })
})

describe('patchTracker', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('sends PATCH request with visibility and returns updated tracker', async () => {
    const updated = { id: 1, name: 'test', visibility: 'public', role: 'owner', liked: false }
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(updated),
    } as Response)

    const result = await patchTracker(1, 'public')
    expect(result).toEqual(updated)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/tracker/1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ visibility: 'public' }),
    })
  })

  it('throws on non-ok response', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: false,
      status: 403,
    } as Response)

    await expect(patchTracker(1, 'public')).rejects.toBeDefined()
  })
})

describe('TrackerView', () => {
  beforeEach(() => {
    vi.mocked(useLoaderData).mockReset()
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders Trackers heading', () => {
    vi.mocked(useLoaderData).mockReturnValue({ trackers: [], total: 0, page: 1, per_page: 12 })
    render(<MemoryRouter><TrackerView /></MemoryRouter>)
    expect(screen.getByText('Trackers')).toBeInTheDocument()
  })

  it('shows Create Tracker link', () => {
    vi.mocked(useLoaderData).mockReturnValue({ trackers: [], total: 0, page: 1, per_page: 12 })
    render(<MemoryRouter><TrackerView /></MemoryRouter>)
    const link = screen.getByText('Create Tracker').closest('a')
    expect(link).toHaveAttribute('href', '/tracker/new')
  })

  it('renders tracker cards', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      trackers: [
        { id: 1, name: 'tracker-a', visibility: 'private', role: 'owner', liked: false },
        { id: 2, name: 'tracker-b', visibility: 'public', role: '', liked: true },
      ],
      total: 2,
      page: 1,
      per_page: 12,
    })
    render(<MemoryRouter><TrackerView /></MemoryRouter>)
    expect(screen.getByText('tracker-a')).toBeInTheDocument()
    expect(screen.getByText('tracker-b')).toBeInTheDocument()
  })

  it('shows role badge for owned trackers', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      trackers: [
        { id: 1, name: 'my-tracker', visibility: 'private', role: 'owner', liked: false },
      ],
      total: 1,
      page: 1,
      per_page: 12,
    })
    render(<MemoryRouter><TrackerView /></MemoryRouter>)
    expect(screen.getByText('owner')).toBeInTheDocument()
  })

  it('shows empty state when no trackers', () => {
    vi.mocked(useLoaderData).mockReturnValue({ trackers: [], total: 0, page: 1, per_page: 12 })
    render(<MemoryRouter><TrackerView /></MemoryRouter>)
    expect(screen.getByText('No trackers yet.')).toBeInTheDocument()
  })

  it('renders pagination controls when multiple pages', () => {
    const trackers = Array.from({ length: 12 }, (_, i) => ({
      id: i + 1,
      name: `tracker-${i}`,
      visibility: 'private' as const,
      role: 'owner' as const,
      liked: false,
    }))
    vi.mocked(useLoaderData).mockReturnValue({
      trackers,
      total: 24,
      page: 1,
      per_page: 12,
    })
    render(<MemoryRouter><TrackerView /></MemoryRouter>)
    expect(screen.getByText('Page 1 of 2')).toBeInTheDocument()
    expect(screen.getByText('Next')).toBeInTheDocument()
  })
})

describe('TrackerCreate', () => {
  beforeEach(() => {
    mockNavigate.mockReset()
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders form with name input, visibility select, and buttons', () => {
    render(<MemoryRouter><TrackerCreate /></MemoryRouter>)
    expect(screen.getByPlaceholderText('Tracker name')).toBeInTheDocument()
    expect(screen.getByText('Create Tracker')).toBeInTheDocument()
    expect(screen.getByText('Cancel')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Private')).toBeInTheDocument()
    const cancelLink = screen.getByText('Cancel').closest('a')
    expect(cancelLink).toHaveAttribute('href', '/tracker')
  })

  it('creates tracker and navigates on submit', async () => {
    const created = { id: 42, name: 'new-tracker', visibility: 'private', role: 'owner', liked: false }
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(created),
    } as Response)

    render(<MemoryRouter><TrackerCreate /></MemoryRouter>)
    const input = screen.getByPlaceholderText('Tracker name')
    const createBtn = screen.getByText('Create')

    input.focus()
    input.setAttribute('value', 'new-tracker')
    input.dispatchEvent(new Event('change', { bubbles: true }))

    createBtn.click()

    await vi.waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/tracker', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: 'new-tracker', visibility: 'private' }),
      })
      expect(mockNavigate).toHaveBeenCalledWith('/tracker/42')
    })
  })

  it('shows error on API failure', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: false,
      status: 400,
    } as Response)

    render(<MemoryRouter><TrackerCreate /></MemoryRouter>)
    const input = screen.getByPlaceholderText('Tracker name')
    const createBtn = screen.getByText('Create')

    input.setAttribute('value', 'fail-tracker')
    input.dispatchEvent(new Event('change', { bubbles: true }))

    createBtn.click()

    await vi.waitFor(() => {
      expect(screen.getByText('Failed to create tracker. Please try again.')).toBeInTheDocument()
    })
  })
})

describe('TrackerDetailView', () => {
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

  it('renders tracker name', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test-tracker', visibility: 'private', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailView /></MemoryRouter>)
    expect(screen.getByText('test-tracker')).toBeInTheDocument()
  })

  it('renders Like button', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailView /></MemoryRouter>)
    expect(screen.getByText('Like')).toBeInTheDocument()
  })

  it('renders Unlike button when liked', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', role: '', liked: true },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailView /></MemoryRouter>)
    expect(screen.getByText('Unlike')).toBeInTheDocument()
  })

  it('shows Edit button when user has role', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailView /></MemoryRouter>)
    expect(screen.getByText('Edit')).toBeInTheDocument()
  })

  it('hides Edit button when user has no role', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailView /></MemoryRouter>)
    expect(screen.queryByText('Edit')).not.toBeInTheDocument()
  })

  it('renders datepickers', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailView /></MemoryRouter>)
    const datepickers = screen.getAllByTestId('datepicker')
    expect(datepickers.length).toBeGreaterThanOrEqual(2)
  })

  it('renders series table with name and data type', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', role: '', liked: false },
      series: [{ id: 1, tracker_id: 1, name: 'series-a', data_type: 'float' }],
    })
    render(<MemoryRouter><TrackerDetailView /></MemoryRouter>)
    expect(screen.getByText('series-a')).toBeInTheDocument()
    expect(screen.getByText('float')).toBeInTheDocument()
  })
})

describe('TrackerDetailEdit', () => {
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

  it('renders edit heading with tracker name', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'edit-tracker', visibility: 'private', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    expect(screen.getByText(/edit-tracker \(Edit\)/)).toBeInTheDocument()
  })

  it('renders Add Series form', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    expect(screen.getByPlaceholderText('Series name')).toBeInTheDocument()
    expect(screen.getByText('Add Series')).toBeInTheDocument()
  })

  it('renders Add Value form', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', role: 'owner', liked: false },
      series: [{ id: 1, tracker_id: 1, name: 's1', data_type: 'float' }],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    expect(screen.getByText('Add')).toBeInTheDocument()
  })

  it('back link goes to tracker detail', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    const backLink = screen.getByText(/Back to Tracker/).closest('a')
    expect(backLink).toHaveAttribute('href', '/tracker/1')
  })

  it('renders visibility selector', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'unlisted', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    expect(screen.getByText('Visibility')).toBeInTheDocument()
    const select = screen.getByDisplayValue('Unlisted')
    expect(select).toBeInTheDocument()
  })
})
