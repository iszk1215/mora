import React, { useEffect, useMemo, useState } from 'react'

import ReactECharts from 'echarts-for-react'
import MDEditor from '@uiw/react-md-editor'
import '@uiw/react-md-editor/markdown-editor.css'
import { MoreVertical, Pencil, Plus, SlidersHorizontal, Star, Trash } from 'lucide-react'
import { AlertDialog, Dialog, DropdownMenu } from 'radix-ui'

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
import { ChartOptionsForm } from './chart-options-form'
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

  const [editingTitle, setEditingTitle] = useState(false)
  const [editingDescription, setEditingDescription] = useState(false)
  const [editingBody, setEditingBody] = useState(false)
  const [showChartOptions, setShowChartOptions] = useState(false)
  const [addSeriesOpen, setAddSeriesOpen] = useState(false)
  const [newSeriesName, setNewSeriesName] = useState('')
  const [newSeriesDataType, setNewSeriesDataType] = useState('float')
  const [newSeriesChartType, setNewSeriesChartType] = useState<'line' | 'bar'>('line')
  const [addingSeries, setAddingSeries] = useState(false)
  const [addSeriesError, setAddSeriesError] = useState<string | null>(null)

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
      return JSON.parse(tracker.chart_config) as ChartConfig
    } catch {
      return null
    }
  })
  const [chartDraft, setChartDraft] = useState<ChartConfig | null>(chartConfig)

  const [range, setRange] = useState<TimeRangeKey>('all')
  const { min, max } = computeDateRange(range)

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
          {isOwner && (
            <Button variant="outline" size="sm" asChild>
              <Link to={`/trackers/${tracker.id}/edit`}>Edit</Link>
            </Button>
          )}
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
                    <SlidersHorizontal className="w-4 h-4" />
                    Chart Options
                  </DropdownMenu.Item>
                  <DropdownMenu.Item
                    className="flex items-center gap-2 rounded px-2 py-1.5 text-sm cursor-pointer outline-none data-[highlighted]:bg-accent"
                    onSelect={() => { setAddSeriesError(null); setNewSeriesName(seriesList.length === 0 ? tracker.name : ''); setAddSeriesOpen(true) }}
                  >
                    <Plus className="w-4 h-4" />
                    Add Series
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
            ) : (
              isOwner && <p className="text-muted-foreground italic">No description</p>
            )}
            {isOwner && (
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

      {isRoleOwner && showChartOptions && (
        <div className="mt-6 bg-card border rounded-lg p-4 shadow-md">
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-xl">Chart Options</h2>
            <Button variant="outline" size="sm" onClick={() => setShowChartOptions(false)}>
              Close
            </Button>
          </div>
          <ChartOptionsForm
            initialConfig={chartDraft ?? chartConfig ?? {}}
            baselineConfig={chartConfig ?? {}}
            onChange={setChartDraft}
            onSave={async (config) => {
              const updated = await patchTracker(tracker.id, { chart_config: JSON.stringify(config) })
              setChartConfig(() => {
                try {
                  return JSON.parse(updated.chart_config) as ChartConfig
                } catch {
                  return config
                }
              })
              setChartDraft(config)
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
            {trackerBody?.trim() ? (
              <MDEditor.Markdown source={trackerBody} />
            ) : (
              <p className="text-muted-foreground italic">No body content</p>
            )}
          </div>
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
  const [chartDraft, setChartDraft] = useState<ChartConfig | null>(null)
  const [description, setDescription] = useState(tracker.description ?? '')
  const [body, setBody] = useState(tracker.body ?? '')
  const [bodySaved, setBodySaved] = useState(false)
  const [visibility, setVisibility] = useState(tracker.visibility)
  const xAxisType = parsedChartConfig.x_axis_type ?? 'date'
  const yAxes: YAxisConfig[] = useMemo(() => {
    if (parsedChartConfig.y_axes && parsedChartConfig.y_axes.length > 0) {
      return parsedChartConfig.y_axes
    }
    return [{ id: 0, position: 'left' }]
  }, [parsedChartConfig])

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
    if (chartDraft) return chartDraft
    try {
      return JSON.parse(savedChartConfig) as ChartConfig
    } catch {
      return null
    }
  }, [chartDraft, savedChartConfig])

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
      <ChartOptionsForm
        initialConfig={parsedChartConfig}
        baselineConfig={parsedChartConfig}
        onChange={setChartDraft}
        onSave={async (config) => {
          try {
            const updated = await patchTracker(tracker.id, { chart_config: JSON.stringify(config) })
            setSavedChartConfig(updated.chart_config)
            setChartDraft(null)
          } catch {
            // ignore
          }
        }}
        onYAxesChange={async (config) => {
          try {
            const updated = await patchTracker(tracker.id, { chart_config: JSON.stringify(config) })
            setSavedChartConfig(updated.chart_config)
          } catch {
            // ignore
          }
        }}
        onRemoveYAxis={async (removed) => {
          const removedIndex = yAxes.findIndex((a) => a.position === removed.position)
          for (const [sidStr, yi] of Object.entries(seriesYAxisIndices)) {
            const sid = Number(sidStr)
            if (yi === removedIndex) {
              await handleSeriesYAxisChange(sid, 0)
            } else if (yi > removedIndex) {
              await handleSeriesYAxisChange(sid, yi - 1)
            }
          }
        }}
      />

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
  {
    path: ':trackerId/edit',
    loader: loadTrackerDetail,
    element: <TrackerDetailEditRouter />,
    handle: {
      crumb: () => ({ label: 'Edit' }),
    },
  },
]
