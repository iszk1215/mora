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
  useLocation,
  useMatches,
  useRouteError,
} from 'react-router'
import { RouterProvider } from 'react-router/dom'
import 'react-datepicker/dist/react-datepicker.css'
import './index.css'

import { Repo, UserData } from './core'
import { coverageRoute } from './coverage'
import { udmRoute, loadUdmMetrics } from './udm'
import { trackerRoute } from './tracker'
import { signupRoute } from './signup'
import { apiKeyRoute } from './apikey'
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

// RepoList

interface Metric {
  name: string,
  link: string,
}

async function loadMetrics(repo_id: number): Promise<Metric[]> {
  const metrics: Metric[] = [{ name: "coverage", link: "coverages" }]

  const udmMetrics = await loadUdmMetrics(repo_id)
  const tmp: Metric[] = udmMetrics.map(
    (m): Metric => ({ name: m.name, link: `udm/metrics/${m.id}` }))

  return metrics.concat(tmp)
}

export async function loadRepoList(): Promise<Repo[]> {
  const data = await fetch('/api/repos')
  const json = await data.json()
  return json
}

export const RepoList = (): React.JSX.Element => {
  const repos: Repo[] = useLoaderData() as Repo[]

  const [metrics, setMetrics] = useState<Metric[][]>([])

  useEffect(() => {
    Promise.all(
      repos.map((r: Repo) => loadMetrics(r.id))).then(setMetrics).catch(() => {})
  }, [])


  const elems: React.JSX.Element[] = []

  repos.forEach((repo: Repo, i: number) => {
    let metricElems: React.JSX.Element[] = []
    if (metrics.length > 0) {
      metricElems = metrics[i].map((m, j) =>
        <li key={j}>
          <DefaultLink to={`/repos/${repo.id}/${m.link}`}>{m.name}</DefaultLink>
        </li>
      )
    }
    elems.push(
      <li className="mb-4" key={i}>
        <h2 className="text-lg">{repo.url}</h2>
        <ul className="list-inside list-disc pl-8">{metricElems}</ul>
      </li>)
  })

  return (
    <div>
      <h2 className="text-3xl my-4">Repositories</h2>
      <ul className="list-inside">{elems}</ul>
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
  const resp = await fetch('/api/scms')
  if (!resp.ok)
    throw resp
  return resp
}

const GitHubIcon = () => (
  <svg viewBox="0 0 24 24" className="w-5 h-5 fill-current" aria-hidden="true">
    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
  </svg>
)

const GiteaIcon = () => (
  <svg viewBox="0 0 24 24" className="w-5 h-5 fill-current" aria-hidden="true">
    <path d="M8 4h10a2 2 0 012 2v8a4 4 0 01-4 4H8a4 4 0 01-4-4V6a2 2 0 012-2zm0 2v8a2 2 0 002 2h6a2 2 0 002-2V6H8zm10 4h2a1 1 0 011 1v2a1 1 0 01-1 1h-2v-4z"/>
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
          Login with {scm.name}
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

export const Header = (): React.JSX.Element => {
  const [user, setUser] = useState<UserData | null>(null)
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  const location = useLocation()

  useEffect(() => {
    fetch('/api/me')
      .then(res => {
        if (res.status === 204) return null
        if (res.ok) return res.json()
        return null
      })
      .then((data: UserData | null) => setUser(data))
      .catch(() => setUser(null))
  }, [location])

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
    <header className="sticky top-0 mb-2 bg-black text-white py-1">
      <div className="w-8/12 m-auto">
        <nav className="flex justify-between">
          <HeaderLink to={'/'}>Mora</HeaderLink>
          <div className="flex items-center gap-2">
            {user ? (
              <div ref={menuRef} className="relative">
                {user.avatar_url && (
                  <img src={user.avatar_url} className="w-6 h-6 rounded-full cursor-pointer"
                       title="Open menu" alt={user.username}
                       onClick={() => setMenuOpen(!menuOpen)} />
                )}
                {menuOpen && (
                  <div className="absolute right-0 mt-2 w-40 bg-white text-black rounded shadow-lg z-50">
                    <a href="/tracker"
                       className="block px-4 py-2 hover:bg-gray-100 text-sm"
                       onClick={() => setMenuOpen(false)}>My Trackers</a>
                    <a href="/tracker/new"
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
              <HeaderLink to={'/scms'}>Login</HeaderLink>
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

  const last = matches[matches.length - 1]

  const crumbs: Crumb[] = matches
    .filter((match: any) => Boolean(match.handle?.crumb))
    .map((match: any) => match.handle.crumb(match.params, last.data))
    .filter((crumb: Crumb) => Boolean(crumb.label))

  return makeBredcrumbs(crumbs)
}

const Root = (): React.JSX.Element => {
  return (
    <div>
      <ScrollRestoration />
      <Header />
      <div className="w-8/12 m-auto">
        <Breadcrumbs />
        <Outlet />
      </div>
    </div>
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

const router = createBrowserRouter([
  {
    path: '/',
    element: <Root />,
    errorElement: <ErrorPage />,
    handle: {
      crumb: () => ({ label: "Top", link: "/" }),
    },
    children: [
      {
        index: true,
        element: <RepoList />,
        loader: loadRepoList,
      },
      {
        path: '/scms',
        element: <SCMList />,
        loader: loadSCMList,
        handle: {
          crumb: (_params: Params, _data: any) => ({ label: "login", link: "/scms" }),
        }
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
        path: '/tracker',
        handle: {
          crumb: (_params: Params, _data: any) => ({ label: "Tracker", link: "/tracker" }),
        },
        children: trackerRoute,
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
            path: 'coverages',
            handle: {
              crumb: (params: Params) => ({
                label: "Coverages",
                link: `/repos/${params.repo_id}/coverages`
              })
            },
            children: coverageRoute,
          },
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
