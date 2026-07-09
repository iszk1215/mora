import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import DatePicker from 'react-datepicker'

import ReactECharts from 'echarts-for-react'

import {
  Link,
  LoaderFunctionArgs,
  useLoaderData,
  useNavigate,
} from 'react-router'

import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ChartConfig, TrackerResponse } from './core'

interface SeriesModel {
  id: number
  tracker_id: number
  name: string
  data_type: string
}

interface ValueModel {
  time: string
  value: number
}

interface SeriesValues {
  series: SeriesModel
  values: ValueModel[]
}

interface Dataset {
  data: Array<{ x: string; y: string }>
  label: string
}

interface TrackerChartProps {
  data?: { datasets: Dataset[] }
  min?: Date | null
  max?: Date | null
  chartConfig?: ChartConfig | null
}

interface TrackerDetailData {
  tracker: TrackerResponse
  series: SeriesModel[]
}

interface PaginatedTrackers {
  trackers: TrackerResponse[]
  total: number
  page: number
  per_page: number
}

interface PreviewData {
  tracker: TrackerResponse
  series: Array<{
    series: SeriesModel
    values: ValueModel[]
  }>
}

async function listTrackers(page?: number, perPage?: number): Promise<PaginatedTrackers> {
  const params = new URLSearchParams()
  if (page) params.set('page', String(page))
  if (perPage) params.set('per_page', String(perPage))
  const qs = params.toString()
  const url = qs ? `/api/tracker?${qs}` : '/api/tracker'
  const resp = await fetch(url)
  if (!resp.ok) throw resp
  return resp.json()
}

async function fetchPreview(trackerId: number): Promise<PreviewData> {
  const resp = await fetch(`/api/tracker/${trackerId}/preview`)
  if (!resp.ok) throw resp
  return resp.json()
}
async function createTracker(name: string, visibility: string, type_?: string, repoId?: number): Promise<TrackerResponse> {
  const body: Record<string, unknown> = { name, visibility }
  if (type_) body.type = type_
  if (repoId !== undefined) body.repo_id = repoId
  const resp = await fetch('/api/tracker', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

async function listSeries(trackerId: number): Promise<TrackerDetailData> {
  const resp = await fetch(`/api/tracker/${trackerId}/series`)
  if (!resp.ok) throw resp
  return resp.json()
}

async function createSeries(trackerId: number, name: string, dataType: string): Promise<SeriesModel> {
  const resp = await fetch(`/api/tracker/${trackerId}/series`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, data_type: dataType }),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

async function deleteSeries(trackerId: number, seriesId: number): Promise<void> {
  const resp = await fetch(`/api/tracker/${trackerId}/series/${seriesId}`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

async function createValue(trackerId: number, seriesId: number, time: string, value: number): Promise<void> {
  const resp = await fetch(`/api/tracker/${trackerId}/series/${seriesId}/values`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ time, value }),
  })
  if (!resp.ok) throw resp
}

async function deleteValues(trackerId: number, seriesId: number): Promise<void> {
  const resp = await fetch(`/api/tracker/${trackerId}/series/${seriesId}/values`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

async function likeTracker(trackerId: number): Promise<void> {
  const resp = await fetch(`/api/tracker/${trackerId}/like`, { method: 'POST' })
  if (!resp.ok) throw resp
}

async function unlikeTracker(trackerId: number): Promise<void> {
  const resp = await fetch(`/api/tracker/${trackerId}/like`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

export async function patchTracker(trackerId: number, opts: { visibility?: string; chart_config?: string }): Promise<TrackerResponse> {
  const resp = await fetch(`/api/tracker/${trackerId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

export async function loadTrackerList(): Promise<PaginatedTrackers> {
  return listTrackers(1, 12)
}

export async function loadTrackerDetail({ params }: LoaderFunctionArgs): Promise<TrackerDetailData> {
  if (!params.trackerId) throw new Error('trackerId is required')
  return listSeries(parseInt(params.trackerId))
}

const TrackerChart = (params: TrackerChartProps): React.JSX.Element => {
  const datasets = params.data?.datasets ?? []
  const cc = params.chartConfig
  const dataZoomAdded = useRef(false)

  const option = useMemo(() => {
    const showLegend = cc?.show_legend !== false && datasets.length > 1
    const opt: any = {
      grid: { left: 60, right: 20, top: showLegend ? 40 : 20, bottom: 60 },
      xAxis: {
        type: 'time' as const,
        splitLine: { show: false },
      },
      yAxis: {
        type: 'value' as const,
        splitLine: { lineStyle: { type: 'dashed' as const, opacity: 0.3 } },
      },
      series: datasets.map((ds) => ({
        name: ds.label,
        type: 'line' as const,
        data: ds.data.map((p) => [p.x, Number(p.y)]),
        areaStyle: cc?.area !== false ? { opacity: 0.12 } : undefined,
      })),
      tooltip: {
        trigger: 'axis',
        valueFormatter: (value: number) =>
          Number.isInteger(value) ? String(value) : value.toFixed(1),
      },
    }
    if (!dataZoomAdded.current) {
      opt.dataZoom = [
        { type: 'inside' as const, xAxisIndex: 0 },
        { type: 'slider' as const, xAxisIndex: 0, bottom: 10 },
      ]
    }
    if (showLegend) {
      opt.legend = { type: 'scroll' as const, top: 0 }
    }
    if (cc) {
      if (cc.x_axis_label) opt.xAxis.name = cc.x_axis_label
      if (cc.y_axis_label) opt.yAxis.name = cc.y_axis_label
      if (cc.y_max !== undefined && cc.y_max > 0) opt.yAxis.max = cc.y_max
    }
    if (params.min) opt.xAxis.min = params.min
    if (params.max) opt.xAxis.max = params.max
    return opt
  }, [datasets, cc, params.min, params.max])

  useEffect(() => {
    dataZoomAdded.current = true
  }, [])

  return (
    <ReactECharts
      option={option}
      style={{ width: '100%', height: 300 }}
    />
  )
}

const TrackerCard = ({ tracker, preview, loading }: { tracker: TrackerResponse; preview?: PreviewData; loading?: boolean }): React.JSX.Element => {
  const option = useMemo(() => {
    const datasets = preview?.series?.map((sv) => ({
      label: sv.series.name,
      data: sv.values.map((v) => ({ x: v.time, y: String(v.value) })),
    })) ?? []

    return {
      animation: false,
      grid: { left: 50, right: 10, top: 10, bottom: 25 },
      xAxis: {
        type: 'time' as const,
        axisLabel: { hideOverlap: true },
      },
      yAxis: { type: 'value' as const },
      series: datasets.map((ds) => ({
        name: ds.label,
        type: 'line' as const,
        data: ds.data.map((p) => [p.x, Number(p.y)]),
        lineStyle: { width: 1.5 },
        symbol: 'none',
        areaStyle: { opacity: 0.1 },
      })),
      tooltip: { trigger: 'axis' as const },
    }
  }, [preview])

  const linkTo = tracker.type === 'coverage' && tracker.repo_id != null
    ? `/repos/${tracker.repo_id}/coverages`
    : `/tracker/${tracker.id}`

  return (
    <Link to={linkTo} className="block border rounded-lg p-4 hover:shadow-md transition-shadow">
      <div className="flex items-center gap-2 mb-2">
        <h3 className="font-semibold text-lg truncate">{tracker.name}</h3>
        {tracker.role && (
          <span className="text-xs bg-primary/10 text-primary px-1.5 py-0.5 rounded">{tracker.role}</span>
        )}
        {tracker.type === 'coverage' && (
          <span className="text-xs bg-green-100 text-green-700 px-1.5 py-0.5 rounded">Coverage</span>
        )}
      </div>
      <div className="h-[120px]">
        {loading ? (
          <div className="flex items-center justify-center h-full text-muted-foreground text-sm">Loading...</div>
        ) : option.series.length > 0 ? (
          <ReactECharts option={option} style={{ width: '100%', height: 120 }} />
        ) : (
          <div className="flex items-center justify-center h-full text-muted-foreground text-sm">No data</div>
        )}
      </div>
    </Link>
  )
}

export const TrackerView = (): React.JSX.Element => {
  const initial = useLoaderData() as PaginatedTrackers
  const [trackers, setTrackers] = useState<TrackerResponse[]>(initial.trackers)
  const [page, setPage] = useState(initial.page)
  const [total, setTotal] = useState(initial.total)
  const perPage = initial.per_page
  const totalPages = perPage > 0 ? Math.ceil(total / perPage) : 1

  const [previews, setPreviews] = useState<Map<number, PreviewData>>(new Map())
  const [loadingPreviews, setLoadingPreviews] = useState(false)
  const [loadingPage, setLoadingPage] = useState(false)

  const loadPreviews = useCallback(async () => {
    if (trackers.length === 0) {
      setPreviews(new Map())
      return
    }
    setLoadingPreviews(true)
    try {
      const results = await Promise.all(
        trackers.map((t) =>
          fetchPreview(t.id).then((data) => ({ id: t.id, data } as const)),
        ),
      )
      const map = new Map<number, PreviewData>()
      results.forEach((r) => map.set(r.id, r.data))
      setPreviews(map)
    } catch {
      // ignore individual preview failures
    } finally {
      setLoadingPreviews(false)
    }
  }, [trackers])

  useEffect(() => {
    void loadPreviews()
  }, [loadPreviews])

  const handlePageChange = async (newPage: number) => {
    setLoadingPage(true)
    try {
      const data = await listTrackers(newPage, perPage)
      setTrackers(data.trackers)
      setPage(data.page)
      setTotal(data.total)
    } catch {
      // ignore
    } finally {
      setLoadingPage(false)
    }
  }

  return (
    <div>
      <div className="mb-4">
        <Button asChild><Link to="/tracker/new">Create Tracker</Link></Button>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {trackers.map((t) => (
          <TrackerCard key={t.id} tracker={t} preview={previews.get(t.id)} loading={loadingPreviews} />
        ))}
        {trackers.length === 0 && !loadingPage && (
          <p className="col-span-full text-center text-muted-foreground py-8">No trackers yet.</p>
        )}
      </div>

      {totalPages > 1 && (
        <div className="flex justify-center items-center gap-2 mt-6">
          <Button variant="outline" size="sm" disabled={page <= 1 || loadingPage} onClick={() => handlePageChange(page - 1)}>
            Prev
          </Button>
          <span className="text-sm text-muted-foreground">
            Page {page} of {totalPages}
          </span>
          <Button variant="outline" size="sm" disabled={page >= totalPages || loadingPage} onClick={() => handlePageChange(page + 1)}>
            Next
          </Button>
        </div>
      )}
    </div>
  )
}

export const TrackerDetailView = (): React.JSX.Element => {
  const data = useLoaderData() as TrackerDetailData
  const { tracker } = data
  const [seriesList] = useState<SeriesModel[]>(data.series)
  const [seriesValues, setSeriesValues] = useState<SeriesValues[]>([])
  const [liked, setLiked] = useState(tracker.liked)
  const [likeLoading, setLikeLoading] = useState(false)

  const [startDate, setStartDate] = useState<Date | null>(null)
  const [endDate, setEndDate] = useState<Date | null>(null)

  const [coverageTimeline, setCoverageTimeline] = useState<Array<{ time: string; value: number; entry_name: string }>>([])

  const coverageZoomAdded = useRef(false)

  const isCoverage = tracker.type === 'coverage'

  useEffect(() => {
    coverageZoomAdded.current = true
  }, [])

  useEffect(() => {
    if (isCoverage && tracker.repo_id != null) {
      fetch(`/api/repos/${tracker.repo_id}/coverage/timeline?limit=30`)
        .then((r) => r.json())
        .then((d: { timeline: Array<{ time: string; value: number; entry_name: string }> }) => {
          setCoverageTimeline(d.timeline ?? [])
          return undefined
        })
        .catch(() => {})
    }
  }, [isCoverage, tracker.repo_id])

  useEffect(() => {
    Promise.all(
      seriesList.map((s) =>
        fetch(`/api/tracker/${tracker.id}/series/${s.id}/values`)
          .then((r) => r.json() as Promise<{ values: ValueModel[] }>)
          .then((d) => ({ series: s, values: d.values ?? [] }))
      )
    ).then(setSeriesValues).catch(() => {})
  }, [seriesList, tracker.id])

  const handleLikeToggle = async () => {
    setLikeLoading(true)
    try {
      if (liked) {
        await unlikeTracker(tracker.id)
        setLiked(false)
      } else {
        await likeTracker(tracker.id)
        setLiked(true)
      }
    } catch {
      // ignore
    } finally {
      setLikeLoading(false)
    }
  }

  const valuesToDataset = (sv: SeriesValues) => ({
    data: sv.values.map((v: ValueModel) => ({ x: v.time, y: String(v.value) })),
    label: sv.series.name,
  })

  const datasets: Dataset[] = useMemo(() => seriesValues.map(valuesToDataset), [seriesValues])

  const viewChartConfig = useMemo<ChartConfig | null>(() => {
    try {
      return JSON.parse(tracker.chart_config) as ChartConfig
    } catch {
      return null
    }
  }, [tracker.chart_config])

  const coverageOption = useMemo(() => {
    if (!isCoverage) return null
    const entryNames = [...new Set(coverageTimeline.map((p) => p.entry_name))]
    const opt: any = {
      grid: { left: 60, right: 20, top: 30, bottom: 60 },
      xAxis: {
        type: 'time' as const,
        splitLine: { show: false },
      },
      yAxis: {
        type: 'value' as const,
        min: 0, max: 100,
        axisLabel: { formatter: '{value}%' },
        splitLine: { lineStyle: { type: 'dashed' as const, opacity: 0.3 } },
      },
      series: entryNames.map((name) => ({
        name,
        type: 'line' as const,
        data: coverageTimeline
          .filter((p) => p.entry_name === name)
          .map((p) => [p.time, p.value]),
        areaStyle: { opacity: 0.12 },
      })),
      tooltip: {
        trigger: 'axis' as const,
        valueFormatter: (value: number) => value.toFixed(1) + '%',
      },
    }
    if (!coverageZoomAdded.current) {
      opt.dataZoom = [
        { type: 'inside' as const, xAxisIndex: 0 },
        { type: 'slider' as const, xAxisIndex: 0, bottom: 10 },
      ]
    }
    return opt
  }, [isCoverage, coverageTimeline])

  return (
    <div>
      <div className="flex items-center gap-3 my-4">
        <h1 className="text-3xl">{tracker.name}</h1>
        {tracker.type === 'coverage' && (
          <span className="text-xs bg-green-100 text-green-700 px-1.5 py-0.5 rounded">Coverage</span>
        )}
        <Button
          variant={liked ? 'default' : 'secondary'}
          size="sm"
          onClick={handleLikeToggle}
          disabled={likeLoading}
        >
          {liked ? 'Unlike' : 'Like'}
        </Button>
        {tracker.role !== '' && !isCoverage && (
          <Button variant="outline" size="sm" asChild>
            <Link to={`/tracker/${tracker.id}/edit`}>Edit</Link>
          </Button>
        )}
      </div>

      {isCoverage ? (
        <>
          <h2 className="text-xl my-2">Coverage Timeline</h2>
          {coverageTimeline.length > 0 ? (
            <ReactECharts option={coverageOption} style={{ width: '100%', height: 300 }} />
          ) : (
            <p className="text-muted-foreground">No coverage data</p>
          )}
          {tracker.repo_id != null && (
            <div className="mt-4">
              <Button asChild>
                <Link to={`/repos/${tracker.repo_id}/coverages`}>View Coverage Details</Link>
              </Button>
            </div>
          )}
        </>
      ) : (
        <>
          <div className="pt-2 flex items-center mb-2">
            <span className="mr-1">From</span>
            <div className="w-1/4">
              <DatePicker
                selected={startDate}
                onChange={(d: Date | null) => setStartDate(d)}
                className="border rounded px-2 py-1 w-full"
                placeholderText="Select date"
                dateFormat="yyyy-MM-dd"
              />
            </div>
            <span className="px-2">To</span>
            <div className="w-1/4">
              <DatePicker
                selected={endDate}
                onChange={(d: Date | null) => setEndDate(d)}
                className="border rounded px-2 py-1 w-full"
                placeholderText="Select date"
                dateFormat="yyyy-MM-dd"
              />
            </div>
          </div>

          {datasets.length > 0 ? (
            <TrackerChart data={{ datasets }} min={startDate} max={endDate} chartConfig={viewChartConfig} />
          ) : (
            <p className="text-muted-foreground">No data to display</p>
          )}
        </>
      )}
    </div>
  )
}

export const TrackerDetailEdit = (): React.JSX.Element => {
  const data = useLoaderData() as TrackerDetailData
  const { tracker } = data
  const [seriesList, setSeriesList] = useState<SeriesModel[]>(data.series)
  const [seriesValues, setSeriesValues] = useState<SeriesValues[]>([])
  const [newSeriesName, setNewSeriesName] = useState('')
  const [newSeriesDataType, setNewSeriesDataType] = useState('float')

  const [selectedSeriesId, setSelectedSeriesId] = useState<number | null>(null)
  const [newValueTime, setNewValueTime] = useState('')
  const [newValueNumber, setNewValueNumber] = useState('')

  const [startDate, setStartDate] = useState<Date | null>(null)
  const [endDate, setEndDate] = useState<Date | null>(null)

  const [savedChartConfig, setSavedChartConfig] = useState(tracker.chart_config)
  const parsedChartConfig = useMemo<ChartConfig>(() => {
    try {
      return JSON.parse(savedChartConfig) as ChartConfig
    } catch {
      return {}
    }
  }, [savedChartConfig])
  const [visibility, setVisibility] = useState(tracker.visibility)
  const [xLabel, setXLabel] = useState(parsedChartConfig.x_axis_label ?? '')
  const [yLabel, setYLabel] = useState(parsedChartConfig.y_axis_label ?? '')
  const [area, setArea] = useState(parsedChartConfig.area ?? true)
  const [showLegend, setShowLegend] = useState(parsedChartConfig.show_legend ?? true)
  const [yMax, setYMax] = useState(parsedChartConfig.y_max ? String(parsedChartConfig.y_max) : '')

  const isCoverage = tracker.type === 'coverage'

  const handleVisibilityChange = async (newVisibility: string) => {
    try {
      await patchTracker(tracker.id, { visibility: newVisibility })
      setVisibility(newVisibility)
    } catch {
      // ignore
    }
  }

  const handleChartConfigSave = async () => {
    const cc: ChartConfig = {}
    if (xLabel.trim()) cc.x_axis_label = xLabel.trim()
    if (yLabel.trim()) cc.y_axis_label = yLabel.trim()
    if (!area) cc.area = false
    if (!showLegend) cc.show_legend = false
    if (yMax.trim()) {
      const parsed = parseFloat(yMax.trim())
      if (parsed > 0) cc.y_max = parsed
    }
    try {
      const updated = await patchTracker(tracker.id, { chart_config: JSON.stringify(cc) })
      setSavedChartConfig(updated.chart_config)
    } catch {
      // ignore
    }
  }

  useEffect(() => {
    Promise.all(
      seriesList.map((s) =>
        fetch(`/api/tracker/${tracker.id}/series/${s.id}/values`)
          .then((r) => r.json() as Promise<{ values: ValueModel[] }>)
          .then((d) => ({ series: s, values: d.values ?? [] }))
      )
    ).then(setSeriesValues).catch(() => {})
  }, [seriesList, tracker.id])

  const handleCreateSeries = async () => {
    if (!newSeriesName.trim()) return
    try {
      const created = await createSeries(tracker.id, newSeriesName.trim(), newSeriesDataType)
      setSeriesList((prev) => [...prev, created])
      setNewSeriesName('')
    } catch {
      // ignore
    }
  }

  const handleDeleteSeries = async (seriesId: number) => {
    try {
      await deleteSeries(tracker.id, seriesId)
      setSeriesList((prev) => prev.filter((s) => s.id !== seriesId))
      setSeriesValues((prev) => prev.filter((sv) => sv.series.id !== seriesId))
    } catch {
      // ignore
    }
  }

  const handleAddValue = async () => {
    if (selectedSeriesId === null || !newValueTime || !newValueNumber) return
    try {
      await createValue(tracker.id, selectedSeriesId, new Date(newValueTime).toISOString(), parseFloat(newValueNumber))
      setNewValueTime('')
      setNewValueNumber('')
      const resp = await fetch(`/api/tracker/${tracker.id}/series/${selectedSeriesId}/values`)
      const data = await resp.json()
      setSeriesValues((prev) =>
        prev.map((sv) =>
          sv.series.id === selectedSeriesId ? { ...sv, values: data.values ?? [] } : sv
        )
      )
    } catch {
      // ignore
    }
  }

  const handleDeleteValues = async (seriesId: number) => {
    try {
      await deleteValues(tracker.id, seriesId)
      setSeriesValues((prev) =>
        prev.map((sv) =>
          sv.series.id === seriesId ? { ...sv, values: [] } : sv
        )
      )
    } catch {
      // ignore
    }
  }

  const valuesToDataset = (sv: SeriesValues) => ({
    data: sv.values.map((v: ValueModel) => ({ x: v.time, y: String(v.value) })),
    label: sv.series.name,
  })

  const datasets: Dataset[] = useMemo(() => seriesValues.map(valuesToDataset), [seriesValues])

  const chartConfigForChart = useMemo<ChartConfig | null>(() => {
    try {
      return JSON.parse(savedChartConfig) as ChartConfig
    } catch {
      return null
    }
  }, [savedChartConfig])

  if (isCoverage) {
    return (
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Link to={`/tracker/${tracker.id}`} className="text-blue-600 dark:text-blue-500 hover:underline">
            &larr; Back to Tracker
          </Link>
        </div>

        <h1 className="text-3xl my-4">{tracker.name} (Edit)</h1>

        <div className="bg-yellow-50 border border-yellow-200 rounded p-4 mb-6">
          <p className="text-yellow-800">
            This tracker is linked to coverage data and cannot be edited directly.
          </p>
        </div>

        {/* Visibility */}
        <h2 className="text-xl my-2">Visibility</h2>
        <select
          value={visibility}
          onChange={(e) => handleVisibilityChange(e.target.value)}
          className="border rounded px-2 py-1 mb-4"
        >
          <option value="private">Private</option>
          <option value="unlisted">Unlisted</option>
          <option value="public">Public</option>
        </select>
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center gap-2 mb-4">
        <Link to={`/tracker/${tracker.id}`} className="text-blue-600 dark:text-blue-500 hover:underline">
          &larr; Back to Tracker
        </Link>
      </div>

      <h1 className="text-3xl my-4">{tracker.name} (Edit)</h1>

      {/* Series list */}
      <h2 className="text-xl my-2">Series</h2>

      <div className="flex items-center gap-2 mb-4">
        <input
          type="text"
          value={newSeriesName}
          onChange={(e) => setNewSeriesName(e.target.value)}
          placeholder="Series name"
          className="border rounded px-2 py-1"
          onKeyDown={(e) => { if (e.key === 'Enter') handleCreateSeries() }}
        />
        <select
          value={newSeriesDataType}
          onChange={(e) => setNewSeriesDataType(e.target.value)}
          className="border rounded px-2 py-1"
        >
          <option value="float">float</option>
          <option value="int">int</option>
        </select>
        <Button onClick={handleCreateSeries} disabled={!newSeriesName.trim()}>Add Series</Button>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Data Type</TableHead>
            <TableHead className="w-48">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {seriesList.length === 0 ? (
            <TableRow>
              <TableCell colSpan={3} className="text-center text-muted-foreground">
                No series yet
              </TableCell>
            </TableRow>
          ) : (
            seriesList.map((s) => (
              <TableRow key={s.id}>
                <TableCell>
                  <button
                    className="text-blue-600 dark:text-blue-500 hover:underline"
                    onClick={() => setSelectedSeriesId(s.id)}
                  >
                    {s.name}
                  </button>
                </TableCell>
                <TableCell>{s.data_type}</TableCell>
                <TableCell className="flex gap-2">
                  <Button variant="destructive" size="sm" onClick={() => handleDeleteSeries(s.id)}>
                    Delete
                  </Button>
                  <Button variant="secondary" size="sm" onClick={() => handleDeleteValues(s.id)}>
                    Clear Values
                  </Button>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>

      {/* Chart */}
      <h2 className="text-xl my-2">Chart</h2>

      <div className="pt-2 flex items-center mb-2">
        <span className="mr-1">From</span>
        <div className="w-1/4">
          <DatePicker
            selected={startDate}
            onChange={(d: Date | null) => setStartDate(d)}
            className="border rounded px-2 py-1 w-full"
            placeholderText="Select date"
            dateFormat="yyyy-MM-dd"
          />
        </div>
        <span className="px-2">To</span>
        <div className="w-1/4">
          <DatePicker
            selected={endDate}
            onChange={(d: Date | null) => setEndDate(d)}
            className="border rounded px-2 py-1 w-full"
            placeholderText="Select date"
            dateFormat="yyyy-MM-dd"
          />
        </div>
      </div>

      {datasets.length > 0 ? (
        <TrackerChart data={{ datasets }} min={startDate} max={endDate} chartConfig={chartConfigForChart} />
      ) : (
        <p className="text-muted-foreground">No data to display</p>
      )}

      {/* Chart Options */}
      <h2 className="text-xl my-2">Chart Options</h2>
      <div className="flex flex-wrap items-center gap-3 mb-4">
        <input
          type="text"
          value={xLabel}
          onChange={(e) => setXLabel(e.target.value)}
          placeholder="X-axis label"
          className="border rounded px-2 py-1 w-40"
        />
        <input
          type="text"
          value={yLabel}
          onChange={(e) => setYLabel(e.target.value)}
          placeholder="Y-axis label"
          className="border rounded px-2 py-1 w-40"
        />
        <label className="flex items-center gap-1 text-sm">
          <input type="checkbox" checked={area} onChange={(e) => setArea(e.target.checked)} />
          Area
        </label>
        <label className="flex items-center gap-1 text-sm">
          <input type="checkbox" checked={showLegend} onChange={(e) => setShowLegend(e.target.checked)} />
          Legend
        </label>
        <input
          type="number"
          value={yMax}
          onChange={(e) => setYMax(e.target.value)}
          placeholder="Y-axis max"
          className="border rounded px-2 py-1 w-28"
          min="0"
        />
        <Button onClick={handleChartConfigSave}>Save Chart Options</Button>
      </div>

      {/* Add value form */}
      <h2 className="text-xl my-2">Add Value</h2>

      <div className="flex items-center gap-2 mb-4">
        <select
          value={selectedSeriesId ?? ''}
          onChange={(e) => setSelectedSeriesId(e.target.value ? parseInt(e.target.value) : null)}
          className="border rounded px-2 py-1"
        >
          <option value="">Select series</option>
          {seriesList.map((s) => (
            <option key={s.id} value={s.id}>{s.name}</option>
          ))}
        </select>
        <input
          type="datetime-local"
          value={newValueTime}
          onChange={(e) => setNewValueTime(e.target.value)}
          className="border rounded px-2 py-1"
        />
        <input
          type="number"
          value={newValueNumber}
          onChange={(e) => setNewValueNumber(e.target.value)}
          placeholder="Value"
          className="border rounded px-2 py-1 w-32"
        />
        <Button
          onClick={handleAddValue}
          disabled={selectedSeriesId === null || !newValueTime || !newValueNumber}
        >
          Add
        </Button>
      </div>

      {/* Visibility */}
      <h2 className="text-xl my-2">Visibility</h2>
      <select
        value={visibility}
        onChange={(e) => handleVisibilityChange(e.target.value)}
        className="border rounded px-2 py-1 mb-4"
      >
        <option value="private">Private</option>
        <option value="unlisted">Unlisted</option>
        <option value="public">Public</option>
      </select>
    </div>
  )
}

export const TrackerCreate = (): React.JSX.Element => {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [visibility, setVisibility] = useState('private')
  const [type, setType] = useState('tracker')
  const [repoId, setRepoId] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const handleCreate = async () => {
    if (!name.trim()) return
    setLoading(true)
    setError(null)
    try {
      const created = await createTracker(
        name.trim(),
        visibility,
        type,
        type === 'coverage' ? (repoId ? parseInt(repoId) : undefined) : undefined,
      )
      navigate(`/tracker/${created.id}`)
    } catch {
      setError('Failed to create tracker. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <div className="flex items-center gap-2 mb-4">
        <Link to="/tracker" className="text-blue-600 dark:text-blue-500 hover:underline">
          &larr; Back to Trackers
        </Link>
      </div>

      <h1 className="text-3xl my-4">Create Tracker</h1>

      {error && <p className="text-red-500 mb-2">{error}</p>}

      <div className="flex flex-col gap-4 max-w-md">
        <div>
          <label className="block mb-1">Name</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Tracker name"
            className="border rounded px-2 py-1 w-full"
            onKeyDown={(e) => { if (e.key === 'Enter') handleCreate() }}
            disabled={loading}
          />
        </div>

        <div>
          <label className="block mb-1">Visibility</label>
          <select
            value={visibility}
            onChange={(e) => setVisibility(e.target.value)}
            className="border rounded px-2 py-1 w-full"
            disabled={loading}
          >
            <option value="private">Private</option>
            <option value="unlisted">Unlisted</option>
            <option value="public">Public</option>
          </select>
        </div>

        <div>
          <label className="block mb-1">Type</label>
          <select
            value={type}
            onChange={(e) => setType(e.target.value)}
            className="border rounded px-2 py-1 w-full"
            disabled={loading}
          >
            <option value="tracker">Tracker</option>
            <option value="coverage">Coverage</option>
          </select>
        </div>

        {type === 'coverage' && (
          <div>
            <label className="block mb-1">Repository ID</label>
            <input
              type="number"
              value={repoId}
              onChange={(e) => setRepoId(e.target.value)}
              placeholder="Repository ID"
              className="border rounded px-2 py-1 w-full"
              disabled={loading}
            />
          </div>
        )}

        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <Link to="/tracker">Cancel</Link>
          </Button>
          <Button onClick={handleCreate} disabled={!name.trim() || loading}>
            Create
          </Button>
        </div>
      </div>
    </div>
  )
}

export const trackerRoute = [
  {
    index: true,
    element: <TrackerView />,
    loader: loadTrackerList,
  },
  {
    path: 'new',
    element: <TrackerCreate />,
  },
  {
    path: ':trackerId',
    loader: loadTrackerDetail,
    element: <TrackerDetailView />,
  },
  {
    path: ':trackerId/edit',
    loader: loadTrackerDetail,
    element: <TrackerDetailEdit />,
  },
]
