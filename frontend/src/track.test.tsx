import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { useLoaderData } from 'react-router'
import { TrackCreate, TrackListView, TrackDetailView, TrackDetailEdit, loadTrackList, loadTrackDetail, patchTrack } from './track'

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

describe('loadTrackList', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns tracks array from successful API response', async () => {
    const mockTracks = [
      { id: 1, name: 'track-a', visibility: 'private', role: 'owner', liked: false },
      { id: 2, name: 'track-b', visibility: 'public', role: '', liked: true },
    ]
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ tracks: mockTracks }),
    } as Response)

    const result = await loadTrackList()
    expect(result).toEqual(mockTracks)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/track')
  })

  it('throws on non-ok response', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: false,
      status: 500,
    } as Response)

    await expect(loadTrackList()).rejects.toBeDefined()
  })
})

describe('loadTrackDetail', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns track detail from API response', async () => {
    const mockResponse = {
      track: { id: 1, name: 'test', visibility: 'private', role: 'owner', liked: false },
      series: [{ id: 1, track_id: 1, name: 's1', data_type: 'float' }],
    }
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    } as Response)

    const params = { trackId: '1' }
    const args = { params, request: {} as Request, url: new URL('http://localhost'), pattern: '/', context: {} }
    const result = await loadTrackDetail(args)
    expect(result).toEqual(mockResponse)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/track/1/series')
  })

  it('throws on missing trackId', async () => {
    const params = {}
    const args = { params, request: {} as Request, url: new URL('http://localhost'), pattern: '/', context: {} }
    await expect(loadTrackDetail(args)).rejects.toThrow('trackId is required')
  })
})

describe('patchTrack', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('sends PATCH request with visibility and returns updated track', async () => {
    const updated = { id: 1, name: 'test', visibility: 'public', role: 'owner', liked: false }
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(updated),
    } as Response)

    const result = await patchTrack(1, 'public')
    expect(result).toEqual(updated)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/track/1', {
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

    await expect(patchTrack(1, 'public')).rejects.toBeDefined()
  })
})

describe('TrackListView', () => {
  beforeEach(() => {
    vi.mocked(useLoaderData).mockReset()
  })

  it('renders Tracks heading', () => {
    vi.mocked(useLoaderData).mockReturnValue([])
    render(<MemoryRouter><TrackListView /></MemoryRouter>)
    expect(screen.getByText('Tracks')).toBeInTheDocument()
  })

  it('shows Create Track link', () => {
    vi.mocked(useLoaderData).mockReturnValue([])
    render(<MemoryRouter><TrackListView /></MemoryRouter>)
    const link = screen.getByText('Create Track').closest('a')
    expect(link).toHaveAttribute('href', '/track/new')
  })

  it('shows My Tracks and Liked Tracks sections', () => {
    vi.mocked(useLoaderData).mockReturnValue([
      { id: 1, name: 'my-track', visibility: 'private', role: 'owner', liked: false },
      { id: 2, name: 'liked-track', visibility: 'public', role: '', liked: true },
    ])
    render(<MemoryRouter><TrackListView /></MemoryRouter>)
    expect(screen.getByText('My Tracks')).toBeInTheDocument()
    expect(screen.getByText('Liked Tracks')).toBeInTheDocument()
  })

  it('shows liked indicator on liked tracks', () => {
    vi.mocked(useLoaderData).mockReturnValue([
      { id: 1, name: 'liked-track', visibility: 'public', role: '', liked: true },
    ])
    render(<MemoryRouter><TrackListView /></MemoryRouter>)
    expect(screen.getByText('\u2665')).toBeInTheDocument()
  })
})

describe('TrackCreate', () => {
  beforeEach(() => {
    mockNavigate.mockReset()
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders form with name input, visibility select, and buttons', () => {
    render(<MemoryRouter><TrackCreate /></MemoryRouter>)
    expect(screen.getByPlaceholderText('Track name')).toBeInTheDocument()
    expect(screen.getByText('Create Track')).toBeInTheDocument()
    expect(screen.getByText('Cancel')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Private')).toBeInTheDocument()
    const cancelLink = screen.getByText('Cancel').closest('a')
    expect(cancelLink).toHaveAttribute('href', '/track')
  })

  it('creates track and navigates on submit', async () => {
    const created = { id: 42, name: 'new-track', visibility: 'private', role: 'owner', liked: false }
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(created),
    } as Response)

    render(<MemoryRouter><TrackCreate /></MemoryRouter>)
    const input = screen.getByPlaceholderText('Track name')
    const createBtn = screen.getByText('Create')

    input.focus()
    // Simulate typing
    input.setAttribute('value', 'new-track')
    input.dispatchEvent(new Event('change', { bubbles: true }))

    createBtn.click()

    await vi.waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/track', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: 'new-track', visibility: 'private' }),
      })
      expect(mockNavigate).toHaveBeenCalledWith('/track/42')
    })
  })

  it('shows error on API failure', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: false,
      status: 400,
    } as Response)

    render(<MemoryRouter><TrackCreate /></MemoryRouter>)
    const input = screen.getByPlaceholderText('Track name')
    const createBtn = screen.getByText('Create')

    input.setAttribute('value', 'fail-track')
    input.dispatchEvent(new Event('change', { bubbles: true }))

    createBtn.click()

    await vi.waitFor(() => {
      expect(screen.getByText('Failed to create track. Please try again.')).toBeInTheDocument()
    })
  })
})

describe('TrackDetailView', () => {
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

  it('renders track name', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      track: { id: 1, name: 'test-track', visibility: 'private', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackDetailView /></MemoryRouter>)
    expect(screen.getByText('test-track')).toBeInTheDocument()
  })

  it('renders Like button', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      track: { id: 1, name: 'test', visibility: 'private', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackDetailView /></MemoryRouter>)
    expect(screen.getByText('Like')).toBeInTheDocument()
  })

  it('renders Unlike button when liked', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      track: { id: 1, name: 'test', visibility: 'private', role: '', liked: true },
      series: [],
    })
    render(<MemoryRouter><TrackDetailView /></MemoryRouter>)
    expect(screen.getByText('Unlike')).toBeInTheDocument()
  })

  it('shows Edit button when user has role', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      track: { id: 1, name: 'test', visibility: 'private', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackDetailView /></MemoryRouter>)
    expect(screen.getByText('Edit')).toBeInTheDocument()
  })

  it('hides Edit button when user has no role', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      track: { id: 1, name: 'test', visibility: 'private', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackDetailView /></MemoryRouter>)
    expect(screen.queryByText('Edit')).not.toBeInTheDocument()
  })

  it('renders datepickers', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      track: { id: 1, name: 'test', visibility: 'private', role: '', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackDetailView /></MemoryRouter>)
    const datepickers = screen.getAllByTestId('datepicker')
    expect(datepickers.length).toBeGreaterThanOrEqual(2)
  })

  it('renders series table with name and data type', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      track: { id: 1, name: 'test', visibility: 'private', role: '', liked: false },
      series: [{ id: 1, track_id: 1, name: 'series-a', data_type: 'float' }],
    })
    render(<MemoryRouter><TrackDetailView /></MemoryRouter>)
    expect(screen.getByText('series-a')).toBeInTheDocument()
    expect(screen.getByText('float')).toBeInTheDocument()
  })
})

describe('TrackDetailEdit', () => {
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

  it('renders edit heading with track name', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      track: { id: 1, name: 'edit-track', visibility: 'private', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackDetailEdit /></MemoryRouter>)
    expect(screen.getByText(/edit-track \(Edit\)/)).toBeInTheDocument()
  })

  it('renders Add Series form', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      track: { id: 1, name: 'test', visibility: 'private', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackDetailEdit /></MemoryRouter>)
    expect(screen.getByPlaceholderText('Series name')).toBeInTheDocument()
    expect(screen.getByText('Add Series')).toBeInTheDocument()
  })

  it('renders Add Value form', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      track: { id: 1, name: 'test', visibility: 'private', role: 'owner', liked: false },
      series: [{ id: 1, track_id: 1, name: 's1', data_type: 'float' }],
    })
    render(<MemoryRouter><TrackDetailEdit /></MemoryRouter>)
    expect(screen.getByText('Add')).toBeInTheDocument()
  })

  it('back link goes to track detail', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      track: { id: 1, name: 'test', visibility: 'private', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackDetailEdit /></MemoryRouter>)
    const backLink = screen.getByText(/Back to Track/).closest('a')
    expect(backLink).toHaveAttribute('href', '/track/1')
  })

  it('renders visibility selector', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      track: { id: 1, name: 'test', visibility: 'unlisted', role: 'owner', liked: false },
      series: [],
    })
    render(<MemoryRouter><TrackDetailEdit /></MemoryRouter>)
    expect(screen.getByText('Visibility')).toBeInTheDocument()
    const select = screen.getByDisplayValue('Unlisted')
    expect(select).toBeInTheDocument()
  })
})
