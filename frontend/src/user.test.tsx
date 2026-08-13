import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { useLoaderData } from 'react-router'
import { UserPage, loadUserPage } from './user'

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return { ...actual, useLoaderData: vi.fn() }
})

vi.mock('echarts-for-react', () => ({
  default: ({ option }: any) => <div data-testid="echart" data-option={JSON.stringify(option)} />,
}))

const mockUser = { id: 2, username: 'alice', avatar_url: '' }

const mockTrackers = (names: string[]) => ({
  trackers: names.map((name, i) => ({
    id: i + 1,
    name,
    visibility: 'public',
    type: 'tracker',
    chart_config: '{}',
    role: '',
    liked: false,
    like_count: 0,
  })),
  total: names.length,
  page: 1,
  per_page: 12,
})

const mockPreview = () => ({ tracker: {}, series: [] })

describe('loadUserPage', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns user from successful API response', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockUser),
    } as Response)

    const params = { userName: 'alice' }
    const args = { params, request: {} as Request, url: new URL('http://localhost'), pattern: '/', context: {} }
    const result = await loadUserPage(args)
    expect(result).toEqual({ user: mockUser })
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/users/alice')
  })

  it('throws Response 404 on non-ok response', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({ ok: false, status: 404 } as Response)

    const params = { userName: 'nobody' }
    const args = { params, request: {} as Request, url: new URL('http://localhost'), pattern: '/', context: {} }
    await expect(loadUserPage(args)).rejects.toMatchObject({ status: 404 })
  })
})

describe('UserPage', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
    vi.mocked(useLoaderData).mockReturnValue({ user: mockUser })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders username header', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockTrackers([])),
    } as Response)
    render(
      <MemoryRouter initialEntries={['/users/alice']}>
        <UserPage />
      </MemoryRouter>
    )
    expect(screen.getByRole('heading', { name: 'alice' })).toBeInTheDocument()
    await screen.findByText('No trackers found.')
  })

  it('fetches and shows user trackers', async () => {
    vi.mocked(globalThis.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockTrackers(['tracker-a'])),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockPreview()),
      } as Response)
    render(
      <MemoryRouter initialEntries={['/users/alice']}>
        <UserPage />
      </MemoryRouter>
    )
    expect(await screen.findByText('tracker-a')).toBeInTheDocument()
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/users/alice/trackers?page=1&per_page=12')
  })

  it('shows No trackers found for a search with no results', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ trackers: [], total: 0, page: 1, per_page: 12 }),
    } as Response)
    render(
      <MemoryRouter initialEntries={['/users/alice?q=nope']}>
        <UserPage />
      </MemoryRouter>
    )
    expect(await screen.findByText('No trackers found.')).toBeInTheDocument()
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/users/alice/trackers?page=1&per_page=12&q=nope')
  })

  it('searches within the user trackers', async () => {
    vi.mocked(globalThis.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockTrackers([])),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockTrackers(['tracker-b'])),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockPreview()),
      } as Response)
    render(
      <MemoryRouter initialEntries={['/users/alice']}>
        <UserPage />
      </MemoryRouter>
    )
    await waitFor(() => {
      expect(screen.getByText('No trackers found.')).toBeInTheDocument()
    })
    const input = screen.getByPlaceholderText('Search trackers...')
    fireEvent.change(input, { target: { value: 'b' } })
    fireEvent.click(screen.getByText('Search'))
    expect(await screen.findByText('tracker-b')).toBeInTheDocument()
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/users/alice/trackers?page=1&per_page=12&q=b')
  })

  it('shows pagination controls and navigates pages', async () => {
    const page1 = { ...mockTrackers(['one']), total: 25, page: 1 }
    vi.mocked(globalThis.fetch)
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(page1) } as Response)
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(mockPreview()) } as Response)
    render(
      <MemoryRouter initialEntries={['/users/alice']}>
        <UserPage />
      </MemoryRouter>
    )
    expect(await screen.findByText('one')).toBeInTheDocument()
    expect(screen.getByText('Page 1 of 3')).toBeInTheDocument()

    const page2 = { ...mockTrackers(['two']), total: 25, page: 2 }
    vi.mocked(globalThis.fetch)
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(page2) } as Response)
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(mockPreview()) } as Response)
    fireEvent.click(screen.getByText('Next'))
    expect(await screen.findByText('two')).toBeInTheDocument()
    expect(screen.getByText('Page 2 of 3')).toBeInTheDocument()
  })
})
