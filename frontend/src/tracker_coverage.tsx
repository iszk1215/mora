import React, { useEffect, useState } from 'react'
import { useLoaderData } from 'react-router'

import { Coverage, CoverageEntry, Repo, TrackerResponse } from './core'
import { CoverageListContent } from './coverage'
import { TimeRangeSelector, computeDateRange } from './time_range'
import type { TimeRangeKey } from './time_range'

interface SeriesModel {
  id: number
  tracker_id: number
  name: string
  data_type: string
}

interface TrackerDetailData {
  tracker: TrackerResponse
  series: SeriesModel[]
}

export const CoverageTrackerDetail = (): React.JSX.Element => {
  const data = useLoaderData() as TrackerDetailData
  const { tracker } = data

  const [coverages, setCoverages] = useState<Coverage[]>([])
  const [repo, setRepo] = useState<Repo | null>(null)

  useEffect(() => {
    if (tracker.repo_id == null) return
    fetch(`/api/repos/${tracker.repo_id}/coverages`)
      .then((r) => r.json())
      .then((d: { repo: Repo; coverages: Coverage[] }) => {
        d.coverages.sort((a: Coverage, b: Coverage) => b.index - a.index)
        for (const cov of d.coverages) {
          let hits = 0; let lines = 0
          cov.entries.sort((a: CoverageEntry, b: CoverageEntry) => a.name.localeCompare(b.name))
          for (const e of cov.entries) {
            hits += e.hits
            lines += e.lines
          }
          cov.hits = hits
          cov.lines = lines
        }
        setCoverages(d.coverages)
        setRepo(d.repo)
        return undefined
      })
      .catch(() => {})
  }, [tracker.repo_id])

  const params = tracker.repo_id != null ? { repo_id: String(tracker.repo_id) } : { repo_id: '' }
  const [range, setRange] = useState<TimeRangeKey>('all')
  const { min, max } = computeDateRange(range)

  return repo && coverages.length > 0
    ? (
      <div>
        <h2 className="text-3xl my-4">{tracker.name}</h2>
        <CoverageListContent
          repo={repo} coverages={coverages} params={params} min={min} max={max}
          rangeSelector={<TimeRangeSelector value={range} onChange={setRange} />}
        />
      </div>
    )
    : <p className="text-muted-foreground">No coverage data</p>
}
