import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
// Import the router pieces from the same module instance so React contexts
// match; importing RouterProvider from 'react-router/dom' would resolve to a
// separate build and break context identity in tests.
import { createMemoryRouter, RouterProvider } from 'react-router'
import { routes } from './main'

vi.mock('react-dom/client', () => ({
  default: { createRoot: () => ({ render: vi.fn() }) },
}))

// Real router behavior is exercised here, so only ScrollRestoration is
// stubbed out (it needs browser scroll APIs unavailable in jsdom).
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return {
    ...actual,
    ScrollRestoration: () => null,
  }
})

const jsonResponse = (body: unknown, status = 200): Response =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const installFetchMock = (): void => {
  vi.spyOn(globalThis, 'fetch').mockImplementation((input: RequestInfo | URL) => {
    const url =
      typeof input === 'string'
        ? input
        : input instanceof Request
          ? input.url
          : input.href
    if (url === '/api/me') return Promise.resolve(new Response(null, { status: 204 }))
    if (url === '/api/config') return Promise.resolve(jsonResponse({ site_name: 'Mora' }))
    return Promise.resolve(jsonResponse({ message: 'not found' }, 404))
  })
}

const renderRoutesAt = (path: string): void => {
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  render(<RouterProvider router={router} />)
}

describe('app router integration', () => {
  beforeEach(() => {
    installFetchMock()
  })

  it('renders NotFoundPage with exactly one header for unknown URLs', async () => {
    renderRoutesAt('/definitely/not/a/page')
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeInTheDocument()
    expect(await screen.findByText('404')).toBeInTheDocument()
    expect(document.querySelectorAll('header')).toHaveLength(1)
  })

  it('bubbles child loader errors to ErrorPage with exactly one header', async () => {
    renderRoutesAt('/users/ghost-user')
    expect(await screen.findByRole('heading', { name: 'Not Found' })).toBeInTheDocument()
    expect(await screen.findByText('404')).toBeInTheDocument()
    expect(document.querySelectorAll('header')).toHaveLength(1)
  })
})
