import { DateTime } from 'luxon'
import React, { useCallback, useMemo, useState } from 'react'
import {
  LoaderFunctionArgs,
  Params,
  redirect,
  useLoaderData,
  useParams,
} from 'react-router'
import { Star } from 'lucide-react'

import { ChartConfig, Coverage, CoverageEntry, FileData, Repo } from './core'
import { TrackerChart } from './chart'
import { Dataset } from './chart'
import { Browser } from './browser'
import { CodeView } from './codeview'
import { DefaultLink, ExternalLink } from './util'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableHeader, TableBody, TableHead, TableRow, TableCell } from '@/components/ui/table'

import { TimeRangeSelector, computeDateRange } from './time_range'
import type { TimeRangeKey } from './time_range'
import { useUser } from './user-context'
import { likeTracker, unlikeTracker } from './tracker'


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

export function buildCoverageClickUrl(trackerId: string | undefined, index: number, seriesName: string): string {
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
  coverages: Coverage[],
  liked: boolean,
  likeCount: number,
  trackerId: number,
  chartConfig: ChartConfig | null,
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
  let liked = false
  let likeCount = 0
  let trackerName = ''
  let chartConfig: ChartConfig | null = null
  if (trackerResp.ok) {
    const trackerData = await trackerResp.json()
    trackerName = trackerData.name
    liked = trackerData.liked ?? false
    likeCount = trackerData.like_count ?? 0
    chartConfig = trackerData.chart_config ? JSON.parse(trackerData.chart_config) : null
  }

  return {
    trackerName,
    repo: data.repo,
    coverages,
    liked,
    likeCount,
    trackerId: parseInt(params.trackerId!, 10),
    chartConfig,
  }
}

interface CoverageSegmentProperty {
  cov: Coverage,
  params: Params,
  entryNames: string[]
}


export const CoverageSegment = (props: CoverageSegmentProperty): React.JSX.Element => {
  const params = props.params
  const cov = props.cov
  const hasScmAccess = !!cov.revision_url

  const entryMap = new Map<string, CoverageEntry>()
  cov.entries.forEach((e) => entryMap.set(e.name, e))

  return (
    <TableRow>
      <TableCell>
        <Badge variant="outline">#{cov.index}</Badge>
      </TableCell>
      <TableCell>{formatRatio(cov.hits, cov.lines)}%</TableCell>
      {props.entryNames.map((name) => {
        const e = entryMap.get(name)
        if (!e) return <TableCell key={name}>-</TableCell>
        const cell = <span>{formatRatio(e.hits, e.lines)}% ({e.hits}/{e.lines})</span>
        if (!hasScmAccess) return <TableCell key={name}>{cell}</TableCell>
        return (
          <TableCell key={name}>
            <DefaultLink to={buildEntryUrl(params, cov, e.name)}>
              {cell}
            </DefaultLink>
          </TableCell>
        )
      })}
      <TableCell>{formatTime(cov.time)}</TableCell>
      <TableCell>
        {hasScmAccess ? (
          <ExternalLink href={cov.revision_url}>
            {formatRevision(cov.revision)}
          </ExternalLink>
        ) : (
          <span>{formatRevision(cov.revision)}</span>
        )}
      </TableCell>
    </TableRow>)
}

const COVERAGE_PER_PAGE = 20

export const CoverageListContent = ({ repo, coverages, params, min, max, rangeSelector, chartConfig }: {
  repo: Repo
  coverages: Coverage[]
  params: Params
  min?: Date | null
  max?: Date | null
  rangeSelector?: React.ReactNode
  chartConfig?: ChartConfig | null
}): React.JSX.Element => {
  const [page, setPage] = useState(1)
  const totalPages = Math.max(1, Math.ceil(coverages.length / COVERAGE_PER_PAGE))
  const start = (page - 1) * COVERAGE_PER_PAGE
  const pageCoverages = coverages.slice(start, start + COVERAGE_PER_PAGE)

  const entryNames = useMemo(() => {
    const nameSet = new Set<string>()
    coverages.forEach((cov) => cov.entries.forEach((e) => nameSet.add(e.name)))
    return Array.from(nameSet)
  }, [coverages])

  const items: React.JSX.Element[] = []
  pageCoverages.forEach((cov: Coverage, i: number) => {
    items.push(<CoverageSegment key={i} cov={cov} params={params} entryNames={entryNames} />)
  })

  const datasets = useMemo(() => coverageToDatasets(coverages), [coverages])

  const onChartClick = useCallback((rawParams: any) => {
    if (rawParams.seriesName !== 'total') {
      const d = rawParams.data
      if (d?.index !== undefined) {
        const url = buildCoverageClickUrl(params.trackerId, d.index, d.entryName)
        window.location.assign(url)
      }
    }
  }, [params.trackerId])

  return (
    <div>
      <div className="mb-4">
        Repository: <ExternalLink href={repo.url}>{repo.url}</ExternalLink>
      </div>
      <div className="bg-card border rounded-lg py-4 pl-2 pr-3 sm:px-4 shadow-md">
        {rangeSelector}
        <TrackerChart
          data={{ datasets }}
          chartConfig={chartConfig ?? undefined}
          min={min}
          max={max}
          animation={false}
          onChartClick={onChartClick}
        />
      </div>
      <div className="mt-4 bg-card border rounded-lg py-4 pl-2 pr-3 sm:px-4 shadow-md">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-16">Index</TableHead>
              <TableHead className="w-24">Total</TableHead>
              {entryNames.map((name) => (
                <TableHead key={name}>{name}</TableHead>
              ))}
              <TableHead className="w-48">Time</TableHead>
              <TableHead className="w-24">Revision</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items}
          </TableBody>
        </Table>
        {totalPages > 1 && (
          <div className="flex justify-center items-center gap-2 mt-4">
            <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>
              Prev
            </Button>
            <span className="text-sm text-muted-foreground">
              Page {page} of {totalPages}
            </span>
            <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
              Next
            </Button>
          </div>
        )}
      </div>
    </div>)
}

export const CoverageTrackerList = (): React.JSX.Element => {
  const data = useLoaderData() as {
    trackerName: string,
    repo: Repo,
    coverages: Coverage[],
    liked: boolean,
    likeCount: number,
    trackerId: number,
    chartConfig: ChartConfig | null,
  }
  const params = useParams()
  const user = useUser()
  const [range, setRange] = useState<TimeRangeKey>('all')
  const [liked, setLiked] = useState(data.liked)
  const [likeCount, setLikeCount] = useState(data.likeCount)
  const [likeLoading, setLikeLoading] = useState(false)
  const { min, max } = computeDateRange(range)

  const handleLikeToggle = async () => {
    setLikeLoading(true)
    try {
      if (liked) {
        await unlikeTracker(data.trackerId)
        setLiked(false)
        setLikeCount((c) => Math.max(0, c - 1))
      } else {
        await likeTracker(data.trackerId)
        setLiked(true)
        setLikeCount((c) => c + 1)
      }
    } catch {
      // ignore
    } finally {
      setLikeLoading(false)
    }
  }

  return (
    <div>
      <div className="flex items-center gap-3 my-4">
        <h2 className="text-3xl">{data.trackerName}</h2>
        <button
          type="button"
          aria-label={liked ? 'Unlike' : 'Like'}
          onClick={handleLikeToggle}
          disabled={likeLoading || !user}
          className="flex items-center gap-1 p-1.5 rounded hover:bg-accent hover:-translate-y-0.5 hover:shadow-sm transition-all disabled:opacity-50"
        >
          <Star
            className={`w-5 h-5 transition-colors ${liked ? 'text-yellow-500 fill-yellow-500' : 'text-gray-400 fill-gray-200'}`}
          />
          {likeCount > 0 && (
            <span className={`text-sm font-medium ${liked ? 'text-yellow-700' : 'text-gray-500'}`}>
              {likeCount}
            </span>
          )}
        </button>
      </div>
      <CoverageListContent
        repo={data.repo} coverages={data.coverages} params={params} min={min} max={max}
        rangeSelector={<TimeRangeSelector value={range} onChange={setRange} />}
        chartConfig={data.chartConfig}
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


