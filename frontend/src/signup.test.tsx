import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { useLoaderData } from 'react-router'
import { SignupPage, loadPendingSignup, sanitizeUsername } from './signup'

const mockNavigate = vi.fn()
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return { ...actual, useLoaderData: vi.fn(), useNavigate: () => mockNavigate }
})

const avatarUrl = 'https://example.com/avatar.png'

describe('loadPendingSignup', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns pending data from successful API response', async () => {
    const mockData = { provider: 'github', username: 'testuser', avatar_url: avatarUrl }
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockData),
    } as Response)

    const result = await loadPendingSignup()
    expect(result).toEqual(mockData)
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/signup/pending')
  })

  it('returns null on non-ok response', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue({
      ok: false,
      status: 401,
    } as Response)

    const result = await loadPendingSignup()
    expect(result).toBeNull()
  })

  it('throws on fetch error', async () => {
    vi.mocked(globalThis.fetch).mockRejectedValue(new Error('network error'))

    await expect(loadPendingSignup()).rejects.toThrow('network error')
  })
})

describe('sanitizeUsername', () => {
  it('lowercases the input', () => {
    expect(sanitizeUsername('Octocat')).toBe('octocat')
  })

  it('replaces invalid characters with a single hyphen', () => {
    expect(sanitizeUsername('山田 tarou')).toBe('tarou')
    expect(sanitizeUsername('John.Doe')).toBe('john-doe')
    expect(sanitizeUsername('a  b')).toBe('a-b')
  })

  it('trims leading and trailing separators', () => {
    expect(sanitizeUsername('-foo_')).toBe('foo')
  })

  it('falls back to user when empty', () => {
    expect(sanitizeUsername('!!!@@#')).toBe('user')
    expect(sanitizeUsername('')).toBe('user')
  })

  it('truncates to 32 characters', () => {
    const long = 'a'.repeat(40)
    expect(sanitizeUsername(long)).toBe('a'.repeat(32))
  })
})

describe('SignupPage', () => {
  beforeEach(() => {
    vi.mocked(useLoaderData).mockReset()
    mockNavigate.mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
    document.cookie = 'csrf_token=; max-age=0; path=/'
  })

  it('shows redirecting and navigates to /auth when pending is null', () => {
    vi.mocked(useLoaderData).mockReturnValue(null)
    render(<MemoryRouter><SignupPage /></MemoryRouter>)
    expect(screen.getByText('Redirecting...')).toBeInTheDocument()
    expect(mockNavigate).toHaveBeenCalledWith('/auth', { replace: true })
  })

  it('renders signup form with user info from pending data', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      provider: 'github',
      username: 'octocat',
      avatar_url: avatarUrl,
    })
    render(<MemoryRouter><SignupPage /></MemoryRouter>)
    expect(screen.getByRole('heading', { name: 'Create Account' })).toBeInTheDocument()
    expect(screen.getByText('octocat')).toBeInTheDocument()
    expect(screen.getByText('via github')).toBeInTheDocument()
    const img = document.querySelector('img') as HTMLImageElement
    expect(img.src).toBe(avatarUrl)
  })

  it('shows avatar when avatar_url is provided', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      provider: 'gitea',
      username: 'user',
      avatar_url: avatarUrl,
    })
    render(<MemoryRouter><SignupPage /></MemoryRouter>)
    const img = document.querySelector('img') as HTMLImageElement
    expect(img.src).toBe(avatarUrl)
    expect(img.className).toContain('rounded-full')
  })

  it('defaults the username input to the sanitized pending username', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      provider: 'github',
      username: 'Octocat',
      avatar_url: avatarUrl,
    })
    render(<MemoryRouter><SignupPage /></MemoryRouter>)
    const input = screen.getByLabelText('Username') as HTMLInputElement
    expect(input.value).toBe('octocat')
  })

  it('sanitizes the username while typing', () => {
    vi.mocked(useLoaderData).mockReturnValue({
      provider: 'github',
      username: 'octocat',
      avatar_url: '',
    })
    render(<MemoryRouter><SignupPage /></MemoryRouter>)
    const input = screen.getByLabelText('Username') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'New.User!' } })
    expect(input.value).toBe('new-user')
  })

  it('includes the username in the confirm request', async () => {
    document.cookie = 'csrf_token=abc123; path=/'
    vi.mocked(useLoaderData).mockReturnValue({
      provider: 'github',
      username: 'test',
      avatar_url: '',
    })
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true } as Response)

    render(<MemoryRouter><SignupPage /></MemoryRouter>)

    const input = screen.getByLabelText('Username') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'octocat' } })

    screen.getByRole('button', { name: 'Create Account' }).click()

    await vi.waitFor(() => {
      const [, init] = vi.mocked(globalThis.fetch).mock.calls[0]
      expect(init).toBeDefined()
      const form = init?.body as FormData
      expect(form.get('username')).toBe('octocat')
    })
  })

  it('shows a suggested username on 409 and applies it on click', async () => {
    document.cookie = 'csrf_token=abc123; path=/'
    vi.mocked(useLoaderData).mockReturnValue({
      provider: 'github',
      username: 'test',
      avatar_url: '',
    })
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      json: () => Promise.resolve({ message: 'username "test" is already taken', suggested_username: 'test-2' }),
    } as Response)

    render(<MemoryRouter><SignupPage /></MemoryRouter>)

    screen.getByRole('button', { name: 'Create Account' }).click()

    await vi.waitFor(() => {
      expect(screen.getByText('username "test" is already taken')).toBeInTheDocument()
    })

    screen.getByRole('button', { name: 'Use suggested: test-2' }).click()
    const input = screen.getByLabelText('Username') as HTMLInputElement
    await vi.waitFor(() => {
      expect(input.value).toBe('test-2')
    })
  })

  it('posts to cancel API and navigates to /auth on Cancel', async () => {
    document.cookie = 'csrf_token=abc123; path=/'
    vi.mocked(useLoaderData).mockReturnValue({
      provider: 'github',
      username: 'test',
      avatar_url: '',
    })
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, status: 204 } as Response)

    render(<MemoryRouter><SignupPage /></MemoryRouter>)

    screen.getByRole('button', { name: 'Cancel' }).click()

    await vi.waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/signup/cancel', {
        method: 'POST',
        body: expect.any(FormData),
      })
      expect(mockNavigate).toHaveBeenCalledWith('/auth', { replace: true })
    })
  })

  it('shows error when CSRF cookie is missing on confirm', async () => {
    vi.mocked(useLoaderData).mockReturnValue({
      provider: 'github',
      username: 'test',
      avatar_url: '',
    })
    render(<MemoryRouter><SignupPage /></MemoryRouter>)

    screen.getByRole('button', { name: 'Create Account' }).click()

    await vi.waitFor(() => {
      expect(screen.getByText('No CSRF token found. Please log in again.')).toBeInTheDocument()
    })
  })

  it('posts to confirm API and navigates on success', async () => {
    document.cookie = 'csrf_token=abc123; path=/'
    vi.mocked(useLoaderData).mockReturnValue({
      provider: 'github',
      username: 'test',
      avatar_url: '',
    })
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true } as Response)

    render(<MemoryRouter><SignupPage /></MemoryRouter>)

    screen.getByRole('button', { name: 'Create Account' }).click()

    await vi.waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/signup/confirm', {
        method: 'POST',
        body: expect.any(FormData),
      })
      expect(mockNavigate).toHaveBeenCalledWith('/', { replace: true })
    })
  })

  it('shows error on confirm API failure', async () => {
    document.cookie = 'csrf_token=abc123; path=/'
    vi.mocked(useLoaderData).mockReturnValue({
      provider: 'github',
      username: 'test',
      avatar_url: '',
    })
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ message: 'Something went wrong' }),
    } as Response)

    render(<MemoryRouter><SignupPage /></MemoryRouter>)

    screen.getByRole('button', { name: 'Create Account' }).click()

    await vi.waitFor(() => {
      expect(screen.getByText('Something went wrong')).toBeInTheDocument()
    })
  })

  it('shows generic error on confirm API failure without message', async () => {
    document.cookie = 'csrf_token=abc123; path=/'
    vi.mocked(useLoaderData).mockReturnValue({
      provider: 'github',
      username: 'test',
      avatar_url: '',
    })
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.reject(new Error('parse error')),
    } as Response)

    render(<MemoryRouter><SignupPage /></MemoryRouter>)

    screen.getByRole('button', { name: 'Create Account' }).click()

    await vi.waitFor(() => {
      expect(screen.getByText('Signup failed')).toBeInTheDocument()
    })
  })

  it('disables button while submitting', async () => {
    document.cookie = 'csrf_token=abc123; path=/'
    vi.mocked(useLoaderData).mockReturnValue({
      provider: 'github',
      username: 'test',
      avatar_url: '',
    })
    globalThis.fetch = vi.fn().mockImplementation(() => new Promise(() => {}))

    render(<MemoryRouter><SignupPage /></MemoryRouter>)

    screen.getByRole('button', { name: 'Create Account' }).click()

    await vi.waitFor(() => {
      expect(screen.getByText('Creating...')).toBeInTheDocument()
    })
  })
})
