import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
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
import { UserProvider } from './user-context'

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
        { id: 1, name: 'tracker-a', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
        { id: 2, name: 'tracker-b', visibility: 'public', type: 'tracker', chart_config: '{}', role: '', liked: true },
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
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/trackers?page=1&per_page=12')
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
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
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
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/trackers/1/series')
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
    const updated = { id: 1, name: 'test', visibility: 'public', type: 'tracker', chart_config: '{}', role: 'owner', liked: false }
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(updated),
    } as Response)

    const result = await patchTracker(1, { visibility: 'public' })
    expect(result).toEqual(updated)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/trackers/1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ visibility: 'public' }),
    })
  })

  it('sends PATCH request with chart_config', async () => {
    const updated = { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{"x_axis_label":"Time","y_axis_label":"Value"}', role: 'owner', liked: false }
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(updated),
    } as Response)

    const result = await patchTracker(1, { chart_config: '{"x_axis_label":"Time","y_axis_label":"Value"}' })
    expect(result).toEqual(updated)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/trackers/1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ chart_config: '{"x_axis_label":"Time","y_axis_label":"Value"}' }),
    })
  })

  it('throws on non-ok response', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: false,
      status: 403,
    } as Response)

    await expect(patchTracker(1, { visibility: 'public' })).rejects.toBeDefined()
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

  it('shows Create Tracker link', () => {
    vi.mocked(useLoaderData).mockReturnValue({ trackers: [], total: 0, page: 1, per_page: 12 })
    render(<MemoryRouter><TrackerView /></MemoryRouter>)
    const link = screen.getByText('Create Tracker').closest('a')
    expect(link).toHaveAttribute('href', '/trackers/new')
  })

  it('renders tracker cards', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      trackers: [
        { id: 1, name: 'tracker-a', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
        { id: 2, name: 'tracker-b', visibility: 'public', type: 'tracker', chart_config: '{}', role: '', liked: true },
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
        { id: 1, name: 'my-tracker', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
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
      type: 'tracker' as const,
      role: 'owner' as const,
      chart_config: '{}' as const,
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
    expect(cancelLink).toHaveAttribute('href', '/trackers')
  })

  it('creates tracker and navigates on submit', async () => {
    const created = { id: 42, name: 'new-tracker', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false }
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
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/trackers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: 'new-tracker', visibility: 'private', type: 'tracker' }),
      })
      expect(mockNavigate).toHaveBeenCalledWith('/trackers/42')
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

  const mockUser = { id: 1, provider: 'github', provider_user_id: '42', username: 'testuser', avatar_url: '' }

  it('renders tracker name', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test-tracker', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByText('test-tracker')).toBeInTheDocument()
  })

  it('renders Like button', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByText('Like')).toBeInTheDocument()
  })

  it('renders Unlike button when liked', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: true },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByText('Unlike')).toBeInTheDocument()
  })

  it('disables Like button when user is not logged in', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={null}><TrackerDetailView /></UserProvider></MemoryRouter>)
    const likeButton = screen.getByText('Like')
    expect(likeButton).toBeDisabled()
  })

  it('shows Edit button when user has role', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByText('Edit')).toBeInTheDocument()
  })

  it('hides Edit button when user has no role', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.queryByText('Edit')).not.toBeInTheDocument()
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
      tracker: { id: 1, name: 'edit-tracker', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    expect(screen.getByText(/edit-tracker \(Edit\)/)).toBeInTheDocument()
  })

  it('renders Add Series form', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    expect(screen.getByPlaceholderText('Series name')).toBeInTheDocument()
    expect(screen.getByText('Add Series')).toBeInTheDocument()
  })

  it('renders Add Value form', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [{ id: 1, tracker_id: 1, name: 's1', data_type: 'float' }],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    expect(screen.getByText('Add Value')).toBeInTheDocument()
  })

  it('back link goes to tracker detail', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    const backLink = screen.getByText(/Back to Tracker/).closest('a')
    expect(backLink).toHaveAttribute('href', '/trackers/1')
  })

  it('renders visibility selector', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    expect(screen.getByText('Visibility')).toBeInTheDocument()
    const select = screen.getByDisplayValue('Private')
    expect(select).toBeInTheDocument()
  })

  it('renders Chart Options section with all inputs', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    expect(screen.getByText('Chart Options')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('X-axis label')).toBeInTheDocument()
    expect(screen.getByText('Y-Axes')).toBeInTheDocument()
    expect(screen.getByText('Area')).toBeInTheDocument()
    expect(screen.getByText('Legend')).toBeInTheDocument()
    expect(screen.getByText('Save Chart Options')).toBeInTheDocument()
  })

  it('pre-fills chart option labels from tracker.chart_config', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{"x_axis_label":"Time","y_axes":[{"id":0,"label":"Value","position":"left"}]}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    expect(screen.getByDisplayValue('Time')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Value')).toBeInTheDocument()
  })

  it('renders Value Format column in series table', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [{ id: 10, tracker_id: 1, name: 's1', data_type: 'float', config: '{}' }],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    expect(screen.getByText('Value Format')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('e.g. %.1f')).toBeInTheDocument()
  })

  it('renders Chart Type and Y-Axis columns in series table', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{"y_axes":[{"id":0,"position":"left"},{"id":1,"position":"right"}]}', role: 'owner', liked: false },
      series: [{ id: 10, tracker_id: 1, name: 's1', data_type: 'float', config: '{"type":"bar","y_axis_index":1}' }],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    expect(screen.getByText('Chart Type')).toBeInTheDocument()
    expect(screen.getByText('Y-Axis')).toBeInTheDocument()
    const typeSelect = screen.getByDisplayValue('Bar') as HTMLSelectElement
    expect(typeSelect).toBeInTheDocument()
    const yAxisSelect = screen.getByDisplayValue(/Y1.*right/) as HTMLSelectElement
    expect(yAxisSelect).toBeInTheDocument()
  })

  it('pre-fills value format input from series config', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [{ id: 10, tracker_id: 1, name: 's1', data_type: 'float', config: '{"value_format":"%.2f"}' }],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    const input = screen.getByPlaceholderText('e.g. %.1f') as HTMLInputElement
    expect(input.value).toBe('%.2f')
  })

  it('sends PATCH request when saving value format', async () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [{ id: 10, tracker_id: 1, name: 's1', data_type: 'float', config: '{}' }],
    })
    globalThis.fetch = vi.fn().mockImplementation(async (url: string, opts?: RequestInit) => {
      if (opts?.method === 'PATCH') {
        return {
          ok: true,
          json: () => Promise.resolve({ id: 10, tracker_id: 1, name: 's1', data_type: 'float', config: '{"value_format":"%.1f"}' }),
        } as Response
      }
      return { ok: true, json: () => Promise.resolve({ values: [] }) } as Response
    })

    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    const input = screen.getByPlaceholderText('e.g. %.1f') as HTMLInputElement
    fireEvent.change(input, { target: { value: '%.1f' } })

    const saveBtn = screen.getByText('Save')
    saveBtn.click()

    await vi.waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/trackers/1/series/10', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ config: '{"value_format":"%.1f"}' }),
      })
    })
  })

  it('clears value format when input is empty and saved', async () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [{ id: 10, tracker_id: 1, name: 's1', data_type: 'float', config: '{"value_format":"%.2f"}' }],
    })
    globalThis.fetch = vi.fn().mockImplementation(async (url: string, opts?: RequestInit) => {
      if (opts?.method === 'PATCH') {
        return {
          ok: true,
          json: () => Promise.resolve({ id: 10, tracker_id: 1, name: 's1', data_type: 'float', config: '{}' }),
        } as Response
      }
      return { ok: true, json: () => Promise.resolve({ values: [] }) } as Response
    })

    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    const input = screen.getByPlaceholderText('e.g. %.1f') as HTMLInputElement
    fireEvent.change(input, { target: { value: '' } })

    const saveBtn = screen.getByText('Save')
    saveBtn.click()

    await vi.waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/trackers/1/series/10', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ config: '{}' }),
      })
    })
  })

  it('reassigns series to Y0 when removed axis was used', async () => {
    const chartConfig = '{"y_axes":[{"id":0,"position":"left","label":"Count"},{"id":1,"position":"right","label":"Rate"}]}'
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: {
        id: 1, name: 'test', visibility: 'private', type: 'tracker',
        chart_config: chartConfig,
        role: 'owner', liked: false,
      },
      series: [
        { id: 1, tracker_id: 1, name: 's1', data_type: 'float', config: '{}' },
        { id: 2, tracker_id: 1, name: 's2', data_type: 'float', config: '{"y_axis_index":1}' },
      ],
    })

    globalThis.fetch = vi.fn().mockImplementation(async (url: string, opts?: RequestInit) => {
      if (opts?.method === 'PATCH') {
        if (url === '/api/trackers/1') {
          const body = JSON.parse(opts.body as string)
          return {
            ok: true,
            json: () => Promise.resolve({ id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: body.chart_config, role: 'owner', liked: false }),
          } as Response
        }
        return {
          ok: true,
          json: () => Promise.resolve({ id: 2, tracker_id: 1, name: 's2', data_type: 'float', config: opts.body }),
        } as Response
      }
      return { ok: true, json: () => Promise.resolve({ values: [] }) } as Response
    })

    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)

    const removeButtons = screen.getAllByText('Remove')
    fireEvent.click(removeButtons[1])

    await vi.waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/trackers/1/series/2', expect.objectContaining({
        method: 'PATCH',
        body: JSON.stringify({ config: '{"y_axis_index":0}' }),
      }))
    })

    await vi.waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/trackers/1', expect.objectContaining({
        method: 'PATCH',
        body: JSON.stringify({ chart_config: '{"y_axes":[{"id":0,"position":"left","label":"Count"}]}' }),
      }))
    })
  })

  it('disables Remove button when only one axis is active', () => {
    const chartConfig = '{"y_axes":[{"id":0,"position":"left","label":"Count"}]}'
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: {
        id: 1, name: 'test', visibility: 'private', type: 'tracker',
        chart_config: chartConfig,
        role: 'owner', liked: false,
      },
      series: [],
    })
    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)
    const removeButtons = screen.getAllByText('Remove')
    expect(removeButtons).toHaveLength(1)
    expect(removeButtons[0]).toBeDisabled()
  })

  it('shows Add button for removed axis and clicking it adds axis back', async () => {
    const chartConfig = '{"y_axes":[{"id":0,"position":"left","label":"Count"},{"id":1,"position":"right","label":"Rate"}]}'
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: {
        id: 1, name: 'test', visibility: 'private', type: 'tracker',
        chart_config: chartConfig,
        role: 'owner', liked: false,
      },
      series: [],
    })

    globalThis.fetch = vi.fn().mockImplementation(async (url: string, opts?: RequestInit) => {
      if (opts?.method === 'PATCH' && url === '/api/trackers/1') {
        const body = JSON.parse(opts.body as string)
        return {
          ok: true,
          json: () => Promise.resolve({ id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: body.chart_config, role: 'owner', liked: false }),
        } as Response
      }
      return { ok: true, json: () => Promise.resolve({ values: [] }) } as Response
    })

    render(<MemoryRouter><TrackerDetailEdit /></MemoryRouter>)

    // Both axes active: two Remove buttons, one Add button (in Add Value form)
    expect(screen.getAllByText('Remove')).toHaveLength(2)
    const addButtonsBefore = screen.getAllByText('Add')
    expect(addButtonsBefore).toHaveLength(1) // Only the "Add Value" button

    // Click Remove on Right axis (second Remove button)
    const removeButtons = screen.getAllByText('Remove')
    fireEvent.click(removeButtons[1])

    // Now: Left active (Remove), Right inactive (Add for axis + Add Value)
    await vi.waitFor(() => {
      expect(screen.getAllByText('Remove')).toHaveLength(1)
      expect(screen.getAllByText('Add')).toHaveLength(2) // Y-axis Add + Add Value
    })

    // Click the first Add button (Y-axis Add)
    const addButtonsAfter = screen.getAllByText('Add')
    fireEvent.click(addButtonsAfter[0])

    await vi.waitFor(() => {
      expect(screen.getAllByText('Remove')).toHaveLength(2)
    })
  })
})
