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
  useMatches,
  useRouteError,
} from 'react-router'
import { RouterProvider } from 'react-router/dom'
import './index.css'

import { Repo } from './core'
import { coverageRoute } from './coverage'
import { udmRoute, loadUdmMetrics } from './udm'
import { DefaultLink, HeaderLink, ExternalLink } from './util'

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

async function loadRepoList(): Promise<Repo[]> {
  const data = await fetch('/api/repos')
  const json = await data.json()
  return json
}

const RepoList = (): React.JSX.Element => {
  const repos: Repo[] = useLoaderData() as Repo[]

  const [metrics, setMetrics] = useState<Metric[][]>([])

  useEffect(() => {
    Promise.all(
      repos.map((r: Repo) => loadMetrics(r.id))).then(setMetrics)
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

const SCMList = (): React.JSX.Element => {
  const scmList = useLoaderData() as SCMData[]
  const items: React.JSX.Element[] = []
  scmList.forEach((scm: SCMData, i: number) => {
    let buttons: React.JSX.Element
    if (scm.logined) {
      buttons = (
        <div className="flex mb-2">
          <div className="px-4 py-2 rounded-l bg-gray-200 text-gray-400">login</div>
          <a className="px-4 py-2 rounded-r bg-gray-400 font-bold" href={'/logout/' + scm.id}>logout</a>
        </div>
      )
    } else {
      buttons = (
        <div className="flex mb-2">
          <a className="px-4 py-2 rounded-l bg-gray-400 font-bold" href={'/login/' + scm.id}>login</a>
          <div className="px-4 py-2 rounded-r bg-gray-200 text-gray-400">logout</div>
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

const Header = (): React.JSX.Element => {
  return (
    <header className="sticky top-0 mb-2 bg-black text-white">
      <div className="w-8/12 m-auto">
        <nav className="flex justify-between">
          <HeaderLink to={'/'}>Top</HeaderLink>
          <HeaderLink to={'/scms'}>Login/Logout</HeaderLink>
        </nav>
      </div>
    </header>
  )
}

interface Crumb {
  label: string
  link?: string
}

const makeBredcrumbs = (crumbs: Crumb[]): React.JSX.Element => {
  const elems = crumbs.map((crumb: { label: string, link?: string }, i: number) => {
    if (i < crumbs.length - 1 && crumb.link) {
      return <DefaultLink to={crumb.link} key={i}>{crumb.label}</DefaultLink>
    } else {
      return <span key={i}>{crumb.label}</span>
    }
  })

  const elems2: React.JSX.Element[] = []
  for (let i = 0; i < elems.length; i++) {
    elems2.push(elems[i])
    if (i < elems.length - 1)
      elems2.push(<span key={elems.length + i} className="mx-1">&gt;</span>)
  }

  return <div>{elems2}</div>
}

const Breadcrumbs = (): React.JSX.Element => {
  const matches = useMatches()

  const last = matches[matches.length - 1]
  const data = last.data as any

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

const ErrorPage = (): React.JSX.Element => {
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
          crumb: (params: Params, data: any) => ({ label: "scm", link: "/scms" }),
        }
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
