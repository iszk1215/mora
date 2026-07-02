import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { PasswordLoginForm } from './auth'

const mockNavigate = vi.fn()
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLoaderData: vi.fn(),
  }
})

function setInput(label: string, value: string) {
  const input = screen.getByLabelText(label) as HTMLInputElement
  fireEvent.change(input, { target: { value } })
}

describe('PasswordLoginForm', () => {
  beforeEach(() => {
    mockNavigate.mockReset()
    globalThis.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders login form by default', () => {
    render(<MemoryRouter><PasswordLoginForm /></MemoryRouter>)
    expect(screen.getByRole('heading', { name: 'Sign in with Password' })).toBeInTheDocument()
    expect(screen.getByLabelText('Username')).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign In' })).toBeInTheDocument()
  })

  it('shows error when CSRF token fetch fails', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValueOnce({
      ok: false,
      status: 500,
    } as Response)

    render(<MemoryRouter><PasswordLoginForm /></MemoryRouter>)

    setInput('Username', 'testuser')
    setInput('Password', 'testpass')

    fireEvent.click(screen.getByRole('button', { name: 'Sign In' }))

    await vi.waitFor(() => {
      expect(screen.getByText('Failed to get CSRF token. Please refresh the page.')).toBeInTheDocument()
    })
  })

  it('navigates to home on successful login', async () => {
    vi.mocked(globalThis.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ csrf_token: 'testtoken123' }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
      } as Response)

    render(<MemoryRouter><PasswordLoginForm /></MemoryRouter>)

    setInput('Username', 'testuser')
    setInput('Password', 'testpass')

    fireEvent.click(screen.getByRole('button', { name: 'Sign In' }))

    await vi.waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/', { replace: true })
    })
  })

  it('shows error on login API failure', async () => {
    vi.mocked(globalThis.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ csrf_token: 'testtoken123' }),
      } as Response)
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ message: 'Invalid credentials' }),
      } as Response)

    render(<MemoryRouter><PasswordLoginForm /></MemoryRouter>)

    setInput('Username', 'testuser')
    setInput('Password', 'wrongpass')

    fireEvent.click(screen.getByRole('button', { name: 'Sign In' }))

    await vi.waitFor(() => {
      expect(screen.getByText('Invalid credentials')).toBeInTheDocument()
    })
  })

  it('shows generic error on login API failure without message', async () => {
    vi.mocked(globalThis.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ csrf_token: 'testtoken123' }),
      } as Response)
      .mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: () => Promise.reject(new Error('parse error')),
      } as Response)

    render(<MemoryRouter><PasswordLoginForm /></MemoryRouter>)

    setInput('Username', 'testuser')
    setInput('Password', 'testpass')

    fireEvent.click(screen.getByRole('button', { name: 'Sign In' }))

    await vi.waitFor(() => {
      expect(screen.getByText('Request failed')).toBeInTheDocument()
    })
  })

  it('disables button while submitting', async () => {
    vi.mocked(globalThis.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ csrf_token: 'testtoken123' }),
      } as Response)
      .mockImplementationOnce(() => new Promise(() => {}))

    render(<MemoryRouter><PasswordLoginForm /></MemoryRouter>)

    setInput('Username', 'testuser')
    setInput('Password', 'testpass')

    fireEvent.click(screen.getByRole('button', { name: 'Sign In' }))

    await vi.waitFor(() => {
      expect(screen.getByText('Signing in...')).toBeInTheDocument()
    })
  })

  it('sends correct POST request to login endpoint', async () => {
    vi.mocked(globalThis.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ csrf_token: 'testtoken123' }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
      } as Response)

    render(<MemoryRouter><PasswordLoginForm /></MemoryRouter>)

    setInput('Username', 'loginuser')
    setInput('Password', 'loginpass')

    fireEvent.click(screen.getByRole('button', { name: 'Sign In' }))

    await vi.waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': 'testtoken123',
        },
        body: JSON.stringify({ username: 'loginuser', password: 'loginpass' }),
      })
    })
  })

})
