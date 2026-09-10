import React, { useEffect, useMemo, useState } from 'react'

import ReactECharts from 'echarts-for-react'
import MDEditor from '@uiw/react-md-editor'
import '@uiw/react-md-editor/markdown-editor.css'
import { MoreVertical, Pencil, Plus, Settings, Settings2, Star, Trash } from 'lucide-react'
import { AlertDialog, Dialog, DropdownMenu } from 'radix-ui'

import {
  Link,
  LoaderFunctionArgs,
  Params,
  useLoaderData,
  useNavigate,
} from 'react-router'

import { Button } from '@/components/ui/button'
import { ChartConfig, SeriesConfig, SeriesModel, TrackerResponse, YAxisConfig, normalizeChartConfig } from './core'
import { SettingsForm } from './settings-form'
import { SeriesTable } from './series-form'
import { formatValue, Dataset, TrackerChart, resolvePalette, areaGradient, CHART_THEME_NAME } from './chart'
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

export async function patchSeries(trackerId: number, seriesId: number, opts: { name?: string; data_type?: string; config?: string }): Promise<SeriesModel> {
  const resp = await fetch(`/api/trackers/${trackerId}/series/${seriesId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

export async function deleteSeries(trackerId: number, seriesId: number): Promise<void> {
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

export async function deleteValues(trackerId: number, seriesId: number): Promise<void> {
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

export async function patchTracker(trackerId: number, opts: { name?: string; visibility?: string; chart_config?: string; description?: string; body?: string }): Promise<TrackerResponse> {
  const resp = await fetch(`/api/trackers/${trackerId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

export async function deleteTracker(trackerId: number): Promise<void> {
  const resp = await fetch(`/api/trackers/${trackerId}`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

export async function loadTrackerDetail({ params }: LoaderFunctionArgs): Promise<TrackerDetailData> {
  if (!params.trackerId) throw new Error('trackerId is required')
  return listSeries(parseInt(params.trackerId))
}

export const TrackerCard = ({ tracker, preview, loading, searchQuery, fromUser }: { tracker: TrackerResponse; preview?: PreviewData; loading?: boolean; searchQuery?: string; fromUser?: string }): React.JSX.Element => {
  const chartConfig = useMemo(() => {
    try { return normalizeChartConfig(JSON.parse(preview?.tracker?.chart_config ?? '{}') as ChartConfig) }
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
        {tracker.owner_name ? (
          <Link to={`/users/${encodeURIComponent(tracker.owner_name)}`} className="truncate hover:text-primary hover:underline">
            {tracker.owner_name}
          </Link>
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
          {tracker.visibility === 'private' && (
            <span className="text-xs bg-gray-100 text-gray-600 px-1.5 py-0.5 rounded">private</span>
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
  const navigate = useNavigate()
  const [seriesList, setSeriesList] = useState<SeriesModel[]>(data.series)
  const [seriesValues, setSeriesValues] = useState<SeriesValues[]>([])
  const [liked, setLiked] = useState(tracker.liked)
  const [likeCount, setLikeCount] = useState(tracker.like_count ?? 0)
  const [likeLoading, setLikeLoading] = useState(false)
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const isOwner = tracker.role !== ''
  const isRoleOwner = tracker.role === 'owner'

  const [trackerName, setTrackerName] = useState(tracker.name)
  const [trackerDescription, setTrackerDescription] = useState(tracker.description ?? '')
  const [trackerBody, setTrackerBody] = useState(tracker.body ?? '')
  const [trackerVisibility, setTrackerVisibility] = useState(tracker.visibility)

  const [editingTitle, setEditingTitle] = useState(false)
  const [editingDescription, setEditingDescription] = useState(false)
  const [editingBody, setEditingBody] = useState(false)
  const [showChartOptions, setShowChartOptions] = useState(false)
  const [showSeriesSettings, setShowSeriesSettings] = useState(false)
  const [addSeriesOpen, setAddSeriesOpen] = useState(false)
  const [newSeriesName, setNewSeriesName] = useState('')
  const [newSeriesDataType, setNewSeriesDataType] = useState('float')
  const [newSeriesChartType, setNewSeriesChartType] = useState<'line' | 'bar'>('line')
  const [addingSeries, setAddingSeries] = useState(false)
  const [addSeriesError, setAddSeriesError] = useState<string | null>(null)

  const [showAddValue, setShowAddValue] = useState(false)
  const [addingValues, setAddingValues] = useState<Record<number, boolean>>({})
  const [addValueError, setAddValueError] = useState<string | null>(null)
  const [valueInputs, setValueInputs] = useState<Record<number, { time: string; value: string }>>({})

  const [draftName, setDraftName] = useState(tracker.name)
  const [draftDescription, setDraftDescription] = useState(tracker.description ?? '')
  const [draftBody, setDraftBody] = useState(tracker.body ?? '')

  const [savingTitle, setSavingTitle] = useState(false)
  const [savingDescription, setSavingDescription] = useState(false)
  const [savingBody, setSavingBody] = useState(false)

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

  const handleDelete = async () => {
    setDeleting(true)
    setDeleteError(null)
    try {
      await deleteTracker(tracker.id)
      if (user) {
        navigate(`/users/${encodeURIComponent(user.username)}`)
      } else {
        navigate('/trackers')
      }
    } catch {
      setDeleteError('Failed to delete tracker. Please try again.')
      setDeleteConfirmOpen(true)
    } finally {
      setDeleting(false)
    }
  }

  const handleAddSeries = async () => {
    const name = newSeriesName.trim()
    if (!name) return
    setAddingSeries(true)
    setAddSeriesError(null)
    try {
      const config = JSON.stringify({ type: newSeriesChartType })
      const created = await createSeries(tracker.id, name, newSeriesDataType, config)
      setSeriesList((prev) => [...prev, created])
      setAddSeriesOpen(false)
      setNewSeriesName('')
      setNewSeriesDataType('float')
      setNewSeriesChartType('line')
    } catch {
      setAddSeriesError('Failed to add series. Please try again.')
    } finally {
      setAddingSeries(false)
    }
  }

  const handleSaveTitle = async () => {
    const trimmed = draftName.trim()
    if (!trimmed || trimmed === trackerName) {
      setEditingTitle(false)
      return
    }
    setSavingTitle(true)
    try {
      await patchTracker(tracker.id, { name: trimmed })
      setTrackerName(trimmed)
      setEditingTitle(false)
    } catch {
      setDraftName(trackerName)
    } finally {
      setSavingTitle(false)
    }
  }

  const handleCancelTitle = () => {
    setDraftName(trackerName)
    setEditingTitle(false)
  }

  const handleSaveDescription = async () => {
    const trimmed = draftDescription.trim()
    if (trimmed === trackerDescription) {
      setEditingDescription(false)
      return
    }
    setSavingDescription(true)
    try {
      await patchTracker(tracker.id, { description: trimmed })
      setTrackerDescription(trimmed)
      setEditingDescription(false)
    } catch {
      setDraftDescription(trackerDescription)
    } finally {
      setSavingDescription(false)
    }
  }

  const handleCancelDescription = () => {
    setDraftDescription(trackerDescription)
    setEditingDescription(false)
  }

  const handleSaveBody = async () => {
    if (draftBody === trackerBody) {
      setEditingBody(false)
      return
    }
    setSavingBody(true)
    try {
      await patchTracker(tracker.id, { body: draftBody })
      setTrackerBody(draftBody)
      setEditingBody(false)
    } catch {
      setDraftBody(trackerBody)
    } finally {
      setSavingBody(false)
    }
  }

  const handleCancelBody = () => {
    setDraftBody(trackerBody)
    setEditingBody(false)
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

  const [chartConfig, setChartConfig] = useState<ChartConfig | null>(() => {
    try {
      return normalizeChartConfig(JSON.parse(tracker.chart_config) as ChartConfig)
    } catch {
      return null
    }
  })
  const [chartDraft, setChartDraft] = useState<ChartConfig | null>(chartConfig)

  const [range, setRange] = useState<TimeRangeKey>('all')
  const { min, max } = computeDateRange(range)

  const xAxisType = chartConfig?.x_axis_type ?? 'date'
  const yAxes: YAxisConfig[] = useMemo(() => {
    if (chartConfig?.y_axes && chartConfig.y_axes.length > 0) {
      return chartConfig.y_axes
    }
    return [{ id: 0, position: 'left' }]
  }, [chartConfig])
  const todayValue = useMemo(() => {
    const now = new Date()
    if (xAxisType === 'date') {
      return now.toISOString().slice(0, 10)
    }
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}T${pad(now.getHours())}:${pad(now.getMinutes())}`
  }, [xAxisType])

  const openAddValue = () => {
    setValueInputs(Object.fromEntries(seriesList.map((s) => [s.id, { time: todayValue, value: '' }])))
    setAddValueError(null)
    setShowAddValue(true)
  }

  const closeAddValue = () => {
    setValueInputs({})
    setAddValueError(null)
    setShowAddValue(false)
  }

  const setValueInput = (seriesId: number, field: 'time' | 'value', val: string) => {
    setValueInputs((prev) => {
      const current = prev[seriesId] ?? { time: todayValue, value: '' }
      return { ...prev, [seriesId]: { ...current, [field]: val } }
    })
  }

  const handleAddValue = async (seriesId: number) => {
    const input = valueInputs[seriesId]
    if (!input?.time || !input?.value) return
    setAddingValues((prev) => ({ ...prev, [seriesId]: true }))
    setAddValueError(null)
    try {
      await createValue(tracker.id, seriesId, new Date(input.time).toISOString(), parseFloat(input.value))
      const resp = await fetch(`/api/trackers/${tracker.id}/series/${seriesId}/values`)
      const data = await resp.json()
      setSeriesValues((prev) =>
        prev.map((sv) =>
          sv.series.id === seriesId ? { ...sv, values: data.values ?? [] } : sv
        )
      )
      setValueInput(seriesId, 'value', '')
    } catch {
      setAddValueError('Failed to add value. Please try again.')
    } finally {
      setAddingValues((prev) => ({ ...prev, [seriesId]: false }))
    }
  }

  const handleSeriesDeleted = (seriesId: number) => {
    setSeriesList((prev) => prev.filter((s) => s.id !== seriesId))
    setSeriesValues((prev) => prev.filter((sv) => sv.series.id !== seriesId))
  }

  const handleValuesCleared = (seriesId: number) => {
    setSeriesValues((prev) =>
      prev.map((sv) =>
        sv.series.id === seriesId ? { ...sv, values: [] } : sv
      )
    )
  }

  return (
    <div>
      <div className="my-4">
        <div className="flex items-center gap-3 pr-3 sm:pr-4">
          {editingTitle ? (
            <div className="flex items-center gap-2 flex-1">
              <input
                type="text"
                value={draftName}
                onChange={(e) => setDraftName(e.target.value)}
                maxLength={200}
                className="text-3xl border rounded px-2 py-1 flex-1"
                autoFocus
              />
              <Button size="sm" onClick={handleSaveTitle} disabled={savingTitle || !draftName.trim()}>
                Save
              </Button>
              <Button size="sm" variant="outline" onClick={handleCancelTitle} disabled={savingTitle}>
                Cancel
              </Button>
            </div>
          ) : (
            <>
              <h1 className="text-3xl">{trackerName}</h1>
              {isOwner && (
                <button
                  type="button"
                  aria-label="Edit title"
                  onClick={() => { setDraftName(trackerName); setEditingTitle(true) }}
                  className="p-1.5 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
                >
                  <Pencil className="w-4 h-4" />
                </button>
              )}
            </>
          )}
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
          {isRoleOwner && (
            <DropdownMenu.Root>
              <DropdownMenu.Trigger asChild>
                <button
                  type="button"
                  aria-label="Tracker menu"
                  className="ml-auto p-1.5 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
                >
                  <MoreVertical className="w-5 h-5" />
                </button>
              </DropdownMenu.Trigger>
              <DropdownMenu.Portal>
                <DropdownMenu.Content align="end" className="bg-popover text-popover-foreground rounded-md border shadow-md p-1 min-w-[12rem] z-50">
                  <DropdownMenu.Item
                    className="flex items-center gap-2 rounded px-2 py-1.5 text-sm cursor-pointer outline-none data-[highlighted]:bg-accent"
                    onSelect={() => setShowChartOptions((v) => !v)}
                  >
                    <Settings className="w-4 h-4" />
                    Settings
                  </DropdownMenu.Item>
                  <DropdownMenu.Item
                    className="flex items-center gap-2 rounded px-2 py-1.5 text-sm cursor-pointer outline-none data-[highlighted]:bg-accent"
                    onSelect={() => setShowSeriesSettings((v) => !v)}
                  >
                    <Settings2 className="w-4 h-4" />
                    Series
                  </DropdownMenu.Item>
                  <DropdownMenu.Item
                    className="flex items-center gap-2 rounded px-2 py-1.5 text-sm cursor-pointer outline-none data-[highlighted]:bg-accent"
                    onSelect={() => { setAddSeriesError(null); setNewSeriesName(seriesList.length === 0 ? tracker.name : ''); setAddSeriesOpen(true) }}
                  >
                    <Plus className="w-4 h-4" />
                    Add Series
                  </DropdownMenu.Item>
                  <DropdownMenu.Item
                    className="flex items-center gap-2 rounded px-2 py-1.5 text-sm cursor-pointer outline-none data-[highlighted]:bg-accent"
                    onSelect={openAddValue}
                  >
                    <Plus className="w-4 h-4" />
                    Add Data Points
                  </DropdownMenu.Item>
                  <DropdownMenu.Separator className="my-1 h-px bg-border" />
                  <DropdownMenu.Item
                    className="flex items-center gap-2 rounded px-2 py-1.5 text-sm cursor-pointer text-red-600 data-[highlighted]:bg-red-50 data-[highlighted]:text-red-700 outline-none"
                    onSelect={() => { setDeleteError(null); setDeleteConfirmOpen(true) }}
                  >
                    <Trash className="w-4 h-4" />
                    Delete
                  </DropdownMenu.Item>
                </DropdownMenu.Content>
              </DropdownMenu.Portal>
            </DropdownMenu.Root>
          )}
        </div>

        {editingDescription ? (
          <div className="mt-2 flex items-center gap-2">
            <input
              type="text"
              value={draftDescription}
              onChange={(e) => setDraftDescription(e.target.value)}
              maxLength={200}
              placeholder="One-line description (max 200 characters)"
              className="border rounded px-2 py-1 flex-1 max-w-md"
              autoFocus
            />
            <Button size="sm" onClick={handleSaveDescription} disabled={savingDescription}>
              Save
            </Button>
            <Button size="sm" variant="outline" onClick={handleCancelDescription} disabled={savingDescription}>
              Cancel
            </Button>
          </div>
        ) : (
          <div className="mt-1 flex items-center gap-2">
            {trackerDescription ? (
              <p>{trackerDescription}</p>
            ) : isOwner ? (
              <button
                type="button"
                onClick={() => { setDraftDescription(trackerDescription); setEditingDescription(true) }}
                className="text-muted-foreground text-sm italic hover:text-foreground transition-colors"
              >
                Add description...
              </button>
            ) : null}
            {trackerDescription && isOwner && (
              <button
                type="button"
                aria-label="Edit description"
                onClick={() => { setDraftDescription(trackerDescription); setEditingDescription(true) }}
                className="p-1 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
              >
                <Pencil className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        )}
      </div>

      <div className="bg-card border rounded-lg py-4 pl-2 pr-3 sm:px-4 shadow-md">
        {datasets.length > 0 ? (
          <>
            <TimeRangeSelector value={range} onChange={setRange} />
            <TrackerChart data={{ datasets }} chartConfig={chartDraft} min={min} max={max} />
          </>
        ) : (
          <div className="flex flex-col items-center gap-3 py-4 text-center">
            <p className="text-muted-foreground">No data to display</p>
            {isRoleOwner && (
              <Button variant="outline" size="sm" onClick={() => { setAddSeriesError(null); setNewSeriesName(seriesList.length === 0 ? tracker.name : ''); setAddSeriesOpen(true) }}>
                <Plus className="w-4 h-4" />
                Add Series
              </Button>
            )}
          </div>
        )}
      </div>

      {isRoleOwner && showSeriesSettings && (
        <div className="mt-6 bg-card border rounded-lg p-4 shadow-md">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl">Series</h2>
            <Button variant="outline" size="sm" onClick={() => setShowSeriesSettings(false)}>
              Close
            </Button>
          </div>
          <SeriesTable
            trackerId={tracker.id}
            seriesList={seriesList}
            yAxes={yAxes}
            onDeleteSeries={handleSeriesDeleted}
            onValuesCleared={handleValuesCleared}
          />
        </div>
      )}

      {isOwner && tracker.type !== 'coverage' && showAddValue && (
        <div className="mt-6 bg-card border rounded-lg p-4 shadow-md">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl">Data Points</h2>
            <Button variant="outline" size="sm" onClick={closeAddValue}>
              Close
            </Button>
          </div>
          {seriesList.length === 0 ? (
            <p className="text-muted-foreground">No series yet. Add a series first.</p>
          ) : (
            <div className="space-y-3">
              {seriesList.map((s) => {
                const input = valueInputs[s.id]
                const adding = addingValues[s.id]
                return (
                  <div key={s.id} className="flex items-center gap-2">
                    <span className="w-32 truncate text-sm font-medium">{s.name}</span>
                    <input
                      type={xAxisType === 'date' ? 'date' : 'datetime-local'}
                      value={input?.time ?? todayValue}
                      onChange={(e) => setValueInput(s.id, 'time', e.target.value)}
                      aria-label={`Date for ${s.name}`}
                      className="border rounded px-2 py-1"
                    />
                    <input
                      type="number"
                      value={input?.value ?? ''}
                      onChange={(e) => setValueInput(s.id, 'value', e.target.value)}
                      placeholder="Value"
                      aria-label={`Value for ${s.name}`}
                      className="border rounded px-2 py-1 w-32"
                    />
                    <Button
                      size="sm"
                      onClick={() => handleAddValue(s.id)}
                      disabled={adding || !input?.time || !input?.value}
                    >
                      {adding ? 'Adding...' : 'Add'}
                    </Button>
                  </div>
                )
              })}
              {addValueError && <p className="text-sm text-red-600">{addValueError}</p>}
            </div>
          )}
        </div>
      )}

      {isRoleOwner && showChartOptions && (
        <div className="mt-6 bg-card border rounded-lg p-4 shadow-md">
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-xl">Settings</h2>
            <Button variant="outline" size="sm" onClick={() => setShowChartOptions(false)}>
              Close
            </Button>
          </div>
          <SettingsForm
            initialConfig={chartDraft ?? chartConfig ?? {}}
            baselineConfig={chartConfig ?? {}}
            initialVisibility={trackerVisibility}
            onChange={setChartDraft}
            onSave={async (config, visibility) => {
              const updated = await patchTracker(tracker.id, { chart_config: JSON.stringify(config), visibility })
              setChartConfig(() => {
                try {
                  return normalizeChartConfig(JSON.parse(updated.chart_config) as ChartConfig)
                } catch {
                  return config
                }
              })
              setChartDraft(config)
              setTrackerVisibility(visibility ?? trackerVisibility)
            }}
            onCancel={setChartDraft}
          />
        </div>
      )}

      {editingBody ? (
        <div className="mt-6 bg-card border rounded-lg p-4 shadow-md">
          <div data-color-mode="light">
            <MDEditor
              value={draftBody}
              onChange={(v) => setDraftBody(v ?? '')}
              preview="live"
              height={300}
              textareaProps={{ placeholder: 'Write the body in Markdown...' }}
            />
          </div>
          <div className="flex items-center gap-2 mt-2">
            <Button size="sm" onClick={handleSaveBody} disabled={savingBody}>
              Save Body
            </Button>
            <Button size="sm" variant="outline" onClick={handleCancelBody} disabled={savingBody}>
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        (trackerBody?.trim() || isOwner) && (
          trackerBody?.trim() ? (
            <div className="mt-6 bg-card border rounded-lg p-4 shadow-md md-body relative">
              {isOwner && (
                <button
                  type="button"
                  aria-label="Edit body"
                  onClick={() => { setDraftBody(trackerBody); setEditingBody(true) }}
                  className="absolute top-2 right-2 p-1.5 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
                >
                  <Pencil className="w-4 h-4" />
                </button>
              )}
              <MDEditor.Markdown source={trackerBody} />
            </div>
          ) : (
            <div className="mt-6">
              <button
                type="button"
                aria-label="Edit body"
                onClick={() => { setDraftBody(trackerBody); setEditingBody(true) }}
                className="p-1.5 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
              >
                <Pencil className="w-4 h-4" />
              </button>
            </div>
          )
        )
      )}

      <AlertDialog.Root open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen}>
        <AlertDialog.Portal>
          <AlertDialog.Overlay className="fixed inset-0 z-50 bg-black/50" />
          <AlertDialog.Content className="fixed left-1/2 top-1/2 z-50 w-full max-w-md -translate-x-1/2 -translate-y-1/2 rounded-lg bg-popover text-popover-foreground border shadow-lg p-6">
            <AlertDialog.Title className="text-lg font-semibold">Delete Tracker?</AlertDialog.Title>
            <AlertDialog.Description className="mt-2 text-sm text-muted-foreground">
              This will permanently delete the tracker and all of its associated series and values. This action cannot be undone.
            </AlertDialog.Description>
            {deleteError && <p className="mt-2 text-sm text-red-600">{deleteError}</p>}
            <div className="mt-6 flex justify-end gap-2">
              <AlertDialog.Cancel asChild>
                <Button variant="outline" size="sm" disabled={deleting}>Cancel</Button>
              </AlertDialog.Cancel>
              <AlertDialog.Action asChild>
                <Button variant="destructive" size="sm" disabled={deleting} onClick={handleDelete}>
                  {deleting ? 'Deleting...' : 'Delete'}
                </Button>
              </AlertDialog.Action>
            </div>
          </AlertDialog.Content>
        </AlertDialog.Portal>
      </AlertDialog.Root>

      <Dialog.Root open={addSeriesOpen} onOpenChange={setAddSeriesOpen}>
        <Dialog.Portal>
          <Dialog.Overlay className="fixed inset-0 z-50 bg-black/50" />
          <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-full max-w-md -translate-x-1/2 -translate-y-1/2 rounded-lg bg-popover text-popover-foreground border shadow-lg p-6">
            <Dialog.Title className="text-lg font-semibold">Add Series</Dialog.Title>
            <Dialog.Description className="mt-2 text-sm text-muted-foreground">
              Add a new series to this tracker.
            </Dialog.Description>
            <div className="mt-4 space-y-4">
              <div className="flex items-center gap-2">
                <label htmlFor="add-series-name" className="text-sm w-20 shrink-0">Name</label>
                <input
                  id="add-series-name"
                  type="text"
                  value={newSeriesName}
                  onChange={(e) => setNewSeriesName(e.target.value)}
                  placeholder="Series name"
                  className="border rounded px-2 py-1 flex-1"
                  autoFocus
                />
              </div>
              <div className="flex items-center gap-2">
                <label htmlFor="add-series-datatype" className="text-sm w-20 shrink-0">Data Type</label>
                <select
                  id="add-series-datatype"
                  value={newSeriesDataType}
                  onChange={(e) => setNewSeriesDataType(e.target.value)}
                  className="border rounded px-2 py-1 flex-1"
                >
                  <option value="float">float</option>
                  <option value="int">int</option>
                </select>
              </div>
              <div className="flex items-center gap-2">
                <label htmlFor="add-series-charttype" className="text-sm w-20 shrink-0">Chart Type</label>
                <select
                  id="add-series-charttype"
                  value={newSeriesChartType}
                  onChange={(e) => setNewSeriesChartType(e.target.value as 'line' | 'bar')}
                  className="border rounded px-2 py-1 flex-1"
                >
                  <option value="line">Line</option>
                  <option value="bar">Bar</option>
                </select>
              </div>
            </div>
            {addSeriesError && <p className="mt-3 text-sm text-red-600">{addSeriesError}</p>}
            <div className="mt-6 flex justify-end gap-2">
              <Button variant="outline" size="sm" onClick={() => setAddSeriesOpen(false)} disabled={addingSeries}>
                Cancel
              </Button>
              <Button size="sm" disabled={!newSeriesName.trim() || addingSeries} onClick={handleAddSeries}>
                {addingSeries ? 'Adding...' : 'Add'}
              </Button>
            </div>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
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

const TrackerDetailRouter = (): React.JSX.Element => {
  const data = useLoaderData() as TrackerDetailData
  if (data.tracker.type === 'coverage') {
    throw new Response('Not Found', { status: 404 })
  }
  return <TrackerDetailView />
}

export const trackerRoute = [
  {
    index: true,
    element: null,
    loader: () => { throw new Response('Not Found', { status: 404 }) },
  },
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
]
