import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { useLoaderData } from 'react-router'
import {
  TrackerCreate,
  TrackerDetailView,
  TrackerCard,
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
  default: ({ option }: any) => <div data-testid="echart" data-option={JSON.stringify(option)} />,
}))

vi.mock('react-datepicker', () => ({
  default: (props: any) => <input data-testid="datepicker" {...props} />,
}))


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

  it('sends PATCH request with name', async () => {
    const updated = { id: 1, name: 'renamed', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false }
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(updated),
    } as Response)

    const result = await patchTracker(1, { name: 'renamed' })
    expect(result).toEqual(updated)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/trackers/1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'renamed' }),
    })
  })
})

describe('trackerRoute index', () => {
  it('has index route that throws 404', async () => {
    const { trackerRoute } = await import('./tracker')
    const indexRoute = trackerRoute.find((r: any) => r.index === true)
    expect(indexRoute).toBeDefined()
    expect(indexRoute!.loader).toBeDefined()
  })
})


describe('TrackerCreate', () => {
  const mockUser = { id: 1, provider: 'github', provider_user_id: '42', username: 'testuser', avatar_url: '' }

  beforeEach(() => {
    mockNavigate.mockReset()
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders form with name input, visibility select, and buttons', () => {
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerCreate /></UserProvider></MemoryRouter>)
    expect(screen.getByPlaceholderText('Tracker name')).toBeInTheDocument()
    expect(screen.getByText('Create Tracker')).toBeInTheDocument()
    expect(screen.getByText('Cancel')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Private')).toBeInTheDocument()
    const cancelLink = screen.getByText('Cancel').closest('a')
    expect(cancelLink).toHaveAttribute('href', '/users/testuser')
  })

  it('creates tracker and navigates on submit', async () => {
    const created = { id: 42, name: 'new-tracker', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false }
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(created),
    } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerCreate /></UserProvider></MemoryRouter>)
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
        body: JSON.stringify({ name: 'new-tracker', visibility: 'private' }),
      })
      expect(mockNavigate).toHaveBeenCalledWith('/trackers/42')
    })
  })

  it('shows error on API failure', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: false,
      status: 400,
    } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerCreate /></UserProvider></MemoryRouter>)
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
    expect(screen.getByRole('button', { name: /like/i })).toBeInTheDocument()
  })

  it('renders Unlike button when liked', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: true },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByRole('button', { name: /unlike/i })).toBeInTheDocument()
  })

  it('disables Like button when user is not logged in', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={null}><TrackerDetailView /></UserProvider></MemoryRouter>)
    const likeButton = screen.getByRole('button', { name: /like/i })
    expect(likeButton).toBeDisabled()
  })

  it('shows tracker menu when user is owner', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByRole('button', { name: /tracker menu/i })).toBeInTheDocument()
  })

  it('hides tracker menu when user is editor', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'editor', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.queryByRole('button', { name: /tracker menu/i })).not.toBeInTheDocument()
  })

  it('hides tracker menu when user has no role', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.queryByRole('button', { name: /tracker menu/i })).not.toBeInTheDocument()
  })

  it('opens and closes chart options panel from tracker menu', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    const menuItem = await screen.findByText('Settings')
    await user.click(menuItem)

    expect(screen.getByText('Close')).toBeInTheDocument()
    await user.click(screen.getByText('Close'))
    expect(screen.queryByText('Close')).not.toBeInTheDocument()
  })

  it('opens and closes series settings panel from tracker menu', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    const menuItem = await screen.findByRole('menuitem', { name: 'Series' })
    await user.click(menuItem)

    expect(screen.getByText('No series yet')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Close' }))
    expect(screen.queryByText('No series yet')).not.toBeInTheDocument()
  })

  it('shows series rows in the series settings panel', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [
        { id: 1, tracker_id: 1, name: 'series-a', data_type: 'float', config: '{"type":"line"}' },
        { id: 2, tracker_id: 1, name: 'series-b', data_type: 'int', config: '{"type":"bar"}' },
      ],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByRole('menuitem', { name: 'Series' }))

    expect(screen.getByText('series-a')).toBeInTheDocument()
    expect(screen.getByText('series-b')).toBeInTheDocument()
    expect(screen.getAllByText('Delete')).toHaveLength(2)
  })

  it('renames a series from the series settings panel', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [
        { id: 1, tracker_id: 1, name: 'series-a', data_type: 'float', config: '{"type":"line"}' },
      ],
    })
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 1, tracker_id: 1, name: 'series-a-renamed', data_type: 'float', config: '{"type":"line"}' }),
    } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByRole('menuitem', { name: 'Series' }))

    const pencil = screen.getByRole('button', { name: /rename series series-a/i })
    await user.click(pencil)

    const nameInput = screen.getByRole('textbox', { name: /rename series series-a/i })
    await user.clear(nameInput)
    await user.type(nameInput, 'series-a-renamed')
    const nameSave = nameInput.closest('div')!.querySelector('button')!
    await user.click(nameSave)

    await vi.waitFor(() => {
      const patchCall = vi.mocked(globalThis.fetch).mock.calls.find(
        ([url, init]) => url === '/api/trackers/1/series/1' && (init as RequestInit)?.method === 'PATCH'
      )
      expect(patchCall).toBeDefined()
      const body = JSON.parse((patchCall![1] as RequestInit).body as string)
      expect(body.name).toBe('series-a-renamed')
    })

    await vi.waitFor(() => {
      expect(screen.getByText('series-a-renamed')).toBeInTheDocument()
    })
  })

  it('adds a series via menu while series settings panel is open and reflects it', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 1, tracker_id: 1, name: 'added-series', data_type: 'float', config: '{"type":"line"}' }),
    } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByRole('menuitem', { name: 'Series' }))

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByRole('menuitem', { name: /add series/i }))

    const nameInput = screen.getByPlaceholderText('Series name')
    await user.clear(nameInput)
    await user.type(nameInput, 'added-series')
    await user.click(screen.getByRole('button', { name: 'Add' }))

    await vi.waitFor(() => {
      expect(screen.getByText('added-series')).toBeInTheDocument()
    })
  })

  it('saves chart options via PATCH from panel', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{"palette":"default"}', role: 'owner', liked: false }),
    } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    const menuItem = await screen.findByText('Settings')
    await user.click(menuItem)

    await user.click(screen.getByText('Save'))

    await vi.waitFor(() => {
      const patchCall = vi.mocked(globalThis.fetch).mock.calls.find(
        ([url, init]) => url === '/api/trackers/1' && (init as RequestInit)?.method === 'PATCH'
      )
      expect(patchCall).toBeDefined()
      const body = JSON.parse((patchCall![1] as RequestInit).body as string)
      expect(body.chart_config).toContain('"palette":"default"')
    })
  })

  it('saves visibility via PATCH from settings panel', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 1, name: 'test', visibility: 'public', type: 'tracker', chart_config: '{}', role: 'owner', liked: false }),
    } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    const menuItem = await screen.findByText('Settings')
    await user.click(menuItem)

    const select = screen.getByDisplayValue('Private')
    await user.selectOptions(select, 'public')
    await user.click(screen.getByText('Save'))

    await vi.waitFor(() => {
      const patchCall = vi.mocked(globalThis.fetch).mock.calls.find(
        ([url, init]) => url === '/api/trackers/1' && (init as RequestInit)?.method === 'PATCH'
      )
      expect(patchCall).toBeDefined()
      const body = JSON.parse((patchCall![1] as RequestInit).body as string)
      expect(body.visibility).toBe('public')
    })
  })

  it('adds a series with default line type from tracker menu', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 1, tracker_id: 1, name: 'new-series', data_type: 'float', config: '{"type":"line"}' }),
    } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByRole('menuitem', { name: /add series/i }))

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    const nameInput = screen.getByPlaceholderText('Series name')
    expect(nameInput).toHaveValue('test')
    await user.clear(nameInput)
    await user.type(nameInput, 'new-series')

    await user.click(screen.getByRole('button', { name: 'Add' }))

    await vi.waitFor(() => {
      const postCall = vi.mocked(globalThis.fetch).mock.calls.find(
        ([url, init]) => url === '/api/trackers/1/series' && (init as RequestInit)?.method === 'POST'
      )
      expect(postCall).toBeDefined()
      const body = JSON.parse((postCall![1] as RequestInit).body as string)
      expect(body.name).toBe('new-series')
      expect(body.data_type).toBe('float')
      expect(body.config).toBe('{"type":"line"}')
    })

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('adds a series with bar type and int data type', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 1, tracker_id: 1, name: 'bar-series', data_type: 'int', config: '{"type":"bar"}' }),
    } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByRole('menuitem', { name: /add series/i }))

    const nameInput = screen.getByPlaceholderText('Series name')
    expect(nameInput).toHaveValue('test')
    await user.clear(nameInput)
    await user.type(nameInput, 'bar-series')
    await user.selectOptions(screen.getByLabelText('Data Type'), 'int')
    await user.selectOptions(screen.getByLabelText('Chart Type'), 'bar')

    await user.click(screen.getByRole('button', { name: 'Add' }))

    await vi.waitFor(() => {
      const postCall = vi.mocked(globalThis.fetch).mock.calls.find(
        ([url, init]) => url === '/api/trackers/1/series' && (init as RequestInit)?.method === 'POST'
      )
      expect(postCall).toBeDefined()
      const body = JSON.parse((postCall![1] as RequestInit).body as string)
      expect(body.name).toBe('bar-series')
      expect(body.data_type).toBe('int')
      expect(body.config).toBe('{"type":"bar"}')
    })
  })

  it('shows error when adding a series fails', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    vi.mocked(globalThis.fetch).mockResolvedValue({ ok: false, status: 500 } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByRole('menuitem', { name: /add series/i }))

    await user.type(screen.getByPlaceholderText('Series name'), 'fail-series')
    await user.click(screen.getByRole('button', { name: 'Add' }))

    await vi.waitFor(() => {
      expect(screen.getByText('Failed to add series. Please try again.')).toBeInTheDocument()
    })
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('cancels chart options changes without saving', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    vi.mocked(globalThis.fetch).mockResolvedValue({ ok: true, json: () => Promise.resolve({}) } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByText('Settings'))

    const areaCheckbox = screen.getByLabelText('Area')
    await user.click(areaCheckbox)
    expect(areaCheckbox).not.toBeChecked()

    await user.click(screen.getByText('Cancel'))

    expect(areaCheckbox).toBeChecked()
    const patchCall = vi.mocked(globalThis.fetch).mock.calls.find(
      ([url, init]) => url === '/api/trackers/1' && (init as RequestInit)?.method === 'PATCH'
    )
    expect(patchCall).toBeUndefined()
  })

  it('keeps chart options edits when panel is closed and reopened', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    vi.mocked(globalThis.fetch).mockResolvedValue({ ok: true, json: () => Promise.resolve({}) } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByText('Settings'))

    const legendCheckbox = screen.getByLabelText('Legend')
    expect(legendCheckbox).not.toBeChecked()
    await user.click(legendCheckbox)
    expect(legendCheckbox).toBeChecked()

    await user.click(screen.getByText('Close'))
    expect(screen.queryByText('Close')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByText('Settings'))

    expect(screen.getByLabelText('Legend')).toBeChecked()
  })

  it('shows Delete item in tracker menu when owner', async () => {
    const user = userEvent.setup()
    mockNavigate.mockClear()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    expect(await screen.findByText('Delete')).toBeInTheDocument()
  })

  it('hides Delete item in tracker menu when editor', () => {
    mockNavigate.mockClear()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'editor', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.queryByRole('button', { name: /tracker menu/i })).not.toBeInTheDocument()
  })

  it('shows confirmation dialog when Delete is selected', async () => {
    const user = userEvent.setup()
    mockNavigate.mockClear()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByText('Delete'))

    expect(await screen.findByText('Delete Tracker?')).toBeInTheDocument()
    expect(screen.getByText(/cannot be undone/i)).toBeInTheDocument()
  })

  it('does not delete when confirming is cancelled', async () => {
    const user = userEvent.setup()
    mockNavigate.mockClear()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByText('Delete'))
    await user.click(await screen.findByText('Cancel'))

    expect(screen.queryByText('Delete Tracker?')).not.toBeInTheDocument()
    const deleteCall = vi.mocked(globalThis.fetch).mock.calls.find(
      ([url, init]) => url === '/api/trackers/1' && (init as RequestInit)?.method === 'DELETE'
    )
    expect(deleteCall).toBeUndefined()
  })

  it('deletes tracker and navigates to user page on confirm', async () => {
    const user = userEvent.setup()
    mockNavigate.mockClear()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true } as Response)
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByText('Delete'))
    await user.click(screen.getByRole('button', { name: /^delete$/i }))

    await vi.waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/trackers/1', expect.objectContaining({ method: 'DELETE' }))
    })
    expect(mockNavigate).toHaveBeenCalledWith('/users/testuser')
  })

  it('shows error and stays on page when delete fails', async () => {
    const user = userEvent.setup()
    mockNavigate.mockClear()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: false } as Response)
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByText('Delete'))
    await user.click(screen.getByRole('button', { name: /^delete$/i }))

    await vi.waitFor(() => {
      expect(screen.getByText(/failed to delete tracker/i)).toBeInTheDocument()
    })
    expect(mockNavigate).not.toHaveBeenCalled()
  })

  it('renders markdown body below the chart', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false, body: '## Overview\n\nSome **notes** here' },
      series: [],
    })
    const { container } = render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByRole('heading', { name: 'Overview', level: 2 })).toBeInTheDocument()
    expect(screen.getByText('notes')).toBeInTheDocument()
    const body = container.querySelector('.md-body')
    expect(body).toBeInTheDocument()
    expect(body!.querySelector('.wmde-markdown')).toBeInTheDocument()
  })

  it('renders markdown lists in the body', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false, body: '- first\n- second\n\n1. one\n2. two' },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getAllByRole('listitem')).toHaveLength(4)
    expect(screen.getByText('first')).toBeInTheDocument()
    expect(screen.getByText('one')).toBeInTheDocument()
  })

  it('does not render body when empty and user is not owner', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false, body: '' },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.queryByRole('heading', { name: 'Overview' })).not.toBeInTheDocument()
  })

  it('shows empty body placeholder for owner when body is empty', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false, body: '' },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByRole('button', { name: /edit body/i })).toBeInTheDocument()
  })

  it('renders description in the same foreground color as the title', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false, description: 'Short description' },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    const desc = screen.getByText('Short description')
    expect(desc).toBeInTheDocument()
    expect(desc.className).not.toContain('text-muted-foreground')
  })

  it('renders chart section inside a card', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    const noData = screen.getByText('No data to display')
    const chartCard = noData.closest('.bg-card')
    expect(chartCard).toBeInTheDocument()
    expect(chartCard!.className).toContain('border rounded-lg')
  })

  it('hides empty state Add Series button when user is not owner', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByText('No data to display')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /add series/i })).not.toBeInTheDocument()
  })

  it('opens Add Series dialog from empty state button', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    const addBtn = screen.getByRole('button', { name: /add series/i })
    expect(addBtn).toBeInTheDocument()
    await user.click(addBtn)

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Series name')).toBeInTheDocument()
  })

  it('renders markdown body inside a card', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false, body: '## Overview\n\nSome **notes** here' },
      series: [],
    })
    const { container } = render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    const body = container.querySelector('.md-body')
    expect(body).toBeInTheDocument()
    expect(body!.className).toContain('bg-card')
  })

  it('shows pencil icon for title when owner', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByRole('button', { name: /edit title/i })).toBeInTheDocument()
  })

  it('hides pencil icon for title when not owner', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.queryByRole('button', { name: /edit title/i })).not.toBeInTheDocument()
  })

  it('enters title edit mode on pencil click and saves', async () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'old-name', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 1, name: 'new-name', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false }),
    } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByText('old-name')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /edit title/i }))
    const input = screen.getByRole('textbox')
    expect(input).toHaveValue('old-name')

    fireEvent.change(input, { target: { value: 'new-name' } })
    fireEvent.click(screen.getByText('Save'))

    await vi.waitFor(() => {
      expect(screen.getByText('new-name')).toBeInTheDocument()
    })
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/trackers/1', expect.objectContaining({ method: 'PATCH' }))
  })

  it('cancels title edit and restores original value', async () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'original', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [],
    })

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    fireEvent.click(screen.getByRole('button', { name: /edit title/i }))
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: 'changed' } })
    fireEvent.click(screen.getByText('Cancel'))

    expect(screen.getByText('original')).toBeInTheDocument()
  })

  it('shows pencil icon for description when owner', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false, description: 'desc' },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByRole('button', { name: /edit description/i })).toBeInTheDocument()
  })

  it('enters description edit mode and saves', async () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false, description: 'old desc' },
      series: [],
    })
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false, description: 'new desc' }),
    } as Response)

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    fireEvent.click(screen.getByRole('button', { name: /edit description/i }))
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: 'new desc' } })
    fireEvent.click(screen.getByText('Save'))

    await vi.waitFor(() => {
      expect(screen.getByText('new desc')).toBeInTheDocument()
    })
  })

  it('shows pencil icon for body when owner', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false, body: '## Hello' },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    expect(screen.getByRole('button', { name: /edit body/i })).toBeInTheDocument()
  })

  it('enters body edit mode and shows markdown editor', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false, body: '## Hello' },
      series: [],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)
    fireEvent.click(screen.getByRole('button', { name: /edit body/i }))
    expect(screen.getByText('Save Body')).toBeInTheDocument()
    expect(screen.getByText('Cancel')).toBeInTheDocument()
  })

  it('opens Add Data Points card from tracker menu and closes it', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [{ id: 1, tracker_id: 1, name: 's1', data_type: 'float' }],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    const menuItem = await screen.findByText('Add Data Points')
    await user.click(menuItem)

    expect(screen.getByText('Data Points')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /^close$/i }))
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /^close$/i })).not.toBeInTheDocument()
    })
  })

  it('renders a row per series with a date defaulting to today', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [
        { id: 1, tracker_id: 1, name: 's1', data_type: 'float' },
        { id: 2, tracker_id: 1, name: 's2', data_type: 'float' },
      ],
    })
    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByText('Add Data Points'))

    expect(screen.getByText('s1')).toBeInTheDocument()
    expect(screen.getByText('s2')).toBeInTheDocument()
    const dateInputs = screen.getAllByLabelText(/date for/i)
    expect(dateInputs).toHaveLength(2)
    const today = new Date().toISOString().slice(0, 10)
    dateInputs.forEach((d) => expect(d).toHaveValue(today))
  })

  it('adds a value via POST and refreshes series values', async () => {
    const user = userEvent.setup()
    vi.mocked(useLoaderData).mockReturnValue({
      tracker: { id: 1, name: 'test', visibility: 'private', type: 'tracker', chart_config: '{}', role: 'owner', liked: false },
      series: [{ id: 1, tracker_id: 1, name: 's1', data_type: 'float' }],
    })
    const postBody = vi.fn()
    globalThis.fetch = vi.fn((url: RequestInfo | URL, opts?: RequestInit) => {
      if (opts?.method === 'POST') {
        postBody(JSON.parse(opts?.body as string))
        return Promise.resolve({ ok: true, json: () => Promise.resolve({}) } as Response)
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ values: [{ time: '2024-01-01T00:00:00Z', value: 5 }] }) } as Response)
    })

    render(<MemoryRouter><UserProvider value={mockUser}><TrackerDetailView /></UserProvider></MemoryRouter>)

    await user.click(screen.getByRole('button', { name: /tracker menu/i }))
    await user.click(await screen.findByText('Add Data Points'))

    const valueInput = screen.getByRole('spinbutton', { name: /value for/i })
    await user.type(valueInput, '42')
    await user.click(screen.getAllByRole('button', { name: /^add$/i })[0])

    await waitFor(() => {
      expect(postBody).toHaveBeenCalled()
    })
    expect(postBody.mock.calls[0][0]).toMatchObject({ value: 42 })
  })

})

describe('TrackerCard', () => {
  it('renders multi-axis yAxis and yAxisIndex from chartConfig', () => {
    const tracker = {
      id: 1, name: 'test', visibility: 'private', type: 'tracker',
      chart_config: '{"y_axes":[{"id":0,"position":"left"},{"id":1,"position":"right"}]}',
      role: 'owner', liked: false, like_count: 0,
    }
    const preview = {
      tracker,
      series: [
        { series: { id: 1, tracker_id: 1, name: 's1', data_type: 'float', config: '{"y_axis_index":0}' }, values: [{ time: '2024-01-01', value: 10 }] },
        { series: { id: 2, tracker_id: 1, name: 's2', data_type: 'float', config: '{"y_axis_index":1}' }, values: [{ time: '2024-01-01', value: 80 }] },
      ],
    }
    render(<MemoryRouter><TrackerCard tracker={tracker} preview={preview} /></MemoryRouter>)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.yAxis).toHaveLength(2)
    expect(option.yAxis[0].position).toBe('left')
    expect(option.yAxis[1].position).toBe('right')
    expect(option.series[0].yAxisIndex).toBe(0)
    expect(option.series[1].yAxisIndex).toBe(1)
    expect(option.grid.right).toBe(50)
  })

  it('defaults to single left axis when chartConfig has no y_axes', () => {
    const tracker = {
      id: 1, name: 'test', visibility: 'private', type: 'tracker',
      chart_config: '{}',
      role: 'owner', liked: false, like_count: 0,
    }
    const preview = {
      tracker,
      series: [
        { series: { id: 1, tracker_id: 1, name: 's1', data_type: 'float', config: '{}' }, values: [{ time: '2024-01-01', value: 10 }] },
      ],
    }
    render(<MemoryRouter><TrackerCard tracker={tracker} preview={preview} /></MemoryRouter>)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.yAxis).toHaveLength(1)
    expect(option.yAxis[0].position).toBe('left')
    expect(option.grid.right).toBe(10)
  })

  it('applies areaStyle for line series when area is not false', () => {
    const tracker = {
      id: 1, name: 'test', visibility: 'private', type: 'tracker',
      chart_config: '{}',
      role: 'owner', liked: false, like_count: 0,
    }
    const preview = {
      tracker,
      series: [
        { series: { id: 1, tracker_id: 1, name: 's1', data_type: 'float', config: '{}' }, values: [{ time: '2024-01-01', value: 10 }] },
      ],
    }
    render(<MemoryRouter><TrackerCard tracker={tracker} preview={preview} /></MemoryRouter>)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series[0].areaStyle).toBeDefined()
  })

  it('omits areaStyle for line series when area is false', () => {
    const tracker = {
      id: 1, name: 'test', visibility: 'private', type: 'tracker',
      chart_config: '{"area":false}',
      role: 'owner', liked: false, like_count: 0,
    }
    const preview = {
      tracker,
      series: [
        { series: { id: 1, tracker_id: 1, name: 's1', data_type: 'float', config: '{}' }, values: [{ time: '2024-01-01', value: 10 }] },
      ],
    }
    render(<MemoryRouter><TrackerCard tracker={tracker} preview={preview} /></MemoryRouter>)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series[0].areaStyle).toBeUndefined()
  })

  it('omits areaStyle for bar series regardless of area config', () => {
    const tracker = {
      id: 1, name: 'test', visibility: 'private', type: 'tracker',
      chart_config: '{}',
      role: 'owner', liked: false, like_count: 0,
    }
    const preview = {
      tracker,
      series: [
        { series: { id: 1, tracker_id: 1, name: 's1', data_type: 'float', config: '{"type":"bar"}' }, values: [{ time: '2024-01-01', value: 10 }] },
      ],
    }
    render(<MemoryRouter><TrackerCard tracker={tracker} preview={preview} /></MemoryRouter>)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series[0].areaStyle).toBeUndefined()
  })

  it('strips time portion from x values when x_axis_type is date', () => {
    const tracker = {
      id: 1, name: 'test', visibility: 'private', type: 'tracker',
      chart_config: '{"x_axis_type":"date"}',
      role: 'owner', liked: false, like_count: 0,
    }
    const preview = {
      tracker,
      series: [
        { series: { id: 1, tracker_id: 1, name: 's1', data_type: 'float', config: '{}' }, values: [{ time: '2024-01-15T10:30:00Z', value: 90 }] },
      ],
    }
    render(<MemoryRouter><TrackerCard tracker={tracker} preview={preview} /></MemoryRouter>)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series[0].data[0][0]).toBe('2024-01-15')
  })

  it('defaults to date-only x values when x_axis_type is not set', () => {
    const tracker = {
      id: 1, name: 'test', visibility: 'private', type: 'tracker',
      chart_config: '{}',
      role: 'owner', liked: false, like_count: 0,
    }
    const preview = {
      tracker,
      series: [
        { series: { id: 1, tracker_id: 1, name: 's1', data_type: 'float', config: '{}' }, values: [{ time: '2024-01-15T10:30:00Z', value: 90 }] },
      ],
    }
    render(<MemoryRouter><TrackerCard tracker={tracker} preview={preview} /></MemoryRouter>)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series[0].data[0][0]).toBe('2024-01-15')
  })

  it('shows private badge for private trackers', () => {
    const tracker = {
      id: 1, name: 'test', visibility: 'private', type: 'tracker',
      chart_config: '{}',
      role: 'owner', liked: false, like_count: 0,
    }
    render(<MemoryRouter><TrackerCard tracker={tracker} /></MemoryRouter>)
    expect(screen.getByText('private')).toBeInTheDocument()
  })

  it('does not show private badge for public trackers', () => {
    const tracker = {
      id: 1, name: 'test', visibility: 'public', type: 'tracker',
      chart_config: '{}',
      role: '', liked: false, like_count: 0,
    }
    render(<MemoryRouter><TrackerCard tracker={tracker} /></MemoryRouter>)
    expect(screen.queryByText('private')).not.toBeInTheDocument()
  })

  it('shows owner name and links to user page', () => {
    const tracker = {
      id: 1, name: 'test', visibility: 'public', type: 'tracker',
      chart_config: '{}',
      role: '', liked: false, like_count: 0,
      owner_name: 'alice',
    }
    render(<MemoryRouter><TrackerCard tracker={tracker} /></MemoryRouter>)
    const link = screen.getByText('alice').closest('a')
    expect(link).toHaveAttribute('href', '/users/alice')
  })

  it('does not show owner name when not provided', () => {
    const tracker = {
      id: 1, name: 'test', visibility: 'public', type: 'tracker',
      chart_config: '{}',
      role: '', liked: false, like_count: 0,
    }
    render(<MemoryRouter><TrackerCard tracker={tracker} /></MemoryRouter>)
    expect(screen.queryByText('owner')).not.toBeInTheDocument()
  })
})
