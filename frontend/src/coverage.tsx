import { DateTime, Duration } from 'luxon'
import React, { useEffect, useState } from 'react'
import Datepicker from "react-tailwindcss-datepicker";
import {
  LoaderFunctionArgs,
  Params,
  redirect,
  useLoaderData,
  useParams,
} from 'react-router-dom'

import { Coverage, CoverageEntry, FileData, Repo } from './core'
import { CoverageChart } from './coverage_chart'
import { Browser } from './browser'
import { CodeView } from './codeview'
import { DefaultLink, ExternalLink } from './util'

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

interface CodeData {
  repo: Repo
  filename: string
  code: string
  blocks: any
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
  return (hits * 100.0 / lines).toFixed(1)
}

// returns CodeData
async function loadFile({ params }: any): Promise<Response> {
  // console.log(params)
  const url = `/api/${makeEntryPath(params)}/files/${params["*"]}`
  const resp = await fetch(url)
  if (!resp.ok)
    throw resp
  console.log(resp)
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
  // console.log(params);
  const url = `/api/${makeEntryPath(params)}/files`
  const resp = await fetch(url)
  // console.log(resp)
  if (resp.status == 403) {
    return redirect("/scms")
  }
  if (!resp.ok)
    throw resp
  return resp
}

const CoverageEntryPage = (): React.JSX.Element => {
  const resp = useLoaderData() as CoverageEntryData
  // console.log(resp)
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
            <ExternalLink href="{meta.revision_url}">
              {formatRevision(meta.revision)}
            </ExternalLink>
          </span>
          {formatTime(meta.time)}
        </div>
      </div>
      <Browser files={resp.files} />
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
    return redirect("/scms")
  }
  if (!resp.ok)
    throw resp

  const data = await resp.json()

  const coverages = data.coverages

  // const coverages = await resp.json() as Coverage[]

  // preprocess
  coverages.sort((a: Coverage, b: Coverage) => {
    return b.index - a.index
  })

  for (const cov of coverages) {
    let hits = 0; let lines = 0

    cov.entries.sort((a: CoverageEntry, b: CoverageEntry) => {
      return a.name < b.name ? -1 : 1
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


const CoverageSegment = (props: CoverageSegmentProperty): React.JSX.Element => {
  const params = props.params
  const cov = props.cov

  const elems: React.JSX.Element[] = []

  if (cov.entries.length > 1) { // Add "Total"
    elems.push(
      <span>
        Total {formatRatio(cov.hits, cov.lines)}% ({cov.hits}/{cov.lines})
      </span>)
  }

  cov.entries.forEach((e: any, i: number) => {
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
    <div className="border-2 rounded my-2 p-2 flex justify-between">
      <div>
        <span className="mr-2">#{cov.index}</span>
        {elemsWithMargin}
      </div>
      <div>
        <span className="mr-2">{formatTime(cov.time)}</span>
        <ExternalLink href={cov.revision_url}>
          {formatRevision(cov.revision)}
        </ExternalLink>
      </div>
    </div>)
}

const CoverageList = (): React.JSX.Element => {
  const data = useLoaderData() as { repo: Repo, coverages: Coverage[] }
  // console.log(document.location)
  const params = useParams()
  const repo = data.repo

  // console.log(data.coverages)

  const [min, setMin] = useState<DateTime | null>(null);

  const items: React.JSX.Element[] = []
  data.coverages.forEach((cov: Coverage, i: number) => {
    items.push(<div key={i}>
      <CoverageSegment cov={cov} params={params} />
    </div>)
  })

  const [startDate, setStartDate] = useState<Date | null>(null);
  const [endDate, setEndDate] = useState<Date | null>(null)

  const onStartDateChange = (date: Date | null) => { setStartDate(date) }
  const onEndDateChange = (date: Date | null) => { setEndDate(date) }

  return (
    <div><h2 className="text-3xl my-4">Coverages</h2>
      Repository: <ExternalLink href={repo.url}>{repo.url}</ExternalLink>
      <div className="pt-2 flex">
        From
        <Datepicker
          containerClassName="relative w-1/4"
          value={{ startDate: startDate, endDate: startDate }}
          onChange={(range) => { if (range) { onStartDateChange(range.startDate) } }}
          useRange={false}
          asSingle={true}
        />
        <span className="px-2">To</span>
        <Datepicker
          containerClassName="relative w-1/4"
          value={{ startDate: endDate, endDate: endDate }}
          onChange={(range) => { if (range) { onEndDateChange(range.startDate) } }}
          useRange={false}
          asSingle={true}
        />
      </div>
      <CoverageChart coverages={data.coverages} min={startDate} max={endDate} />
      <div>{items}</div>
    </div>)
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


