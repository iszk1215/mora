import React, { useEffect, useState } from 'react'
import { useLoaderData } from 'react-router'

import { Coverage, CoverageEntry, Repo, TrackerResponse } from './core'
import { CoverageListContent } from './coverage'

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

  return repo && coverages.length > 0
    ? <CoverageListContent repo={repo} coverages={coverages} params={params} />
    : <p className="text-muted-foreground">No coverage data</p>
}
