import { DateTime } from 'luxon'
import React, { useCallback, useMemo, useState } from 'react'
import {
  LoaderFunctionArgs,
  Params,
  redirect,
  useLoaderData,
  useParams,
} from 'react-router'

import { Coverage, CoverageEntry, FileData, Repo } from './core'
import { TrackerChart } from './chart'
import { Dataset } from './chart'
import { Browser } from './browser'
import { CodeView } from './codeview'
import { DefaultLink, ExternalLink } from './util'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { TimeRangeSelector, computeDateRange } from './time_range'
import type { TimeRangeKey } from './time_range'

interface Point {
  x: string
  y: number
  index: number
}

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

export function makeCoverageTrackerPath(params: Params) {
  return `coverages/${params.trackerId}`
}

export function makeCoverageTrackerEntryPath(params: Params) {
  return `${makeCoverageTrackerPath(params)}/${params.index}/${params.entry}`
}

export function buildEntryUrl(params: Params, cov: Coverage, entryName: string): string {
  return `/coverages/${params.trackerId}/${cov.index}/${entryName}`
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

export function buildCoverageClickUrl(repoId: string | undefined, trackerId: string | undefined, index: number, seriesName: string): string {
  return `/coverages/${trackerId}/${index}/${seriesName}`
}

const FilePage = (): React.JSX.Element => {
  const data = useLoaderData() as CodeData
  return (
    <div>
      <CodeView path={data.filename} code={data.code} blocks={data.blocks} />
    </div>)
}

// CoverageEntryPage

async function loadCoverageEntryByTracker({ params }: LoaderFunctionArgs): Promise<Response> {
  const url = `/api/coverages/${params.trackerId}/${params.index}/${params.entry}/files`
  const resp = await fetch(url)
  if (resp.status == 403) {
    return redirect("/auth")
  }
  if (!resp.ok)
    throw resp
  return resp
}

async function loadFileByTracker({ params }: LoaderFunctionArgs): Promise<Response> {
  const url = `/api/coverages/${params.trackerId}/${params.index}/${params.entry}/files/${params["*"]}`
  const resp = await fetch(url)
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


async function loadCoverageListByTracker({ params }: { params: Params }): Promise<Response | {
  trackerName: string,
  repo: Repo,
  coverages: Coverage[]
}> {
  const url = `/api/coverages/${params.trackerId}`
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

  const trackerResp = await fetch(`/api/trackers/${params.trackerId}`)
  const trackerName = trackerResp.ok ? (await trackerResp.json()).name : ''

  return { trackerName, repo: data.repo, coverages: coverages }
}

interface CoverageSegmentProperty {
  cov: Coverage,
  params: Params
}


export const CoverageSegment = (props: CoverageSegmentProperty): React.JSX.Element => {
  const params = props.params
  const cov = props.cov
  const hasScmAccess = !!cov.revision_url

  const elems: React.JSX.Element[] = []

  if (cov.entries.length > 1) { // Add "Total"
    elems.push(
      <span>
        Total {formatRatio(cov.hits, cov.lines)}% ({cov.hits}/{cov.lines})
      </span>)
  }

  cov.entries.forEach((e: CoverageEntry) => {
    if (hasScmAccess) {
      const href = buildEntryUrl(params, cov, e.name)
      elems.push(
        <DefaultLink to={href}>
          {e.name} {formatRatio(e.hits, e.lines)}% ({e.hits}/{e.lines})
        </DefaultLink>)
    } else {
      elems.push(
        <span>
          {e.name} {formatRatio(e.hits, e.lines)}% ({e.hits}/{e.lines})
        </span>)
    }
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
          {hasScmAccess ? (
            <ExternalLink href={cov.revision_url}>
              {formatRevision(cov.revision)}
            </ExternalLink>
          ) : (
            <span>{formatRevision(cov.revision)}</span>
          )}
        </div>
      </CardContent>
    </Card>)
}

export const CoverageListContent = ({ repo, coverages, params, min, max, rangeSelector }: {
  repo: Repo
  coverages: Coverage[]
  params: Params
  min?: Date | null
  max?: Date | null
  rangeSelector?: React.ReactNode
}): React.JSX.Element => {
  const items: React.JSX.Element[] = []
  coverages.forEach((cov: Coverage, i: number) => {
    items.push(<div key={i}>
      <CoverageSegment cov={cov} params={params} />
    </div>)
  })

  const datasets = useMemo(() => coverageToDatasets(coverages), [coverages])

  const onChartClick = useCallback((rawParams: any) => {
    if (rawParams.seriesName !== 'total') {
      const d = rawParams.data
      if (d?.index !== undefined) {
        const url = buildCoverageClickUrl(params.repo_id, params.trackerId, d.index, d.entryName)
        window.location.assign(url)
      }
    }
  }, [params.repo_id, params.trackerId])

  return (
    <div>
      <div className="mb-4">
        Repository: <ExternalLink href={repo.url}>{repo.url}</ExternalLink>
      </div>
      {rangeSelector}
      <TrackerChart
        data={{ datasets }}
        min={min}
        max={max}
        animation={false}
        onChartClick={onChartClick}
      />
      <div>{items}</div>
    </div>)
}

export const CoverageTrackerList = (): React.JSX.Element => {
  const data = useLoaderData() as { trackerName: string, repo: Repo, coverages: Coverage[] }
  const params = useParams()
  const [range, setRange] = useState<TimeRangeKey>('all')
  const { min, max } = computeDateRange(range)
  return (
    <div>
      <h2 className="text-3xl my-4">{data.trackerName}</h2>
      <CoverageListContent
        repo={data.repo} coverages={data.coverages} params={params} min={min} max={max}
        rangeSelector={<TimeRangeSelector value={range} onChange={setRange} />}
      />
    </div>
  )
}

export const coverageTrackerRoute = [
  {
    index: true,
    element: <CoverageTrackerList />,
    loader: loadCoverageListByTracker,
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
            link: `/coverages/${params.trackerId}/${params.index}/${params.entry}`
          }),
        },
        children: [
          {
            index: true,
            element: <CoverageEntryPage />,
            loader: loadCoverageEntryByTracker,
          },
          {
            path: '*',
            element: <FilePage />,
            loader: loadFileByTracker,
            handle: {
              crumb: (params: Params) => ({ label: params["*"] })
            },
          },
        ],
      },
    ],
  },
]

export function makeCoverageSeries(coverages: Coverage[]) {
  const map: { [name: string]: Point[] } = {}

  const hasMultiEntries = coverages.reduce(
    (flag: boolean, cov: Coverage) => flag || cov.entries.length > 1,
    false
  )
  if (hasMultiEntries) {
    map.total = []
  }

  for (const cov of coverages) {
    for (const e of cov.entries) {
      if (!(e.name in map)) {
        map[e.name] = []
      }
      map[e.name].push(
        { x: cov.time, y: e.lines === 0 ? 0 : e.hits * 100.0 / e.lines, index: cov.index }
      )
    }
    if (hasMultiEntries) {
      map.total.push(
        { x: cov.time, y: cov.lines === 0 ? 0 : cov.hits * 100.0 / cov.lines, index: cov.index }
      )
    }
  }

  const series = []
  for (const k in map) {
    const name = k === '_default' ? 'coverage' : k
    series.push({
      name,
      type: 'line' as const,
      data: map[k].map(p => ({ value: [p.x, p.y], index: p.index })),
    })
  }

  return series
}

export function coverageToDatasets(coverages: Coverage[]): Dataset[] {
  const map: { [name: string]: Array<{ x: string; y: string; extra?: Record<string, any> }> } = {}

  const hasMultiEntries = coverages.reduce(
    (flag: boolean, cov: Coverage) => flag || cov.entries.length > 1,
    false
  )
  if (hasMultiEntries) {
    map.total = []
  }

  for (const cov of coverages) {
    for (const e of cov.entries) {
      if (!(e.name in map)) {
        map[e.name] = []
      }
      const y = e.lines === 0 ? 0 : e.hits * 100.0 / e.lines
      map[e.name].push({ x: cov.time, y: y.toFixed(1), extra: { index: cov.index, entryName: e.name } })
    }
    if (hasMultiEntries) {
      const y = cov.lines === 0 ? 0 : cov.hits * 100.0 / cov.lines
      map.total.push({ x: cov.time, y: y.toFixed(1), extra: { index: cov.index, entryName: 'total' } })
    }
  }

  const datasets: Dataset[] = []
  for (const k in map) {
    const label = k === '_default' ? 'coverage' : k
    datasets.push({
      label,
      data: map[k],
      seriesConfig: { value_format: '%.1f%%' },
    })
  }

  return datasets
}


