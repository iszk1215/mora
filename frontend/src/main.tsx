import React from 'react'
import { useEffect, useState, useRef, useCallback } from 'react'
import ReactDOM from 'react-dom/client'
import {
  createBrowserRouter,
  Outlet,
  Params,
  isRouteErrorResponse,
  ScrollRestoration,
  useLoaderData,
  useMatches,
  useRouteError,
  useSearchParams,
  useLocation,
} from 'react-router'
import { RouterProvider } from 'react-router/dom'
import 'react-datepicker/dist/react-datepicker.css'
import './index.css'

import { UserData, TrackerResponse } from './core'
import { UserProvider, useUser } from './user-context'
import { SearchContext, useSearch } from './search-context'
import type { SearchState } from './search-context'
import { coverageTrackerRoute } from './coverage'
import { udmRoute } from './udm'
import { trackerRoute, listTrackers, TrackerCard, fetchPreview, PreviewData } from './tracker'
import { userPageRoute } from './user'
import { signupRoute } from './signup'
import { apiKeyRoute } from './apikey'
import { PasswordLoginForm } from './auth'
import { DefaultLink, HeaderLink } from './util'
import { Button } from '@/components/ui/button'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'

// Tracker Search Page (top page)

const TrackerSearchPage = (): React.JSX.Element => {
  const [searchParams, setSearchParams] = useSearchParams()
  const urlQuery = searchParams.get('q') ?? ''
  const [query, setQuery] = useState(urlQuery)
  const [trackers, setTrackers] = useState<TrackerResponse[]>([])
  const [previews, setPreviews] = useState<Map<number, PreviewData>>(new Map())
  const [searching, setSearching] = useState(false)
  const [initial, setInitial] = useState(true)
  const search = useSearch()

  // Initial load: check context cache first, then fetch
  useEffect(() => {
    // If URL has query and context has cached results for this query, use cache
    if (urlQuery && search?.query === urlQuery && search.results.length > 0) {
      setTrackers(search.results)
      setPreviews(search.previews)
      setInitial(false)
      return
    }

    // Otherwise fetch from API
    const q = urlQuery || undefined
    listTrackers(1, urlQuery ? 12 : 100, q)
      .then((data) => {
        setTrackers(data.trackers)
        setInitial(false)
        // If URL has query, save to context
        if (urlQuery) {
          search?.setSearch({ query: urlQuery, results: data.trackers, previews: new Map() })
        }
        return undefined
      })
      .catch(() => setInitial(false))
  }, [urlQuery])

  // Fetch previews when trackers change
  useEffect(() => {
    if (trackers.length === 0) return
    const loadAll = async () => {
      const entries = await Promise.all(
        trackers.map(async (t) => {
          try {
            const data = await fetchPreview(t.id, t.type)
            return [t.id, data] as const
          } catch {
            return null
          }
        })
      )
      const map = new Map<number, PreviewData>()
      for (const entry of entries) {
        if (entry) map.set(entry[0], entry[1])
      }
      setPreviews(map)
      // Update context with previews
      if (urlQuery && search) {
        search.setSearch({ query: urlQuery, results: trackers, previews: map })
      }
    }
    void loadAll()
  }, [trackers])

  // Clear search context when navigating away from top page
  useEffect(() => {
    return () => {
      search?.setSearch({ query: '', results: [], previews: new Map() })
    }
  }, [])

  const handleSearch = async () => {
    const q = query.trim()
    setSearching(true)
    try {
      const data = await listTrackers(1, 12, q || undefined)
      setTrackers(data.trackers)
      // Update URL params
      if (q) {
        setSearchParams({ q }, { replace: true })
      } else {
        setSearchParams({}, { replace: true })
      }
      // Save to context (previews will be updated by the useEffect above)
      search?.setSearch({ query: q, results: data.trackers, previews: new Map() })
    } catch {
      setTrackers([])
    } finally {
      setSearching(false)
    }
  }

  return (
    <div>
      <div className="flex justify-center items-center gap-2 mb-6">
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          placeholder="Search trackers..."
          className="w-96 border rounded px-3 py-2"
        />
        <Button onClick={handleSearch} disabled={searching}>
          Search
        </Button>
      </div>
      {initial && <p className="text-muted-foreground">Loading...</p>}
      {!searching && trackers.length === 0 && query && (
        <p className="text-muted-foreground">No trackers found.</p>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {trackers.map((t) => (
          <TrackerCard key={t.id} tracker={t} preview={previews.get(t.id)} searchQuery={urlQuery} />
        ))}
      </div>
    </div>
  )
}

// SCM List

interface SCMData {
  id: number
  url: string
  name: string
  logined: boolean
}

// returns SCMData[]
async function loadSCMList(): Promise<Response> {
  const resp = await fetch('/api/providers')
  if (!resp.ok)
    throw resp
  return resp
}

const GitHubIcon = () => (
  <svg viewBox="0 0 24 24" className="w-5 h-5 fill-current" aria-hidden="true">
    <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/>
  </svg>
)

const GiteaIcon = () => (
  <svg viewBox="0 0 24 24" className="w-5 h-5 fill-current" aria-hidden="true">
    <path d="M4.209 4.603c-.247 0-.525.02-.84.088-.333.07-1.28.283-2.054 1.027C-.403 7.25.035 9.685.089 10.052c.065.446.263 1.687 1.21 2.768 1.749 2.141 5.513 2.092 5.513 2.092s.462 1.103 1.168 2.119c.955 1.263 1.936 2.248 2.89 2.367 2.406 0 7.212-.004 7.212-.004s.458.004 1.08-.394c.535-.324 1.013-.893 1.013-.893s.492-.527 1.18-1.73c.21-.37.385-.729.538-1.068 0 0 2.107-4.471 2.107-8.823-.042-1.318-.367-1.55-.443-1.627-.156-.156-.366-.153-.366-.153s-4.475.252-6.792.306c-.508.011-1.012.023-1.512.027v4.474l-.634-.301c0-1.39-.004-4.17-.004-4.17-1.107.016-3.405-.084-3.405-.084s-5.399-.27-5.987-.324c-.187-.011-.401-.032-.648-.032zm.354 1.832h.111s.271 2.269.6 3.597C5.549 11.147 6.22 13 6.22 13s-.996-.119-1.641-.348c-.99-.324-1.409-.714-1.409-.714s-.73-.511-1.096-1.52C1.444 8.73 2.021 7.7 2.021 7.7s.32-.859 1.47-1.145c.395-.106.863-.12 1.072-.12zm8.33 2.554c.26.003.509.127.509.127l.868.422-.529 1.075a.686.686 0 0 0-.614.359.685.685 0 0 0 .072.756l-.939 1.924a.69.69 0 0 0-.66.527.687.687 0 0 0 .347.763.686.686 0 0 0 .867-.206.688.688 0 0 0-.069-.882l.916-1.874a.667.667 0 0 0 .237-.02.657.657 0 0 0 .271-.137 8.826 8.826 0 0 1 1.016.512.761.761 0 0 1 .286.282c.073.21-.073.569-.073.569-.087.29-.702 1.55-.702 1.55a.692.692 0 0 0-.676.477.681.681 0 1 0 1.157-.252c.073-.141.141-.282.214-.431.19-.397.515-1.16.515-1.16.035-.066.218-.394.103-.814-.095-.435-.48-.638-.48-.638-.467-.301-1.116-.58-1.116-.58s0-.156-.042-.27a.688.688 0 0 0-.148-.241l.516-1.062 2.89 1.401s.48.218.583.619c.073.282-.019.534-.069.657-.24.587-2.1 4.317-2.1 4.317s-.232.554-.748.588a1.065 1.065 0 0 1-.393-.045l-.202-.08-4.31-2.1s-.417-.218-.49-.596c-.083-.31.104-.691.104-.691l2.073-4.272s.183-.37.466-.497a.855.855 0 0 1 .35-.077z"/>
  </svg>
)

const LoginIcon = () => (
  <svg viewBox="0 0 24 24" className="w-5 h-5 fill-current" aria-hidden="true">
    <path d="M10 17l5-5-5-5v3H3v4h7v3zm9-14H5a2 2 0 00-2 2v3h2V5h14v14H5v-3H3v3a2 2 0 002 2h14a2 2 0 002-2V5a2 2 0 00-2-2z"/>
  </svg>
)

function scmIcon(name: string) {
  const lower = name.toLowerCase()
  if (lower.includes('github')) return <GitHubIcon />
  if (lower.includes('gitea') || lower.includes('tea')) return <GiteaIcon />
  return <LoginIcon />
}

function displayName(name: string): string {
  const lower = name.toLowerCase()
  if (lower === 'github') return 'GitHub'
  if (lower === 'gitea') return 'Gitea'
  return name.charAt(0).toUpperCase() + name.slice(1)
}

function scmBrandStyle(name: string): React.CSSProperties {
  const lower = name.toLowerCase()
  if (lower.includes('github')) {
    return { backgroundColor: '#24292f', color: '#fff' }
  }
  if (lower.includes('gitea') || lower.includes('tea')) {
    return { backgroundColor: '#609926', color: '#fff', border: '1px solid #4a7a1e' }
  }
  return { backgroundColor: '#6366f1', color: '#fff' }
}

export const AuthPage = (): React.JSX.Element => {
  return (
    <div>
      <SCMList />
      <div className="max-w-md mx-auto">
        <PasswordLoginForm />
      </div>
    </div>
  )
}

export const SCMList = (): React.JSX.Element => {
  const scmList = useLoaderData() as SCMData[]
  const items: React.JSX.Element[] = []
  scmList.forEach((scm: SCMData, i: number) => {
    let card: React.JSX.Element
    if (scm.logined) {
      card = (
        <div key={i} className="flex items-center justify-between p-4 border rounded-lg">
          <span className="text-sm text-green-600 flex items-center gap-1">
            <svg viewBox="0 0 24 24" className="w-4 h-4 fill-current" aria-hidden="true">
              <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/>
            </svg>
            Connected
          </span>
          <form method="POST" action={'/logout/' + scm.id}
                onSubmit={(e) => {
                  const match = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/)
                  if (match) {
                    const input = document.createElement('input')
                    input.type = 'hidden'
                    input.name = 'csrf_token'
                    input.value = match[1]
                    e.currentTarget.appendChild(input)
                  }
                }}>
            <Button variant="outline" type="submit" size="sm">Logout</Button>
          </form>
        </div>
      )
    } else {
      card = (
        <a key={i} href={'/login/' + scm.id}
           className="flex items-center justify-center gap-3 w-full px-4 py-3 rounded-lg text-sm font-semibold transition-colors hover:opacity-90 no-underline"
           style={scmBrandStyle(scm.name)}>
          {scmIcon(scm.name)}
           Login with {displayName(scm.name)}
        </a>
      )
    }
    items.push(card)
  })

  return (
    <div className="max-w-md mx-auto mt-8">
      <h1 className="text-3xl font-bold mb-6">Login</h1>
      <div className="space-y-3">
        {items}
      </div>
    </div>
  )
}

let configCache: { site_name: string } | null = null
let configPromise: Promise<{ site_name: string }> | null = null

export function resetConfigCache() {
  configCache = null
  configPromise = null
}

export const Header = (): React.JSX.Element => {
  const user = useUser()
  const [siteName, setSiteName] = useState('Mora')
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!configPromise) {
      configPromise = fetch('/api/config')
        .then(r => r.json())
        .then(cfg => { configCache = cfg; return cfg })
        .catch(() => ({ site_name: 'Mora' }))
    }
    configPromise.then(cfg => setSiteName(cfg.site_name))
  }, [])

  const handleClickOutside = useCallback((e: MouseEvent) => {
    if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
      setMenuOpen(false)
    }
  }, [])

  useEffect(() => {
    if (menuOpen) {
      document.addEventListener('mousedown', handleClickOutside)
    }
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [menuOpen, handleClickOutside])

  return (
    <header className="sticky top-0 mb-2 bg-black text-white py-1 z-10">
      <div className="px-4 sm:px-8">
        <nav className="flex justify-between">
          <HeaderLink to={'/'}>{siteName}</HeaderLink>
          <div className="flex items-center gap-2">
            {user ? (
              <div ref={menuRef} className="relative">
                {user.avatar_url ? (
                  <img src={user.avatar_url} className="w-6 h-6 rounded-full cursor-pointer"
                       title="Open menu" alt={user.username}
                       onClick={() => setMenuOpen(!menuOpen)} />
                ) : (
                  <div className="w-6 h-6 rounded-full bg-blue-500 cursor-pointer flex items-center justify-center text-xs font-bold text-white select-none"
                       title="Open menu"
                       onClick={() => setMenuOpen(!menuOpen)}>
                    {user.username.charAt(0).toUpperCase()}
                  </div>
                )}
                {menuOpen && (
                  <div className="absolute right-0 mt-2 w-40 bg-card text-black rounded shadow-lg z-50">
                    <a href="/trackers"
                       className="block px-4 py-2 hover:bg-gray-100 text-sm"
                       onClick={() => setMenuOpen(false)}>My Trackers</a>
                    <a href="/trackers/new"
                       className="block px-4 py-2 hover:bg-gray-100 text-sm"
                       onClick={() => setMenuOpen(false)}>Create Tracker</a>
                    <a href="/settings/api-keys"
                       className="block px-4 py-2 hover:bg-gray-100 text-sm"
                       onClick={() => setMenuOpen(false)}>API Keys</a>
                    <form method="POST" action="/logout/"
                          onSubmit={(e) => {
                            const match = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/)
                            if (match) {
                              const input = document.createElement('input')
                              input.type = 'hidden'
                              input.name = 'csrf_token'
                              input.value = match[1]
                              e.currentTarget.appendChild(input)
                            }
                          }}>
                      <button type="submit"
                              className="block w-full text-left px-4 py-2 hover:bg-gray-100 text-sm">
                        Logout
                      </button>
                    </form>
                  </div>
                )}
              </div>
            ) : (
              <HeaderLink to={'/auth'}>Login</HeaderLink>
            )}
          </div>
        </nav>
      </div>
    </header>
  )
}

interface Crumb {
  label: string
  link?: string
}

export const makeBredcrumbs = (crumbs: Crumb[]): React.JSX.Element => {
  return (
    <Breadcrumb>
      <BreadcrumbList>
        {crumbs.map((crumb, i) => {
          const isLast = i === crumbs.length - 1
          return (
            <React.Fragment key={i}>
              <BreadcrumbItem>
                {isLast ? (
                  <BreadcrumbPage>{crumb.label}</BreadcrumbPage>
                ) : crumb.link ? (
                  <BreadcrumbLink asChild>
                    <DefaultLink to={crumb.link}>{crumb.label}</DefaultLink>
                  </BreadcrumbLink>
                ) : (
                  <span>{crumb.label}</span>
                )}
              </BreadcrumbItem>
              {!isLast && <BreadcrumbSeparator />}
            </React.Fragment>
          )
        })}
      </BreadcrumbList>
    </Breadcrumb>
  )
}

export const Breadcrumbs = (): React.JSX.Element => {
  const matches = useMatches()
  const location = useLocation()
  const searchQuery = (location.state as any)?.fromSearch as string | undefined
  const fromUser = (location.state as any)?.fromUser as string | undefined

  const last = matches[matches.length - 1]

  const crumbs: Crumb[] = []

  // Detect if we are on a tracker detail or edit page using params (not route IDs)
  const hasTrackerId = matches.some(m => Boolean((m.params as any)?.trackerId))
  const isOnEditPage = last.pathname.endsWith('/edit')
  const isTrackerDetail = hasTrackerId && !isOnEditPage
  const isTrackerEdit = hasTrackerId && isOnEditPage

  // For tracker detail page: add "Search Results" parent if navigated from a search
  // (top page or user page), otherwise add the owner username (linking to the user page)
  if (isTrackerDetail) {
    if (searchQuery) {
      if (fromUser) {
        crumbs.push({ label: fromUser, link: `/users/${encodeURIComponent(fromUser)}` })
      }
      crumbs.push({
        label: 'Search Results',
        link: fromUser
          ? `/users/${encodeURIComponent(fromUser)}?q=${encodeURIComponent(searchQuery)}`
          : `/?q=${encodeURIComponent(searchQuery)}`,
      })
    } else {
      const ownerName = (last.data as any)?.tracker?.owner_name as string | undefined
      if (ownerName) {
        crumbs.push({ label: ownerName, link: `/users/${encodeURIComponent(ownerName)}` })
      }
    }
  }

  // For tracker edit page: always add tracker name, plus "Search Results" or owner username
  if (isTrackerEdit) {
    if (searchQuery) {
      if (fromUser) {
        crumbs.push({ label: fromUser, link: `/users/${encodeURIComponent(fromUser)}` })
      }
      crumbs.push({
        label: 'Search Results',
        link: fromUser
          ? `/users/${encodeURIComponent(fromUser)}?q=${encodeURIComponent(searchQuery)}`
          : `/?q=${encodeURIComponent(searchQuery)}`,
      })
    } else {
      const ownerName = (last.data as any)?.tracker?.owner_name as string | undefined
      if (ownerName) {
        crumbs.push({ label: ownerName, link: `/users/${encodeURIComponent(ownerName)}` })
      }
    }
    // Find the tracker detail match to get tracker name
    const trackerMatch = matches.find((m: any) => Boolean(m.params?.trackerId))
    if (trackerMatch) {
      const trackerData = trackerMatch.data as any
      crumbs.push({
        label: trackerData?.tracker?.name ?? 'Tracker',
        link: `/trackers/${trackerMatch.params?.trackerId}`,
      })
    }
  }

  // Add route-defined crumbs
  matches
    .filter((match: any) => Boolean(match.handle?.crumb))
    .forEach((match: any) => {
      crumbs.push(match.handle.crumb(match.params, last.data))
    })

  const filtered = crumbs.filter((crumb: Crumb) => Boolean(crumb.label))
  if (filtered.length === 0) return <></>

  return makeBredcrumbs(filtered)
}

async function loadRootData(): Promise<{ user: UserData | null }> {
  try {
    const resp = await fetch('/api/me')
    if (resp.status === 204) return { user: null }
    if (resp.ok) return { user: await resp.json() as UserData }
    return { user: null }
  } catch {
    return { user: null }
  }
}

const Root = (): React.JSX.Element => {
  const { user } = useLoaderData() as { user: UserData | null }
  const [searchState, setSearchState] = useState<SearchState>({
    query: '',
    results: [],
    previews: new Map(),
  })

  return (
    <UserProvider value={user}>
      <SearchContext.Provider value={{ ...searchState, setSearch: setSearchState }}>
        <div>
          <ScrollRestoration />
          <Header />
          <div className="w-8/12 m-auto">
            <Breadcrumbs />
            <Outlet />
          </div>
        </div>
      </SearchContext.Provider>
    </UserProvider>
  )
}

export const ErrorPage = (): React.JSX.Element => {
  const error = useRouteError()
  let message = <span>Error</span>
  if (isRouteErrorResponse(error)) {
    message = <i>{error.statusText}</i>
  }
  return (
    <div>
      <ScrollRestoration />
      <Header />
      <div className="w-8/12 m-auto">
        <h1>Error</h1>
        <p>Sorry, unexpected error has happend. Back to the top page.</p>
        <p>{message}</p>
      </div>
    </div>
  )
}

export function rootShouldRevalidate({ currentUrl, nextUrl, defaultShouldRevalidate }: {
  currentUrl: URL
  nextUrl: URL
  defaultShouldRevalidate: boolean
}) {
  if (currentUrl.pathname !== nextUrl.pathname) return true
  return defaultShouldRevalidate
}

const router = createBrowserRouter([
  {
    path: '/',
    element: <Root />,
    loader: loadRootData,
    shouldRevalidate: rootShouldRevalidate,
    errorElement: <ErrorPage />,
    children: [
      {
        index: true,
        element: <TrackerSearchPage />,
      },
      {
        path: '/auth',
        element: <AuthPage />,
        loader: loadSCMList,
      },
      {
        path: '/signup',
        handle: {
          crumb: (_params: Params, _data: any) => ({ label: "Sign Up", link: "/signup" }),
        },
        children: [signupRoute],
      },
      {
        path: '/settings/api-keys',
        handle: {
          crumb: (_params: Params, _data: any) => ({ label: "API Keys", link: "/settings/api-keys" }),
        },
        children: [apiKeyRoute],
      },
      {
        path: '/trackers',
        children: trackerRoute,
      },
      {
        path: '/users/:userName',
        children: userPageRoute,
      },
      {
        path: '/coverages/:trackerId',
        handle: {
          crumb: (params: Params, data: any) => ({
            label: data?.trackerName ?? `Coverage #${params.trackerId}`,
            link: `/coverages/${params.trackerId}`,
          })
        },
        children: coverageTrackerRoute,
      },
      {
        path: '/repos/:repo_id',
        handle: {
          crumb: (params: Params, data: any) => {
            if (data)
              return { label: `${data.repo.namespace}/${data.repo.name}` }
            return { label: undefined }
          }
        },
        children: [
          {
            path: 'udm',
            handle: {},
            children: udmRoute,
          },
        ],
      },
    ]
  }
]
)

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>
)
