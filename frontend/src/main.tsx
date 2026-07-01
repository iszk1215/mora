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
  const [user, setUser] = useState<UserData | null>(null)
  const [siteName, setSiteName] = useState('Mora')
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  const location = useLocation()

  useEffect(() => {
    if (!configPromise) {
      configPromise = fetch('/api/config')
        .then(r => r.json())
        .then(cfg => { configCache = cfg; return cfg })
        .catch(() => ({ site_name: 'Mora' }))
    }
    configPromise.then(cfg => setSiteName(cfg.site_name))
  }, [])

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
          <HeaderLink to={'/'}>{siteName}</HeaderLink>
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
    children: [
      {
        index: true,
        element: <RepoList />,
        loader: loadRepoList,
      },
      {
        path: '/auth',
        element: <SCMList />,
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
