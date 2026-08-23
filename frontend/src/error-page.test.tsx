import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { useRouteError } from 'react-router'
import { ErrorPage, NotFoundPage, describeError } from './error-page'

vi.mock('react-dom/client', () => ({
  default: { createRoot: () => ({ render: vi.fn() }) },
}))

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return {
    ...actual,
    useRouteError: vi.fn(),
    ScrollRestoration: () => null,
  }
})

const routeError = (status: number, statusText: string) => ({
  status,
  statusText,
  data: null,
  internal: false,
})

describe('describeError', () => {
  it('uses status and statusText for route error responses', () => {
    const info = describeError(routeError(404, 'Not Found'))
    expect(info.status).toBe(404)
    expect(info.title).toBe('Not Found')
    expect(info.unexpected).toBe(false)
  })

  it('falls back to a default title when statusText is empty', () => {
    const info = describeError(routeError(403, ''))
    expect(info.status).toBe(403)
    expect(info.title).toBe('Forbidden')
  })

  it('normalizes thrown Response objects', () => {
    const info = describeError(new Response('Not Found', { status: 404 }))
    expect(info.status).toBe(404)
    expect(info.title).toBe('Not Found')
    expect(info.unexpected).toBe(false)
  })

  it('treats Error instances as unexpected errors', () => {
    const info = describeError(new Error('test error'))
    expect(info.status).toBeUndefined()
    expect(info.title).toBe('Something went wrong')
    expect(info.detail).toBe('test error')
    expect(info.unexpected).toBe(true)
  })

  it('treats unknown thrown values as unexpected errors', () => {
    const info = describeError('boom')
    expect(info.status).toBeUndefined()
    expect(info.title).toBe('Something went wrong')
    expect(info.unexpected).toBe(true)
  })
})

describe('ErrorPage', () => {
  beforeEach(() => {
    vi.spyOn(globalThis, 'fetch').mockImplementation(() =>
      Promise.resolve(new Response(JSON.stringify({ site_name: 'Mora' })))
    )
    vi.mocked(useRouteError).mockReset()
  })

  it('renders status number, title and top page link for 404', () => {
    vi.mocked(useRouteError).mockReturnValue(routeError(404, 'Not Found'))
    render(<MemoryRouter><ErrorPage /></MemoryRouter>)
    expect(screen.getByText('404')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Not Found' })).toBeInTheDocument()
    const link = screen.getByText('Back to top page').closest('a')
    expect(link).not.toBeNull()
    expect(link).toHaveAttribute('href', '/')
  })

  it('renders title derived from status for empty statusText', () => {
    vi.mocked(useRouteError).mockReturnValue(routeError(403, ''))
    render(<MemoryRouter><ErrorPage /></MemoryRouter>)
    expect(screen.getByText('403')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Forbidden' })).toBeInTheDocument()
  })

  it('renders thrown Response objects with their status', () => {
    vi.mocked(useRouteError).mockReturnValue(new Response('Not Found', { status: 500 }))
    render(<MemoryRouter><ErrorPage /></MemoryRouter>)
    expect(screen.getByText('500')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Internal Server Error' })).toBeInTheDocument()
  })

  it('renders unexpected errors with detail message', () => {
    vi.mocked(useRouteError).mockReturnValue(new Error('test error'))
    render(<MemoryRouter><ErrorPage /></MemoryRouter>)
    expect(screen.getByRole('heading', { name: 'Something went wrong' })).toBeInTheDocument()
    expect(screen.getByText('test error')).toBeInTheDocument()
    expect(screen.getByText('Back to top page')).toBeInTheDocument()
  })
})

describe('NotFoundPage', () => {
  beforeEach(() => {
    vi.spyOn(globalThis, 'fetch').mockImplementation(() =>
      Promise.resolve(new Response(JSON.stringify({ site_name: 'Mora' })))
    )
  })

  it('renders 404 content with link back to top', () => {
    render(<MemoryRouter><NotFoundPage /></MemoryRouter>)
    expect(screen.getByText('404')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Page not found' })).toBeInTheDocument()
    const link = screen.getByText('Back to top page').closest('a')
    expect(link).not.toBeNull()
    expect(link).toHaveAttribute('href', '/')
  })
})
