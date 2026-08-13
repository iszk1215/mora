import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { useLoaderData, useRouteError, useMatches, isRouteErrorResponse } from 'react-router'
import { SCMList, Header, ErrorPage, Breadcrumbs, makeBredcrumbs, resetConfigCache, rootShouldRevalidate } from './main'
import { UserProvider } from './user-context'

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

  const mockUser = { id: 1, provider: 'github', provider_user_id: '42', username: 'testuser', avatar_url: 'https://example.com/avatar.jpg' }

  it('renders Login link for anonymous user', async () => {
    mockConfig()
    render(<MemoryRouter><UserProvider value={null}><Header /></UserProvider></MemoryRouter>)
    expect(screen.getByText('Mora')).toBeInTheDocument()
    const login = await screen.findByText('Login')
    expect(login).toBeInTheDocument()
    expect(screen.queryByText('Login/Logout')).not.toBeInTheDocument()
  })

  it('renders avatar with Open menu tooltip for logged-in user', async () => {
    mockConfig()
    render(<MemoryRouter><UserProvider value={mockUser}><Header /></UserProvider></MemoryRouter>)
    const img = await screen.findByRole('img')
    expect(img).toHaveAttribute('src', 'https://example.com/avatar.jpg')
    expect(img).toHaveAttribute('title', 'Open menu')
    expect(screen.queryByText('testuser')).not.toBeInTheDocument()
    expect(screen.queryByText('Login')).not.toBeInTheDocument()
  })

  it('opens menu with My Trackers link on avatar click', async () => {
    mockConfig()
    render(<MemoryRouter><UserProvider value={mockUser}><Header /></UserProvider></MemoryRouter>)
    const img = await screen.findByRole('img')
    expect(screen.queryByText('My Trackers')).not.toBeInTheDocument()
    await img.click()
    const link = screen.getByText('My Trackers')
    expect(link).toBeInTheDocument()
    expect(link.closest('a')).toHaveAttribute('href', '/trackers')
  })

  it('closes menu when clicking My Trackers', async () => {
    mockConfig()
    render(<MemoryRouter><UserProvider value={mockUser}><Header /></UserProvider></MemoryRouter>)
    const img = await screen.findByRole('img')
    await img.click()
    expect(screen.getByText('My Trackers')).toBeInTheDocument()
    const link = screen.getByText('My Trackers')
    await link.click()
    expect(screen.queryByText('My Trackers')).not.toBeInTheDocument()
  })

  it('renders nothing when avatar_url is empty', async () => {
    const userNoAvatar = { ...mockUser, avatar_url: '' }
    mockConfig()
    render(<MemoryRouter><UserProvider value={userNoAvatar}><Header /></UserProvider></MemoryRouter>)
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

  it('shows @Username > Tracker Name when no search location state', () => {
    vi.mocked(useMatches).mockReturnValue([
      {
        id: '0', pathname: '/', params: {}, data: undefined, loaderData: undefined,
        handle: {},
      },
      {
        id: 'routes/trackers/:trackerId', pathname: '/trackers/1', params: { trackerId: '1' }, 
        data: { tracker: { id: 1, name: 'My Tracker', owner_name: 'alice' } }, loaderData: undefined,
        handle: { crumb: (params: any, data: any) => ({ label: data?.tracker?.name ?? 'Tracker' }) },
      },
    ])
    render(
      <MemoryRouter initialEntries={[{ pathname: '/trackers/1' }]}>
        <Breadcrumbs />
      </MemoryRouter>
    )
    expect(screen.getByText('@alice')).toBeInTheDocument()
    expect(screen.getByText('My Tracker')).toBeInTheDocument()
    expect(screen.queryByText('Search Results')).toBeNull()
  })

  it('shows only Tracker Name when owner_name is missing and no search location state', () => {
    vi.mocked(useMatches).mockReturnValue([
      {
        id: '0', pathname: '/', params: {}, data: undefined, loaderData: undefined,
        handle: {},
      },
      {
        id: 'routes/trackers/:trackerId', pathname: '/trackers/1', params: { trackerId: '1' }, 
        data: { tracker: { id: 1, name: 'My Tracker' } }, loaderData: undefined,
        handle: { crumb: (params: any, data: any) => ({ label: data?.tracker?.name ?? 'Tracker' }) },
      },
    ])
    render(
      <MemoryRouter initialEntries={[{ pathname: '/trackers/1' }]}>
        <Breadcrumbs />
      </MemoryRouter>
    )
    expect(screen.getByText('My Tracker')).toBeInTheDocument()
    expect(screen.queryByText('@')).toBeNull()
    expect(screen.queryByText('Search Results')).toBeNull()
  })

  it('shows Search Results crumb when location state has fromSearch', () => {
    vi.mocked(useMatches).mockReturnValue([
      {
        id: '0', pathname: '/', params: {}, data: undefined, loaderData: undefined,
        handle: {},
      },
      {
        id: 'routes/trackers/:trackerId', pathname: '/trackers/1', params: { trackerId: '1' }, 
        data: { tracker: { id: 1, name: 'My Tracker', owner_name: 'alice' } }, loaderData: undefined,
        handle: { crumb: (params: any, data: any) => ({ label: data?.tracker?.name ?? 'Tracker' }) },
      },
    ])
    render(
      <MemoryRouter initialEntries={[{ pathname: '/trackers/1', state: { fromSearch: 'foo' } }]}>
        <Breadcrumbs />
      </MemoryRouter>
    )
    expect(screen.getByText('Search Results')).toBeInTheDocument()
    expect(screen.getByText('My Tracker')).toBeInTheDocument()
    expect(screen.queryByText('@alice')).toBeNull()
    const link = screen.getByText('Search Results').closest('a')
    expect(link).toHaveAttribute('href', '/?q=foo')
  })

  it('shows Search Results > Tracker Name > Edit for edit page with fromSearch', () => {
    vi.mocked(useMatches).mockReturnValue([
      {
        id: '0', pathname: '/', params: {}, data: undefined, loaderData: undefined,
        handle: {},
      },
      {
        id: 'routes/trackers/:trackerId', pathname: '/trackers/1', params: { trackerId: '1' }, 
        data: { tracker: { id: 1, name: 'My Tracker' } }, loaderData: undefined,
        handle: {},
      },
      {
        id: 'routes/trackers/:trackerId/edit', pathname: '/trackers/1/edit', params: { trackerId: '1' }, 
        data: { tracker: { id: 1, name: 'My Tracker' } }, loaderData: undefined,
        handle: { crumb: () => ({ label: 'Edit' }) },
      },
    ])
    render(
      <MemoryRouter initialEntries={[{ pathname: '/trackers/1/edit', state: { fromSearch: 'foo' } }]}>
        <Breadcrumbs />
      </MemoryRouter>
    )
    expect(screen.getByText('Search Results')).toBeInTheDocument()
    expect(screen.getByText('My Tracker')).toBeInTheDocument()
    expect(screen.getByText('Edit')).toBeInTheDocument()
    const searchLink = screen.getByText('Search Results').closest('a')
    expect(searchLink).toHaveAttribute('href', '/?q=foo')
    const trackerLink = screen.getByText('My Tracker').closest('a')
    expect(trackerLink).toHaveAttribute('href', '/trackers/1')
  })

  it('shows @Username > Tracker Name > Edit for edit page without fromSearch', () => {
    vi.mocked(useMatches).mockReturnValue([
      {
        id: '0', pathname: '/', params: {}, data: undefined, loaderData: undefined,
        handle: {},
      },
      {
        id: 'routes/trackers/:trackerId', pathname: '/trackers/1', params: { trackerId: '1' }, 
        data: { tracker: { id: 1, name: 'My Tracker', owner_name: 'alice' } }, loaderData: undefined,
        handle: {},
      },
      {
        id: 'routes/trackers/:trackerId/edit', pathname: '/trackers/1/edit', params: { trackerId: '1' }, 
        data: { tracker: { id: 1, name: 'My Tracker', owner_name: 'alice' } }, loaderData: undefined,
        handle: { crumb: () => ({ label: 'Edit' }) },
      },
    ])
    render(
      <MemoryRouter initialEntries={[{ pathname: '/trackers/1/edit' }]}>
        <Breadcrumbs />
      </MemoryRouter>
    )
    expect(screen.queryByText('Search Results')).toBeNull()
    expect(screen.getByText('@alice')).toBeInTheDocument()
    expect(screen.getByText('My Tracker')).toBeInTheDocument()
    expect(screen.getByText('Edit')).toBeInTheDocument()
    const trackerLink = screen.getByText('My Tracker').closest('a')
    expect(trackerLink).toHaveAttribute('href', '/trackers/1')
  })

  it('shows tracker name in coverage breadcrumb from data', () => {
    vi.mocked(useMatches).mockReturnValue([
      {
        id: '0', pathname: '/', params: {}, data: undefined, loaderData: undefined,
        handle: {},
      },
      {
        id: 'routes/coverages/:trackerId', pathname: '/coverages/3', params: { trackerId: '3' },
        data: { trackerName: 'My Coverage Tracker' }, loaderData: undefined,
        handle: { crumb: (params: any, data: any) => ({ label: data?.trackerName ?? `Coverage #${params.trackerId}`, link: `/coverages/${params.trackerId}` }) },
      },
    ])
    render(
      <MemoryRouter initialEntries={[{ pathname: '/coverages/3' }]}>
        <Breadcrumbs />
      </MemoryRouter>
    )
    expect(screen.getByText('My Coverage Tracker')).toBeInTheDocument()
    expect(screen.queryByText('Coverage #3')).toBeNull()
  })

  it('falls back to Coverage #id when data is missing', () => {
    vi.mocked(useMatches).mockReturnValue([
      {
        id: '0', pathname: '/', params: {}, data: undefined, loaderData: undefined,
        handle: {},
      },
      {
        id: 'routes/coverages/:trackerId', pathname: '/coverages/5', params: { trackerId: '5' },
        data: undefined, loaderData: undefined,
        handle: { crumb: (params: any, data: any) => ({ label: data?.trackerName ?? `Coverage #${params.trackerId}`, link: `/coverages/${params.trackerId}` }) },
      },
    ])
    render(
      <MemoryRouter initialEntries={[{ pathname: '/coverages/5' }]}>
        <Breadcrumbs />
      </MemoryRouter>
    )
    expect(screen.getByText('Coverage #5')).toBeInTheDocument()
  })

  it('Search Results link includes encoded query', () => {
    vi.mocked(useMatches).mockReturnValue([
      {
        id: '0', pathname: '/', params: {}, data: undefined, loaderData: undefined,
        handle: {},
      },
      {
        id: '0-4-2', pathname: '/trackers/1', params: { trackerId: '1' }, 
        data: { tracker: { id: 1, name: 'My Tracker' } }, loaderData: undefined,
        handle: { crumb: (params: any, data: any) => ({ label: data?.tracker?.name ?? 'Tracker' }) },
      },
    ])
    render(
      <MemoryRouter initialEntries={[{ pathname: '/trackers/1', state: { fromSearch: 'foo bar' } }]}>
        <Breadcrumbs />
      </MemoryRouter>
    )
    const link = screen.getByText('Search Results').closest('a')
    expect(link).toHaveAttribute('href', '/?q=foo%20bar')
  })

  // Tests using React Router v7 auto-generated numeric IDs (e.g. "0-4-2")
  describe('with React Router v7 numeric route IDs', () => {
    it('shows Search Results crumb on tracker detail page with query (real IDs)', () => {
      vi.mocked(useMatches).mockReturnValue([
        {
          id: '0', pathname: '/', params: {}, data: undefined, loaderData: undefined,
          handle: {},
        },
        {
          id: '0-4', pathname: '/trackers', params: {}, data: undefined, loaderData: undefined,
          handle: {},
        },
        {
          id: '0-4-2', pathname: '/trackers/1', params: { trackerId: '1' },
          data: { tracker: { id: 1, name: 'My Tracker' } }, loaderData: undefined,
          handle: { crumb: (params: any, data: any) => ({ label: data?.tracker?.name ?? 'Tracker' }) },
        },
      ])
      render(
        <MemoryRouter initialEntries={[{ pathname: '/trackers/1', state: { fromSearch: 'foo' } }]}>
          <Breadcrumbs />
        </MemoryRouter>
      )
      expect(screen.getByText('Search Results')).toBeInTheDocument()
      expect(screen.getByText('My Tracker')).toBeInTheDocument()
      const link = screen.getByText('Search Results').closest('a')
      expect(link).toHaveAttribute('href', '/?q=foo')
    })

    it('shows @Username > Tracker Name on tracker detail page without fromSearch (real IDs)', () => {
      vi.mocked(useMatches).mockReturnValue([
        {
          id: '0', pathname: '/', params: {}, data: undefined, loaderData: undefined,
          handle: {},
        },
        {
          id: '0-4', pathname: '/trackers', params: {}, data: undefined, loaderData: undefined,
          handle: {},
        },
        {
          id: '0-4-2', pathname: '/trackers/1', params: { trackerId: '1' },
          data: { tracker: { id: 1, name: 'My Tracker', owner_name: 'alice' } }, loaderData: undefined,
          handle: { crumb: (params: any, data: any) => ({ label: data?.tracker?.name ?? 'Tracker' }) },
        },
      ])
      render(
        <MemoryRouter initialEntries={[{ pathname: '/trackers/1' }]}>
          <Breadcrumbs />
        </MemoryRouter>
      )
      expect(screen.queryByText('Search Results')).toBeNull()
      expect(screen.getByText('@alice')).toBeInTheDocument()
      expect(screen.getByText('My Tracker')).toBeInTheDocument()
    })

    it('shows Search Results > Tracker Name > Edit on edit page with fromSearch (real IDs)', () => {
      vi.mocked(useMatches).mockReturnValue([
        {
          id: '0', pathname: '/', params: {}, data: undefined, loaderData: undefined,
          handle: {},
        },
        {
          id: '0-4', pathname: '/trackers', params: {}, data: undefined, loaderData: undefined,
          handle: {},
        },
        {
          id: '0-4-3', pathname: '/trackers/1/edit', params: { trackerId: '1' },
          data: { tracker: { id: 1, name: 'My Tracker' } }, loaderData: undefined,
          handle: { crumb: () => ({ label: 'Edit' }) },
        },
      ])
      render(
        <MemoryRouter initialEntries={[{ pathname: '/trackers/1/edit', state: { fromSearch: 'foo' } }]}>
          <Breadcrumbs />
        </MemoryRouter>
      )
      expect(screen.getByText('Search Results')).toBeInTheDocument()
      expect(screen.getByText('My Tracker')).toBeInTheDocument()
      expect(screen.getByText('Edit')).toBeInTheDocument()
      const searchLink = screen.getByText('Search Results').closest('a')
      expect(searchLink).toHaveAttribute('href', '/?q=foo')
      const trackerLink = screen.getByText('My Tracker').closest('a')
      expect(trackerLink).toHaveAttribute('href', '/trackers/1')
    })

    it('shows @Username > Tracker Name > Edit on edit page without fromSearch (real IDs)', () => {
      vi.mocked(useMatches).mockReturnValue([
        {
          id: '0', pathname: '/', params: {}, data: undefined, loaderData: undefined,
          handle: {},
        },
        {
          id: '0-4', pathname: '/trackers', params: {}, data: undefined, loaderData: undefined,
          handle: {},
        },
        {
          id: '0-4-3', pathname: '/trackers/1/edit', params: { trackerId: '1' },
          data: { tracker: { id: 1, name: 'My Tracker', owner_name: 'alice' } }, loaderData: undefined,
          handle: { crumb: () => ({ label: 'Edit' }) },
        },
      ])
      render(
        <MemoryRouter initialEntries={[{ pathname: '/trackers/1/edit' }]}>
          <Breadcrumbs />
        </MemoryRouter>
      )
      expect(screen.queryByText('Search Results')).toBeNull()
      expect(screen.getByText('@alice')).toBeInTheDocument()
      expect(screen.getByText('My Tracker')).toBeInTheDocument()
      expect(screen.getByText('Edit')).toBeInTheDocument()
    })
  })
})

describe('rootShouldRevalidate', () => {
  it('returns true when pathname changes', () => {
    expect(rootShouldRevalidate({
      currentUrl: new URL('http://localhost/auth'),
      nextUrl: new URL('http://localhost/'),
      defaultShouldRevalidate: false,
    })).toBe(true)
  })

  it('returns true when pathname changes reverse', () => {
    expect(rootShouldRevalidate({
      currentUrl: new URL('http://localhost/'),
      nextUrl: new URL('http://localhost/auth'),
      defaultShouldRevalidate: false,
    })).toBe(true)
  })

  it('returns defaultShouldRevalidate when pathname is same', () => {
    expect(rootShouldRevalidate({
      currentUrl: new URL('http://localhost/'),
      nextUrl: new URL('http://localhost/'),
      defaultShouldRevalidate: false,
    })).toBe(false)
  })

  it('returns true when defaultShouldRevalidate is true and pathname is same', () => {
    expect(rootShouldRevalidate({
      currentUrl: new URL('http://localhost/'),
      nextUrl: new URL('http://localhost/'),
      defaultShouldRevalidate: true,
    })).toBe(true)
  })
})
