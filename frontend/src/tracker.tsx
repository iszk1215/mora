import React, { useCallback, useEffect, useMemo, useState } from 'react'

import ReactECharts from 'echarts-for-react'
import MDEditor from '@uiw/react-md-editor'
import '@uiw/react-md-editor/markdown-editor.css'
import { Star } from 'lucide-react'

import {
  Link,
  LoaderFunctionArgs,
  Params,
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
import { ChartConfig, SeriesConfig, SeriesModel, TrackerResponse, YAxisConfig } from './core'
import { formatValue, Dataset, TrackerChart, resolvePalette, areaGradient, PALETTE_NAMES, CHART_THEME_NAME } from './chart'
import { TimeRangeSelector, computeDateRange } from './time_range'
import type { TimeRangeKey } from './time_range'
import { useUser } from './user-context'

interface ValueModel {
  time: string
  value: number
}

interface SeriesValues {
  series: SeriesModel
  values: ValueModel[]
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

export interface PreviewData {
  tracker: TrackerResponse
  series: Array<{
    series: SeriesModel
    values: ValueModel[]
  }>
}

export async function listTrackers(page?: number, perPage?: number, query?: string): Promise<PaginatedTrackers> {
  const params = new URLSearchParams()
  if (page) params.set('page', String(page))
  if (perPage) params.set('per_page', String(perPage))
  if (query) params.set('q', query)
  const qs = params.toString()
  const url = qs ? `/api/trackers?${qs}` : '/api/trackers'
  const resp = await fetch(url)
  if (!resp.ok) throw resp
  return resp.json()
}

export async function fetchPreview(trackerId: number, type?: string): Promise<PreviewData> {
  const path =
    type === 'coverage'
      ? `/api/coverages/${trackerId}/preview`
      : `/api/trackers/${trackerId}/preview`
  const resp = await fetch(path)
  if (!resp.ok) throw resp
  return resp.json()
}
async function createTracker(name: string, visibility: string, description?: string): Promise<TrackerResponse> {
  const body: Record<string, unknown> = { name, visibility }
  if (description) body.description = description
  const resp = await fetch('/api/trackers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

async function listSeries(trackerId: number): Promise<TrackerDetailData> {
  const resp = await fetch(`/api/trackers/${trackerId}/series`)
  if (!resp.ok) throw resp
  return resp.json()
}

async function createSeries(trackerId: number, name: string, dataType: string, config?: string): Promise<SeriesModel> {
  const body: Record<string, unknown> = { name, data_type: dataType }
  if (config) body.config = config
  const resp = await fetch(`/api/trackers/${trackerId}/series`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

async function patchSeries(trackerId: number, seriesId: number, opts: { name?: string; data_type?: string; config?: string }): Promise<SeriesModel> {
  const resp = await fetch(`/api/trackers/${trackerId}/series/${seriesId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

async function deleteSeries(trackerId: number, seriesId: number): Promise<void> {
  const resp = await fetch(`/api/trackers/${trackerId}/series/${seriesId}`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

async function createValue(trackerId: number, seriesId: number, time: string, value: number): Promise<void> {
  const resp = await fetch(`/api/trackers/${trackerId}/series/${seriesId}/values`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ time, value }),
  })
  if (!resp.ok) throw resp
}

async function deleteValues(trackerId: number, seriesId: number): Promise<void> {
  const resp = await fetch(`/api/trackers/${trackerId}/series/${seriesId}/values`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

export async function likeTracker(trackerId: number): Promise<void> {
  const resp = await fetch(`/api/trackers/${trackerId}/like`, { method: 'POST' })
  if (!resp.ok) throw resp
}

export async function unlikeTracker(trackerId: number): Promise<void> {
  const resp = await fetch(`/api/trackers/${trackerId}/like`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

export async function patchTracker(trackerId: number, opts: { visibility?: string; chart_config?: string; description?: string; body?: string }): Promise<TrackerResponse> {
  const resp = await fetch(`/api/trackers/${trackerId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

export async function loadTrackerDetail({ params }: LoaderFunctionArgs): Promise<TrackerDetailData> {
  if (!params.trackerId) throw new Error('trackerId is required')
  return listSeries(parseInt(params.trackerId))
}

export const TrackerCard = ({ tracker, preview, loading, searchQuery, fromUser }: { tracker: TrackerResponse; preview?: PreviewData; loading?: boolean; searchQuery?: string; fromUser?: string }): React.JSX.Element => {
  const chartConfig = useMemo(() => {
    try { return JSON.parse(preview?.tracker?.chart_config ?? '{}') as ChartConfig }
    catch { return {} as ChartConfig }
  }, [preview])
  const colors = useMemo(() => resolvePalette(chartConfig.palette), [chartConfig.palette])
  const option = useMemo(() => {
    const datasets = preview?.series?.map((sv) => {
      let seriesConfig: SeriesConfig | undefined
      try {
        seriesConfig = JSON.parse(sv.series.config) as SeriesConfig
      } catch { /* ignore */ }
      return {
        label: sv.series.name,
        data: sv.values.map((v) => ({ x: v.time, y: String(v.value) })),
        seriesConfig,
      }
    }) ?? []

    const yAxes = chartConfig.y_axes?.length
      ? chartConfig.y_axes
      : [{ id: 0, position: 'left' as const }]
    const hasRightAxis = yAxes.some((a) => a.position === 'right')
    const isDateOnly = chartConfig.x_axis_type === 'date'

    const xAxis: any = {
      type: 'time' as const,
      axisLabel: { hideOverlap: true },
    }
    if (isDateOnly) {
      const currentYear = new Date().getFullYear()
      xAxis.axisLabel = {
        hideOverlap: true,
        formatter: (value: number) => {
          const d = new Date(value)
          const m = d.getMonth() + 1
          const day = d.getDate()
          if (d.getFullYear() === currentYear) {
            return `${m}/${day}`
          }
          return `${d.getFullYear()}/${m}/${day}`
        },
      }
    }

    return {
      animation: false,
      color: colors,
      grid: { left: 40, right: hasRightAxis ? 50 : 10, top: 10, bottom: 25 },
      xAxis,
      yAxis: yAxes.map((a) => ({
        type: 'value' as const,
        position: a.position,
        splitLine: {
          lineStyle: { type: 'dashed' as const, opacity: 0.3 },
          show: a.position === 'left' && !hasRightAxis,
        },
      })),
      series: datasets.map((ds, i) => {
        const seriesType = ds.seriesConfig?.type ?? 'line'
        const entry: any = {
          name: ds.label,
          type: seriesType,
          yAxisIndex: ds.seriesConfig?.y_axis_index ?? 0,
          data: ds.data.map((p) => [isDateOnly ? p.x.substring(0, 10) : p.x, Number(p.y)]),
        }
        if (seriesType === 'bar') {
          entry.barMaxWidth = '90%'
        } else {
          entry.lineStyle = { width: 1.5 }
          if (chartConfig.show_symbols !== true) {
            entry.symbol = 'none'
          }
          if (chartConfig.area !== false) {
            entry.areaStyle = areaGradient(colors[i % colors.length], 0.3)
          }
        }
        return entry
      }),
      tooltip: {
        trigger: 'axis',
        formatter: (params: any) => {
          const items = Array.isArray(params) ? params : [params]
          const axisValue = items[0]?.axisValue ?? ''
          const header = axisValue
            ? isDateOnly
              ? `<b>${new Date(axisValue).toLocaleDateString()}</b><br/>`
              : `<b>${new Date(axisValue).toLocaleString()}</b><br/>`
            : ''
          const body = items.map((p: any) => {
            const fmt = datasets[p.seriesIndex]?.seriesConfig?.value_format
            return `${p.marker} ${p.seriesName}: ${formatValue(p.value[1], fmt)}`
          }).join('<br/>')
          return header + body
        },
      },
    }
  }, [preview, colors])

  const linkTo = tracker.type === 'coverage'
    ? `/coverages/${tracker.id}`
    : `/trackers/${tracker.id}`
  const linkState = fromUser
    ? { fromSearch: searchQuery, fromUser }
    : searchQuery
      ? { fromSearch: searchQuery }
      : undefined

  return (
    <div className="bg-card border rounded-lg py-4 pl-2 pr-3 hover:shadow-md transition-shadow">
      <div className="flex items-center justify-between gap-2 mb-1 pl-2 text-xs text-muted-foreground">
        {tracker.visibility === 'private' ? (
          <span className="text-xs bg-gray-100 text-gray-600 px-1.5 py-0.5 rounded">private</span>
        ) : (
          <span />
        )}
        <span className="flex-shrink-0">
          {tracker.last_updated_at ? new Date(tracker.last_updated_at).toLocaleDateString() : ''}
        </span>
      </div>
      <Link to={linkTo} state={linkState} className="block">
        <div className="flex items-center gap-2 mb-2 pl-2">
          <h3 className="font-semibold text-lg truncate">{tracker.name}</h3>
          {tracker.role && (
            <span className="text-xs bg-primary/10 text-primary px-1.5 py-0.5 rounded">{tracker.role}</span>
          )}
          {tracker.type === 'coverage' && (
            <span className="text-xs bg-green-100 text-green-700 px-1.5 py-0.5 rounded">Coverage</span>
          )}
          {(tracker.like_count ?? 0) > 0 && (
            <span className="relative ml-auto flex-shrink-0">
              <Star className="w-4 h-4 text-yellow-500 fill-yellow-500" />
              <span className="absolute -bottom-1 -right-1.5 text-[10px] leading-none font-medium text-yellow-700 bg-yellow-50 rounded px-0.5">{tracker.like_count}</span>
            </span>
          )}
        </div>
        <div className="h-[120px]">
          {loading ? (
            <div className="flex items-center justify-center h-full text-muted-foreground text-sm">Loading...</div>
          ) : option.series.length > 0 ? (
            <ReactECharts option={option} style={{ width: '100%', height: 120 }} opts={{ renderer: 'svg' }} theme={CHART_THEME_NAME} />
          ) : (
            <div className="flex items-center justify-center h-full text-muted-foreground text-sm">No data</div>
          )}
        </div>
      </Link>
    </div>
  )
}

export const TrackerDetailView = (): React.JSX.Element => {
  const data = useLoaderData() as TrackerDetailData
  const { tracker } = data
  const user = useUser()
  const [seriesList] = useState<SeriesModel[]>(data.series)
  const [seriesValues, setSeriesValues] = useState<SeriesValues[]>([])
  const [liked, setLiked] = useState(tracker.liked)
  const [likeCount, setLikeCount] = useState(tracker.like_count ?? 0)
  const [likeLoading, setLikeLoading] = useState(false)

  useEffect(() => {
    Promise.all(
      seriesList.map((s) =>
        fetch(`/api/trackers/${tracker.id}/series/${s.id}/values`)
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
        setLikeCount((c) => Math.max(0, c - 1))
      } else {
        await likeTracker(tracker.id)
        setLiked(true)
        setLikeCount((c) => c + 1)
      }
    } catch {
      // ignore
    } finally {
      setLikeLoading(false)
    }
  }

  const valuesToDataset = (sv: SeriesValues): Dataset => {
    let seriesConfig: SeriesConfig | undefined
    try {
      seriesConfig = JSON.parse(sv.series.config) as SeriesConfig
    } catch { /* ignore */ }
    return {
      data: sv.values.map((v: ValueModel) => ({ x: v.time, y: String(v.value) })),
      label: sv.series.name,
      seriesConfig,
    }
  }

  const datasets: Dataset[] = useMemo(() => seriesValues.map(valuesToDataset), [seriesValues])

  const viewChartConfig = useMemo<ChartConfig | null>(() => {
    try {
      return JSON.parse(tracker.chart_config) as ChartConfig
    } catch {
      return null
    }
  }, [tracker.chart_config])

  const [range, setRange] = useState<TimeRangeKey>('all')
  const { min, max } = computeDateRange(range)

  return (
    <div>
      <div className="my-4">
        <div className="flex items-center gap-3">
          <h1 className="text-3xl">{tracker.name}</h1>
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
          {tracker.role !== '' && (
            <Button variant="outline" size="sm" asChild>
              <Link to={`/trackers/${tracker.id}/edit`}>Edit</Link>
            </Button>
          )}
        </div>
        {tracker.description && (
          <p className="mt-1">{tracker.description}</p>
        )}
      </div>

      <div className="bg-card border rounded-lg py-4 pl-2 pr-3 sm:px-4 shadow-md">
        {datasets.length > 0 ? (
          <>
            <TimeRangeSelector value={range} onChange={setRange} />
            <TrackerChart data={{ datasets }} chartConfig={viewChartConfig} min={min} max={max} />
          </>
        ) : (
          <p className="text-muted-foreground">No data to display</p>
        )}
      </div>

      {tracker.body?.trim() && (
        <div className="mt-6 bg-card border rounded-lg p-4 shadow-md md-body">
          <MDEditor.Markdown source={tracker.body} />
        </div>
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

  const [seriesValueFormats, setSeriesValueFormats] = useState<Record<number, string>>(() => {
    const map: Record<number, string> = {}
    for (const s of data.series) {
      try {
        const cfg = JSON.parse(s.config) as SeriesConfig
        if (cfg.value_format) map[s.id] = cfg.value_format
      } catch { /* ignore */ }
    }
    return map
  })

  const [seriesTypes, setSeriesTypes] = useState<Record<number, 'line' | 'bar'>>(() => {
    const map: Record<number, 'line' | 'bar'> = {}
    for (const s of data.series) {
      try {
        const cfg = JSON.parse(s.config) as SeriesConfig
        if (cfg.type) map[s.id] = cfg.type
      } catch { /* ignore */ }
    }
    return map
  })

  const [seriesYAxisIndices, setSeriesYAxisIndices] = useState<Record<number, number>>(() => {
    const map: Record<number, number> = {}
    for (const s of data.series) {
      try {
        const cfg = JSON.parse(s.config) as SeriesConfig
        if (cfg.y_axis_index !== undefined) map[s.id] = cfg.y_axis_index
      } catch { /* ignore */ }
    }
    return map
  })

  const [savedChartConfig, setSavedChartConfig] = useState(tracker.chart_config)
  const parsedChartConfig = useMemo<ChartConfig>(() => {
    try {
      return JSON.parse(savedChartConfig) as ChartConfig
    } catch {
      return {}
    }
  }, [savedChartConfig])
  const [description, setDescription] = useState(tracker.description ?? '')
  const [body, setBody] = useState(tracker.body ?? '')
  const [bodySaved, setBodySaved] = useState(false)
  const [visibility, setVisibility] = useState(tracker.visibility)
  const [xLabel, setXLabel] = useState(parsedChartConfig.x_axis_label ?? '')
  const [xAxisType, setXAxisType] = useState<'date' | 'datetime'>(parsedChartConfig.x_axis_type ?? 'date')
  const [area, setArea] = useState(parsedChartConfig.area ?? true)
  const [showLegend, setShowLegend] = useState(parsedChartConfig.show_legend ?? true)
  const [showSymbols, setShowSymbols] = useState(parsedChartConfig.show_symbols ?? true)
  const [showSlider, setShowSlider] = useState(parsedChartConfig.show_slider ?? true)
  const [showToolbox, setShowToolbox] = useState(parsedChartConfig.show_toolbox ?? true)
  const [palette, setPalette] = useState(parsedChartConfig.palette ?? 'default')
  const [yAxes, setYAxes] = useState<YAxisConfig[]>(() => {
    if (parsedChartConfig.y_axes && parsedChartConfig.y_axes.length > 0) {
      return parsedChartConfig.y_axes
    }
    return [{ id: 0, position: 'left' }]
  })

  const isCoverage = tracker.type === 'coverage'

  const handleDescriptionBlur = async () => {
    const trimmed = description.trim()
    if (trimmed === (tracker.description ?? '')) return
    try {
      await patchTracker(tracker.id, { description: trimmed })
    } catch {
      setDescription(tracker.description ?? '')
    }
  }

  const handleBodySave = async () => {
    setBodySaved(false)
    try {
      const updated = await patchTracker(tracker.id, { body })
      setBody(updated.body ?? '')
      setBodySaved(true)
    } catch {
      setBody(tracker.body ?? '')
    }
  }

  const handleVisibilityChange = async (newVisibility: string) => {
    try {
      await patchTracker(tracker.id, { visibility: newVisibility })
      setVisibility(newVisibility)
    } catch {
      // ignore
    }
  }

  const saveChartConfig = async (newYAxes: YAxisConfig[]) => {
    const cc: ChartConfig = {}
    if (xLabel.trim()) cc.x_axis_label = xLabel.trim()
    if (xAxisType === 'date') cc.x_axis_type = 'date'
    if (!area) cc.area = false
    if (!showLegend) cc.show_legend = false
    if (!showSymbols) cc.show_symbols = false
    if (!showSlider) cc.show_slider = false
    if (showToolbox) cc.show_toolbox = true
    cc.palette = palette
    cc.y_axes = newYAxes.map((a) => {
      const axis: YAxisConfig = { id: a.id, position: a.position }
      if (a.label?.trim()) axis.label = a.label.trim()
      if (a.min !== undefined) axis.min = a.min
      if (a.max !== undefined) axis.max = a.max
      return axis
    })
    try {
      const updated = await patchTracker(tracker.id, { chart_config: JSON.stringify(cc) })
      setSavedChartConfig(updated.chart_config)
    } catch {
      // ignore
    }
  }

  const handleChartConfigSave = () => saveChartConfig(yAxes)

  useEffect(() => {
    Promise.all(
      seriesList.map((s) =>
        fetch(`/api/trackers/${tracker.id}/series/${s.id}/values`)
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
      const resp = await fetch(`/api/trackers/${tracker.id}/series/${selectedSeriesId}/values`)
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

  const buildSeriesConfig = (seriesId: number): SeriesConfig => {
    const config: SeriesConfig = {}
    const fmt = seriesValueFormats[seriesId]
    if (fmt) config.value_format = fmt
    const t = seriesTypes[seriesId]
    if (t) config.type = t
    const yi = seriesYAxisIndices[seriesId]
    if (yi !== undefined) config.y_axis_index = yi
    return config
  }

  const handleSaveSeriesConfig = async (seriesId: number) => {
    const config = buildSeriesConfig(seriesId)
    try {
      const updated = await patchSeries(tracker.id, seriesId, { config: JSON.stringify(config) })
      setSeriesList((prev) => prev.map((s) => s.id === seriesId ? updated : s))
    } catch {
      // ignore
    }
  }

  const handleSaveValueFormat = async (seriesId: number, fmt: string) => {
    if (fmt) {
      setSeriesValueFormats((prev) => ({ ...prev, [seriesId]: fmt }))
    } else {
      setSeriesValueFormats((prev) => {
        const next = { ...prev }
        delete next[seriesId]
        return next
      })
    }
    const config: SeriesConfig = {}
    if (fmt) config.value_format = fmt
    const t = seriesTypes[seriesId]
    if (t) config.type = t
    const yi = seriesYAxisIndices[seriesId]
    if (yi !== undefined) config.y_axis_index = yi
    try {
      const updated = await patchSeries(tracker.id, seriesId, { config: JSON.stringify(config) })
      setSeriesList((s) => s.map((s) => s.id === seriesId ? updated : s))
    } catch {
      // ignore
    }
  }

  const handleSeriesTypeChange = async (seriesId: number, type: 'line' | 'bar') => {
    setSeriesTypes((prev) => ({ ...prev, [seriesId]: type }))
    const config: SeriesConfig = { ...buildSeriesConfig(seriesId), type }
    try {
      const updated = await patchSeries(tracker.id, seriesId, { config: JSON.stringify(config) })
      setSeriesList((prev) => prev.map((s) => s.id === seriesId ? updated : s))
    } catch {
      // ignore
    }
  }

  const handleSeriesYAxisChange = async (seriesId: number, yAxisIndex: number) => {
    setSeriesYAxisIndices((prev) => ({ ...prev, [seriesId]: yAxisIndex }))
    const config: SeriesConfig = { ...buildSeriesConfig(seriesId), y_axis_index: yAxisIndex }
    try {
      const updated = await patchSeries(tracker.id, seriesId, { config: JSON.stringify(config) })
      setSeriesList((prev) => prev.map((s) => s.id === seriesId ? updated : s))
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

  const valuesToDataset = (sv: SeriesValues): Dataset => {
    let seriesConfig: SeriesConfig | undefined
    try {
      seriesConfig = JSON.parse(sv.series.config) as SeriesConfig
    } catch { /* ignore */ }
    return {
      data: sv.values.map((v: ValueModel) => ({ x: v.time, y: String(v.value) })),
      label: sv.series.name,
      seriesConfig,
    }
  }

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
          <option value="public">Public</option>
        </select>
      </div>
    )
  }

  return (
    <div>
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
            <TableHead>Chart Type</TableHead>
            <TableHead>Y-Axis</TableHead>
            <TableHead>Value Format</TableHead>
            <TableHead className="w-48">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {seriesList.length === 0 ? (
            <TableRow>
              <TableCell colSpan={6} className="text-center text-muted-foreground">
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
                <TableCell>
                  <select
                    value={seriesTypes[s.id] ?? 'line'}
                    onChange={(e) => handleSeriesTypeChange(s.id, e.target.value as 'line' | 'bar')}
                    className="border rounded px-1 py-0.5 text-sm"
                  >
                    <option value="line">Line</option>
                    <option value="bar">Bar</option>
                  </select>
                </TableCell>
                <TableCell>
                  {(() => {
                    const sorted = yAxes.map((a, i) => ({ ...a, origIdx: i }))
                      .sort((a, b) => (a.position === 'left' ? -1 : 1))
                    const currentIdx = seriesYAxisIndices[s.id] ?? 0
                    const dispIdx = sorted.findIndex((a) => a.origIdx === currentIdx)
                    return (
                      <select
                        value={dispIdx >= 0 ? dispIdx : 0}
                        onChange={(e) => handleSeriesYAxisChange(s.id, sorted[parseInt(e.target.value)].origIdx)}
                        className="border rounded px-1 py-0.5 text-sm"
                      >
                        {sorted.map((a, i) => (
                          <option key={i} value={i}>
                            {a.position === 'left' ? 'Left' : 'Right'}{a.label ? ` (${a.label})` : ''}
                          </option>
                        ))}
                      </select>
                    )
                  })()}
                </TableCell>
                <TableCell>
                  <ValueFormatCell
                    seriesId={s.id}
                    initialFormat={seriesValueFormats[s.id] ?? ''}
                    onSave={handleSaveValueFormat}
                  />
                </TableCell>
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

      <div className="bg-card border rounded-lg py-4 pl-2 pr-3 sm:px-4 mb-4 shadow-md">
        {datasets.length > 0 ? (
          <TrackerChart data={{ datasets }} chartConfig={chartConfigForChart} />
        ) : (
          <p className="text-muted-foreground">No data to display</p>
        )}
      </div>

      {/* Chart Options */}
      <h2 className="text-xl my-2">Chart Options</h2>
      <div className="flex flex-wrap items-center gap-3 mb-4">
        <select
          value={xAxisType}
          onChange={(e) => setXAxisType(e.target.value as 'date' | 'datetime')}
          className="border rounded px-2 py-1"
        >
          <option value="date">Date</option>
          <option value="datetime">Datetime</option>
        </select>
        <input
          type="text"
          value={xLabel}
          onChange={(e) => setXLabel(e.target.value)}
          placeholder="X-axis label"
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
        <label className="flex items-center gap-1 text-sm">
          <input type="checkbox" checked={showSymbols} onChange={(e) => setShowSymbols(e.target.checked)} />
          Symbols
        </label>
        <label className="flex items-center gap-1 text-sm">
          <input type="checkbox" checked={showSlider} onChange={(e) => setShowSlider(e.target.checked)} />
          Slider
        </label>
        <label className="flex items-center gap-1 text-sm">
          <input type="checkbox" checked={showToolbox} onChange={(e) => setShowToolbox(e.target.checked)} />
          Toolbox
        </label>
        <select
          value={palette}
          onChange={(e) => setPalette(e.target.value)}
          className="border rounded px-2 py-1"
        >
          {PALETTE_NAMES.map((name) => (
            <option key={name} value={name}>{name}</option>
          ))}
        </select>
      </div>

      {/* Y-Axes */}
      <h3 className="text-lg my-2">Y-Axes</h3>
      <div className="flex flex-col gap-2 mb-4">
        {(['left', 'right'] as const).map((pos) => {
          const axis = yAxes.find((a) => a.position === pos)
          const active = !!axis
          const canRemove = yAxes.length > 1

          return (
            <div key={pos} className="flex items-center gap-2">
              <span className="text-sm font-medium w-12">{pos === 'left' ? 'Left' : 'Right'}</span>
              {active ? (
                <>
                  <input
                    type="text"
                    value={axis.label ?? ''}
                    onChange={(e) => {
                      const next = yAxes.map((a) =>
                        a.position === pos ? { ...a, label: e.target.value || undefined } : a
                      )
                      setYAxes(next)
                    }}
                    placeholder="Label"
                    className="border rounded px-2 py-1 w-32 text-sm"
                  />
                  <input
                    type="number"
                    value={axis.min ?? ''}
                    onChange={(e) => {
                      const v = e.target.value ? parseFloat(e.target.value) : undefined
                      const next = yAxes.map((a) =>
                        a.position === pos ? { ...a, min: v } : a
                      )
                      setYAxes(next)
                    }}
                    placeholder="Min"
                    className="border rounded px-2 py-1 w-20 text-sm"
                  />
                  <input
                    type="number"
                    value={axis.max ?? ''}
                    onChange={(e) => {
                      const v = e.target.value ? parseFloat(e.target.value) : undefined
                      const next = yAxes.map((a) =>
                        a.position === pos ? { ...a, max: v } : a
                      )
                      setYAxes(next)
                    }}
                    placeholder="Max"
                    className="border rounded px-2 py-1 w-20 text-sm"
                  />
                  <Button
                    variant="destructive"
                    size="sm"
                    disabled={!canRemove}
                    onClick={async () => {
                      const removedIndex = yAxes.findIndex((a) => a.position === pos)
                      for (const [sidStr, yi] of Object.entries(seriesYAxisIndices)) {
                        const sid = Number(sidStr)
                        if (yi === removedIndex) {
                          await handleSeriesYAxisChange(sid, 0)
                        } else if (yi > removedIndex) {
                          await handleSeriesYAxisChange(sid, yi - 1)
                        }
                      }
                      const newYAxes = yAxes.filter((a) => a.position !== pos)
                      setYAxes(newYAxes)
                      await saveChartConfig(newYAxes)
                    }}
                  >
                    Remove
                  </Button>
                </>
              ) : (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={async () => {
                    const id = yAxes.length > 0 ? Math.max(...yAxes.map((a) => a.id)) + 1 : 0
                    const newYAxes = [...yAxes, { id, position: pos }]
                    setYAxes(newYAxes)
                    await saveChartConfig(newYAxes)
                  }}
                >
                  Add
                </Button>
              )}
            </div>
          )
        })}
      </div>

      <Button onClick={handleChartConfigSave}>Save Chart Options</Button>

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
          type={xAxisType === 'date' ? 'date' : 'datetime-local'}
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

      {/* Description */}
      <h2 className="text-xl my-2">Description</h2>
      <input
        type="text"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        onBlur={handleDescriptionBlur}
        placeholder="One-line description (max 200 characters)"
        maxLength={200}
        className="border rounded px-2 py-1 mb-4 w-full max-w-md"
      />

      {/* Body */}
      <h2 className="text-xl my-2">Body</h2>
      <p className="text-sm text-muted-foreground mb-2">
        Markdown is supported. This content is shown below the chart on the detail page.
      </p>
      <div data-color-mode="light" className="mb-2 bg-card border rounded-lg p-4 shadow-md">
        <MDEditor
          value={body}
          onChange={(v) => { setBody(v ?? ''); setBodySaved(false) }}
          preview="live"
          height={300}
          textareaProps={{ placeholder: 'Write the body in Markdown...' }}
        />
      </div>
      <div className="flex items-center gap-2 mb-4">
        <Button size="sm" onClick={handleBodySave}>
          {bodySaved ? 'Saved' : 'Save Body'}
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
        <option value="public">Public</option>
      </select>
    </div>
  )
}

export const TrackerCreate = (): React.JSX.Element => {
  const navigate = useNavigate()
  const user = useUser()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [visibility, setVisibility] = useState('private')
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
        description.trim() || undefined,
      )
      navigate(`/trackers/${created.id}`)
    } catch {
      setError('Failed to create tracker. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
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
          <label className="block mb-1">Description</label>
          <input
            type="text"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="One-line description (max 200 characters)"
            maxLength={200}
            className="border rounded px-2 py-1 w-full"
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
            <option value="public">Public</option>
          </select>
        </div>

        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <Link to={user ? `/users/${user.username}` : '/'}>Cancel</Link>
          </Button>
          <Button onClick={handleCreate} disabled={!name.trim() || loading}>
            Create
          </Button>
        </div>
      </div>
    </div>
  )
}

function ValueFormatCell({ seriesId, initialFormat, onSave }: { seriesId: number; initialFormat: string; onSave: (seriesId: number, fmt: string) => void }): React.JSX.Element {
  const [value, setValue] = useState(initialFormat)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  useEffect(() => { setValue(initialFormat); setSaved(false) }, [initialFormat])

  const handleSave = async () => {
    setSaving(true)
    setSaved(false)
    onSave(seriesId, value)
    setSaving(false)
    setSaved(true)
  }

  return (
    <div className="flex items-center gap-1">
      <input
        type="text"
        value={value}
        onChange={(e) => { setValue(e.target.value); setSaved(false) }}
        placeholder="e.g. %.1f"
        className="border rounded px-1 py-0.5 w-20 text-sm"
        onKeyDown={(e) => { if (e.key === 'Enter') handleSave() }}
      />
      <Button size="sm" variant="outline" onClick={handleSave} disabled={saving}>
        {saving ? '...' : saved ? 'Saved' : 'Save'}
      </Button>
    </div>
  )
}

const TrackerDetailRouter = (): React.JSX.Element => {
  const data = useLoaderData() as TrackerDetailData
  if (data.tracker.type === 'coverage') {
    throw new Response('Not Found', { status: 404 })
  }
  return <TrackerDetailView />
}

export const TrackerDetailEditRouter = (): React.JSX.Element => {
  const data = useLoaderData() as TrackerDetailData
  if (data.tracker.role === '') {
    throw new Response('Forbidden', { status: 403 })
  }
  return <TrackerDetailEdit />
}

export const trackerRoute = [
  {
    path: 'new',
    element: <TrackerCreate />,
  },
  {
    path: ':trackerId',
    loader: loadTrackerDetail,
    element: <TrackerDetailRouter />,
    handle: {
      crumb: (params: Params, data: any) => ({
        label: data?.tracker?.name ?? `Tracker #${params.trackerId}`,
      }),
    },
  },
  {
    path: ':trackerId/edit',
    loader: loadTrackerDetail,
    element: <TrackerDetailEditRouter />,
    handle: {
      crumb: () => ({ label: 'Edit' }),
    },
  },
]
