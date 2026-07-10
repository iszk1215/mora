import { DateTime } from 'luxon'
import React from 'react'
import {
  LoaderFunctionArgs,
  Params,
  redirect,
  useLoaderData,
  useParams,
} from 'react-router'

import { Coverage, CoverageEntry, FileData, Repo } from './core'
import { CoverageChart } from './coverage_chart'
import { Browser } from './browser'
import { CodeView } from './codeview'
import { DefaultLink, ExternalLink } from './util'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'

interface CoverageEntryMetadata {
  hits: number
  lines: number
  revision: string
  revision_url: string
  time: string
}

interface CoverageEntryData {
  files: FileData[]
  meta: CoverageEntryMetadata
}

import type { CoverageBlock } from './core'

interface CodeData {
  repo: Repo
  filename: string
  code: string
  blocks: CoverageBlock[]
}

export function makeRepoCoverageListPath(params: Params) {
  return `repos/${params.repo_id}/coverages`
}

export function makeEntryPath(params: Params) {
  return `${makeRepoCoverageListPath(params)}/${params.index}/${params.entry}`
}

// formatters

export function formatRevision(revision: string) {
  return revision.substring(0, 10)
}

export function formatTime(time: string) {
  return DateTime.fromISO(time).toLocaleString(DateTime.DATETIME_FULL)
}

export function formatRatio(hits: number, lines: number) {
  if (lines === 0) { return 'N/A' }
  return (hits * 100.0 / lines).toFixed(1)
}

// returns CodeData
async function loadFile({ params }: LoaderFunctionArgs): Promise<Response> {
  const url = `/api/${makeEntryPath(params)}/files/${params["*"]}`
  const resp = await fetch(url)
  if (!resp.ok)
    throw resp
  return resp
}

const FilePage = (): React.JSX.Element => {
  const data = useLoaderData() as CodeData
  return (
    <div>
      <CodeView path={data.filename} code={data.code} blocks={data.blocks} />
    </div>)
}

// CoverageEntryPage

async function loadCoverageEntry({ params }: LoaderFunctionArgs): Promise<Response> {
  const url = `/api/${makeEntryPath(params)}/files`
  const resp = await fetch(url)
  if (resp.status == 403) {
    return redirect("/auth")
  }
  if (!resp.ok)
    throw resp
  return resp
}

export const CoverageEntryPage = (): React.JSX.Element => {
  const resp = useLoaderData() as CoverageEntryData
  const meta = resp.meta

  return (
    <div>
      <h2 className="text-3xl my-2">Coverage</h2>
      <div className="flex justify-between mb-2">
        <div>
          Lines: {meta.hits}/{meta.lines} Coverage: {formatRatio(meta.hits, meta.lines)}%
        </div>
        <div>
          <span className="mr-2">
            <ExternalLink href={meta.revision_url}>
              {formatRevision(meta.revision)}
            </ExternalLink>
          </span>
          {formatTime(meta.time)}
        </div>
      </div>
      <div className="overflow-y-auto" style={{ maxHeight: 'calc(100dvh - 180px)' }}>
        <Browser files={resp.files} />
      </div>
    </div>)
}


// CoverageListPage

async function loadCoverageList({ params }: { params: Params }): Promise<Response | {
  repo: Repo,
  coverages: Coverage[]
}> {
  const url = `/api/repos/${params.repo_id}/coverages`
  const resp = await fetch(url)
  if (resp.status == 403) {
    return redirect("/auth")
  }
  if (!resp.ok)
    throw resp

  const data = await resp.json()

  const coverages = data.coverages

  // preprocess
  coverages.sort((a: Coverage, b: Coverage) => {
    return b.index - a.index
  })

  for (const cov of coverages) {
    let hits = 0; let lines = 0

    cov.entries.sort((a: CoverageEntry, b: CoverageEntry) => {
      return a.name.localeCompare(b.name)
    })

    for (const e of cov.entries) {
      hits += e.hits
      lines += e.lines
    }
    cov.hits = hits
    cov.lines = lines
  }

  return { repo: data.repo, coverages: coverages }
}

interface CoverageSegmentProperty {
  cov: Coverage,
  params: Params
}


export const CoverageSegment = (props: CoverageSegmentProperty): React.JSX.Element => {
  const params = props.params
  const cov = props.cov

  const elems: React.JSX.Element[] = []

  if (cov.entries.length > 1) { // Add "Total"
    elems.push(
      <span>
        Total {formatRatio(cov.hits, cov.lines)}% ({cov.hits}/{cov.lines})
      </span>)
  }

  cov.entries.forEach((e: CoverageEntry) => {
    const href = `/${makeRepoCoverageListPath(params)}/${cov.index}/${e.name}`
    elems.push(
      <DefaultLink to={href}>
        {e.name} {formatRatio(e.hits, e.lines)}% ({e.hits}/{e.lines})
      </DefaultLink>)
  })

  const elemsWithMargin = elems.map((e: React.JSX.Element, i: number) => {
    return <span className="mx-2" key={i}>{e} </span>
  })

  return (
    <Card size="sm" className="my-2">
      <CardContent className="flex justify-between text-base">
        <div>
          <Badge variant="outline" className="mr-2">#{cov.index}</Badge>
          {elemsWithMargin}
        </div>
        <div>
          <span className="mr-2">{formatTime(cov.time)}</span>
          <ExternalLink href={cov.revision_url}>
            {formatRevision(cov.revision)}
          </ExternalLink>
        </div>
      </CardContent>
    </Card>)
}

export const CoverageListContent = ({ repo, coverages, params }: {
  repo: Repo
  coverages: Coverage[]
  params: Params
}): React.JSX.Element => {
  const items: React.JSX.Element[] = []
  coverages.forEach((cov: Coverage, i: number) => {
    items.push(<div key={i}>
      <CoverageSegment cov={cov} params={params} />
    </div>)
  })

  return (
    <div><h2 className="text-3xl my-4">Coverages</h2>
      <div className="mb-4">
        Repository: <ExternalLink href={repo.url}>{repo.url}</ExternalLink>
      </div>
      <CoverageChart coverages={coverages} />
      <div>{items}</div>
    </div>)
}

export const CoverageList = (): React.JSX.Element => {
  const data = useLoaderData() as { repo: Repo, coverages: Coverage[] }
  const params = useParams()
  return <CoverageListContent repo={data.repo} coverages={data.coverages} params={params} />
}

export const coverageRoute = [
  {
    index: true,
    element: <CoverageList />,
    loader: loadCoverageList,
  },
  {
    path: ':index',
    handle: {
      crumb: (params: Params) => {
        return { label: `#${params.index}` }
      }
    },
    children: [
      {
        path: ':entry',
        handle: {
          crumb: (params: Params) => ({
            label: params.entry,
            link: `/${makeEntryPath(params)}`
          }),
        },
        children: [
          {
            index: true,
            element: <CoverageEntryPage />,
            loader: loadCoverageEntry,
          },
          {
            path: '*',
            element: <FilePage />,
            loader: loadFile,
            handle: {
              crumb: (params: Params) => ({ label: params["*"] })
            },
          },
        ],
      },
    ],
  },
]


