import React from 'react'
import { useEffect, useState } from 'react'
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
import { trackRoute } from './track'
import { DefaultLink, HeaderLink, ExternalLink } from './util'
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
  logined: boolean
}

// returns SCMData[]
async function loadSCMList(): Promise<Response> {
  const resp = await fetch('/api/scms')
  if (!resp.ok)
    throw resp
  return resp
}

export const SCMList = (): React.JSX.Element => {
  const scmList = useLoaderData() as SCMData[]
  const items: React.JSX.Element[] = []
  scmList.forEach((scm: SCMData, i: number) => {
    let buttons: React.JSX.Element
    if (scm.logined) {
      buttons = (
        <div className="flex mb-2">
          <Button variant="ghost" disabled>login</Button>
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
            <Button variant="secondary" type="submit">logout</Button>
          </form>
        </div>
      )
    } else {
      buttons = (
        <div className="flex mb-2">
          <Button variant="secondary" asChild>
            <a href={'/login/' + scm.id}>login</a>
          </Button>
          <Button variant="ghost" disabled>logout</Button>
        </div>
      )
    }
    const item: React.JSX.Element = (
      <div key={i}>
        <ExternalLink href={scm.url}>{scm.url}</ExternalLink>
        {buttons}
      </div>
    )

    items.push(item)
  })

  return (
    <div>
      <h1 className="text-3xl my-2">SCM</h1>
      <div>
        {items}
      </div>
    </div>
  )
}

export const Header = (): React.JSX.Element => {
  const [user, setUser] = useState<UserData | null>(null)
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

  return (
    <header className="sticky top-0 mb-2 bg-black text-white">
      <div className="w-8/12 m-auto">
        <nav className="flex justify-between">
          <HeaderLink to={'/'}>Top</HeaderLink>
          <div className="flex items-center gap-2">
            {user ? (
              <a href="/scms" className="block">
                {user.avatar_url && (
                  <img src={user.avatar_url} className="w-6 h-6 rounded-full"
                       title={user.username} alt={user.username} />
                )}
              </a>
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
          crumb: (_params: Params, _data: any) => ({ label: "scm", link: "/scms" }),
        }
      },
      {
        path: '/track',
        handle: {
          crumb: (_params: Params, _data: any) => ({ label: "Track", link: "/track" }),
        },
        children: trackRoute,
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
