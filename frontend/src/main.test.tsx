import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { useLoaderData, useRouteError, useMatches, isRouteErrorResponse } from 'react-router'
import { SCMList, Header, ErrorPage, Breadcrumbs, makeBredcrumbs, resetConfigCache } from './main'

vi.mock('react-dom/client', () => ({
  default: { createRoot: () => ({ render: vi.fn() }) },
}))

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return {
    ...actual,
    useLoaderData: vi.fn(),
    useRouteError: vi.fn(),
    useMatches: vi.fn(),
    isRouteErrorResponse: vi.fn(),
    ScrollRestoration: () => null,
  }
})

describe('makeBredcrumbs', () => {
  it('renders single crumb without separator', () => {
    render(<div>{makeBredcrumbs([{ label: 'Home' }])}</div>)
    expect(screen.getByText('Home')).toBeInTheDocument()
  })

  it('renders multiple crumbs with separators', () => {
    const { container } = render(
      <MemoryRouter><div>{makeBredcrumbs([{ label: 'Home', link: '/' }, { label: 'Page' }])}</div></MemoryRouter>
    )
    expect(screen.getByText('Home')).toBeInTheDocument()
    expect(screen.getByText('Page')).toBeInTheDocument()
    expect(container.querySelector('[data-slot="breadcrumb-separator"]')).toBeInTheDocument()
  })

  it('renders link for non-last crumb when link is set', () => {
    render(
      <MemoryRouter>
        <div>{makeBredcrumbs([{ label: 'Home', link: '/' }, { label: 'Page' }])}</div>
      </MemoryRouter>
    )
    const link = screen.getByText('Home').closest('a')
    expect(link).not.toBeNull()
    expect(link).toHaveAttribute('href', '/')
  })

  it('renders plain text for last crumb even when link is set', () => {
    render(<div>{makeBredcrumbs([{ label: 'Home', link: '/' }])}</div>)
    const el = screen.getByText('Home')
    expect(el.closest('a')).toBeNull()
  })
})

describe('Header', () => {
  beforeEach(() => {
    vi.spyOn(globalThis, 'fetch')
    resetConfigCache()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  function mockConfig() {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ site_name: 'Mora' }), { headers: { 'Content-Type': 'application/json' } })
    )
  }

  it('renders Login link for anonymous user', async () => {
    mockConfig()
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(null, { status: 204 })
    )
    render(<MemoryRouter><Header /></MemoryRouter>)
    expect(screen.getByText('Mora')).toBeInTheDocument()
    const login = await screen.findByText('Login')
    expect(login).toBeInTheDocument()
    expect(screen.queryByText('Login/Logout')).not.toBeInTheDocument()
  })

  it('renders avatar with Open menu tooltip for logged-in user', async () => {
    const user = { id: 1, provider: 'github', provider_user_id: '42', username: 'testuser', avatar_url: 'https://example.com/avatar.jpg' }
    mockConfig()
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(user), { status: 200, headers: { 'Content-Type': 'application/json' } })
    )
    render(<MemoryRouter><Header /></MemoryRouter>)
    const img = await screen.findByRole('img')
    expect(img).toHaveAttribute('src', 'https://example.com/avatar.jpg')
    expect(img).toHaveAttribute('title', 'Open menu')
    expect(screen.queryByText('testuser')).not.toBeInTheDocument()
    expect(screen.queryByText('Login')).not.toBeInTheDocument()
  })

  it('opens menu with My Trackers link on avatar click', async () => {
    const user = { id: 1, provider: 'github', provider_user_id: '42', username: 'testuser', avatar_url: 'https://example.com/avatar.jpg' }
    mockConfig()
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(user), { status: 200, headers: { 'Content-Type': 'application/json' } })
    )
    render(<MemoryRouter><Header /></MemoryRouter>)
    const img = await screen.findByRole('img')
    expect(screen.queryByText('My Trackers')).not.toBeInTheDocument()
    await img.click()
    const link = screen.getByText('My Trackers')
    expect(link).toBeInTheDocument()
    expect(link.closest('a')).toHaveAttribute('href', '/tracker')
  })

  it('closes menu when clicking My Trackers', async () => {
    const user = { id: 1, provider: 'github', provider_user_id: '42', username: 'testuser', avatar_url: 'https://example.com/avatar.jpg' }
    mockConfig()
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(user), { status: 200, headers: { 'Content-Type': 'application/json' } })
    )
    render(<MemoryRouter><Header /></MemoryRouter>)
    const img = await screen.findByRole('img')
    await img.click()
    expect(screen.getByText('My Trackers')).toBeInTheDocument()
    const link = screen.getByText('My Trackers')
    await link.click()
    expect(screen.queryByText('My Trackers')).not.toBeInTheDocument()
  })

  it('renders nothing when avatar_url is empty', async () => {
    const user = { id: 1, provider: 'github', provider_user_id: '42', username: 'testuser', avatar_url: '' }
    mockConfig()
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(user), { status: 200, headers: { 'Content-Type': 'application/json' } })
    )
    render(<MemoryRouter><Header /></MemoryRouter>)
    await waitFor(() => {
      expect(screen.queryByRole('img')).not.toBeInTheDocument()
    })
    expect(screen.queryByText('testuser')).not.toBeInTheDocument()
    expect(screen.queryByText('Login')).not.toBeInTheDocument()
  })
})

describe('Header z-index', () => {
  it('has z-index class to stay above page content', () => {
    const { container } = render(<MemoryRouter><Header /></MemoryRouter>)
    const header = container.querySelector('header')
    expect(header).not.toBeNull()
    expect(header!.className).toMatch(/\bz-\d+\b/)
  })
})

describe('ErrorPage', () => {
  beforeEach(() => {
    vi.mocked(useRouteError).mockReset()
    vi.mocked(isRouteErrorResponse).mockReset()
  })

  it('renders generic error message for non-route errors', () => {
    vi.mocked(useRouteError).mockReturnValue(new Error('test error'))
    vi.mocked(isRouteErrorResponse).mockReturnValue(false)
    render(<MemoryRouter><ErrorPage /></MemoryRouter>)
    expect(screen.getAllByText('Error')).toHaveLength(2)
    expect(screen.getByText('Sorry, unexpected error has happend. Back to the top page.')).toBeInTheDocument()
  })

  it('renders route error status text', () => {
    vi.mocked(useRouteError).mockReturnValue({ statusText: 'Not Found' })
    vi.mocked(isRouteErrorResponse).mockReturnValue(true)
    render(<MemoryRouter><ErrorPage /></MemoryRouter>)
    expect(screen.getByText('Not Found')).toBeInTheDocument()
  })
})

describe('SCMList', () => {
  beforeEach(() => {
    vi.mocked(useLoaderData).mockReset()
  })

  it('renders Login heading', () => {
    vi.mocked(useLoaderData).mockReturnValue([])
    render(<MemoryRouter><SCMList /></MemoryRouter>)
    expect(screen.getByText('Login')).toBeInTheDocument()
  })

  it('renders Login with GitHub link for unauthenticated SCM', () => {
    vi.mocked(useLoaderData).mockReturnValue([
      { id: 1, url: 'https://github.com', name: 'github', logined: false },
    ])
    render(<MemoryRouter><SCMList /></MemoryRouter>)
    const loginEl = screen.getByText('Login with GitHub')
    expect(loginEl.closest('a')).toHaveAttribute('href', '/login/1')
  })

  it('renders Logout button for authenticated SCM', () => {
    vi.mocked(useLoaderData).mockReturnValue([
      { id: 1, url: 'https://github.com', name: 'github', logined: true },
    ])
    render(<MemoryRouter><SCMList /></MemoryRouter>)
    const logoutBtn = screen.getByText('Logout')
    expect(logoutBtn).toBeInTheDocument()
    expect(logoutBtn.closest('button')).not.toBeNull()
  })

  it('shows Connected for authenticated SCM', () => {
    vi.mocked(useLoaderData).mockReturnValue([
      { id: 1, url: 'https://github.com', name: 'github', logined: true },
    ])
    render(<MemoryRouter><SCMList /></MemoryRouter>)
    expect(screen.getByText('Connected')).toBeInTheDocument()
  })

  it('renders provider name in login buttons for each SCM entry', () => {
    vi.mocked(useLoaderData).mockReturnValue([
      { id: 1, url: 'https://github.com', name: 'github', logined: false },
      { id: 2, url: 'https://gitea.example.com', name: 'gitea', logined: false },
    ])
    render(<MemoryRouter><SCMList /></MemoryRouter>)
    expect(screen.getByText('Login with GitHub')).toBeInTheDocument()
    expect(screen.getByText('Login with Gitea')).toBeInTheDocument()
  })

  it('renders multiple SCM entries with different states', () => {
    vi.mocked(useLoaderData).mockReturnValue([
      { id: 1, url: 'https://github.com', name: 'github', logined: true },
      { id: 2, url: 'https://gitea.example.com', name: 'gitea', logined: false },
    ])
    render(<MemoryRouter><SCMList /></MemoryRouter>)
    expect(screen.getByText('Login with Gitea')).toBeInTheDocument()
    expect(screen.getByText('Logout')).toBeInTheDocument()
  })

  it('renders Login with Gitea for Gitea provider with brand style', () => {
    vi.mocked(useLoaderData).mockReturnValue([
      { id: 2, url: 'https://gitea.example.com', name: 'gitea', logined: false },
    ])
    render(<MemoryRouter><SCMList /></MemoryRouter>)
    const link = screen.getByText('Login with Gitea')
    expect(link.closest('a')).toHaveAttribute('href', '/login/2')
  })
})

describe('Breadcrumbs', () => {
  beforeEach(() => {
    vi.mocked(useMatches).mockReset()
  })

  it('renders breadcrumbs from route matches', () => {
    vi.mocked(useMatches).mockReturnValue([
      {
        id: '1', pathname: '/signup', params: {}, data: undefined, loaderData: undefined,
        handle: { crumb: () => ({ label: 'Sign Up', link: '/signup' }) },
      },
      {
        id: '2', pathname: '/settings/api-keys', params: {}, data: undefined, loaderData: undefined,
        handle: { crumb: () => ({ label: 'API Keys', link: '/settings/api-keys' }) },
      },
    ])
    render(<MemoryRouter><Breadcrumbs /></MemoryRouter>)
    expect(screen.getByText('Sign Up')).toBeInTheDocument()
    expect(screen.getByText('API Keys')).toBeInTheDocument()
  })

  it('filters out matches without crumb handle', () => {
    vi.mocked(useMatches).mockReturnValue([
      {
        id: '0', pathname: '/signup', params: {}, data: undefined, loaderData: undefined,
        handle: { crumb: () => ({ label: 'Sign Up', link: '/signup' }) },
      },
      {
        id: '1', pathname: '/no-crumb', params: {}, data: undefined, loaderData: undefined,
        handle: {},
      },
      {
        id: '2', pathname: '/settings/api-keys', params: {}, data: undefined, loaderData: undefined,
        handle: { crumb: () => ({ label: 'API Keys', link: '/settings/api-keys' }) },
      },
    ])
    render(<MemoryRouter><Breadcrumbs /></MemoryRouter>)
    expect(screen.getByText('Sign Up')).toBeInTheDocument()
    expect(screen.getByText('API Keys')).toBeInTheDocument()
  })

  it('filters out crumbs with undefined label', () => {
    vi.mocked(useMatches).mockReturnValue([
      {
        id: '0', pathname: '/signup', params: {}, data: undefined, loaderData: undefined,
        handle: { crumb: () => ({ label: 'Sign Up', link: '/signup' }) },
      },
      {
        id: '1', pathname: '/hidden', params: {}, data: undefined, loaderData: undefined,
        handle: { crumb: () => ({ label: undefined }) },
      },
    ])
    render(<MemoryRouter><Breadcrumbs /></MemoryRouter>)
    expect(screen.getByText('Sign Up')).toBeInTheDocument()
    expect(screen.queryByText('hidden')).toBeNull()
  })
})
