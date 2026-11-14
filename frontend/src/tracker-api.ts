import type { LoaderFunctionArgs } from 'react-router'
import type { PreviewSeriesValues, SeriesModel, TrackerResponse } from './core'

export interface ValueModel {
  id: string
  time: string
  value: number
}

export interface SeriesValues {
  series: SeriesModel
  values: ValueModel[]
}

export interface TrackerDetailData {
  tracker: TrackerResponse
  series: SeriesModel[]
}

export interface PaginatedTrackers {
  trackers: TrackerResponse[]
  total: number
  page: number
  per_page: number
}

export interface PreviewData {
  tracker: TrackerResponse
  series: PreviewSeriesValues[]
}

// previewsFromTrackers builds the per-tracker preview map from a list response
// that was requested with include=preview. Trackers without preview data map to
// an empty series list so cards render their "No data" state.
export function previewsFromTrackers(trackers: TrackerResponse[]): Map<string, PreviewData> {
  const map = new Map<string, PreviewData>()
  for (const t of trackers) {
    map.set(t.id, { tracker: t, series: t.series ?? [] })
  }
  return map
}

export interface ValueUpdate {
  id: string
  time: string
  value: number
}

export async function listTrackers(page?: number, perPage?: number, query?: string, includePreview?: boolean): Promise<PaginatedTrackers> {
  const params = new URLSearchParams()
  if (page) params.set('page', String(page))
  if (perPage) params.set('per_page', String(perPage))
  if (query) params.set('q', query)
  if (includePreview) params.set('include', 'preview')
  const qs = params.toString()
  const url = qs ? `/api/trackers?${qs}` : '/api/trackers'
  const resp = await fetch(url)
  if (!resp.ok) throw resp
  return resp.json()
}

export async function createTracker(name: string, visibility: string, description?: string, type?: string): Promise<TrackerResponse> {
  const body: Record<string, unknown> = { name, visibility }
  if (description) body.description = description
  if (type) body.type = type
  const resp = await fetch('/api/trackers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

async function listSeries(trackerId: string): Promise<TrackerDetailData> {
  const resp = await fetch(`/api/trackers/${trackerId}/series`)
  if (!resp.ok) throw resp
  return resp.json()
}

export async function createSeries(trackerId: string, name: string, dataType: string, config?: string): Promise<SeriesModel> {
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

export async function patchSeries(trackerId: string, seriesId: string, opts: { name?: string; data_type?: string; config?: string }): Promise<SeriesModel> {
  const resp = await fetch(`/api/trackers/${trackerId}/series/${seriesId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

export async function deleteSeries(trackerId: string, seriesId: string): Promise<void> {
  const resp = await fetch(`/api/trackers/${trackerId}/series/${seriesId}`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

export async function createValue(trackerId: string, seriesId: string, time: string, value: number): Promise<void> {
  const resp = await fetch(`/api/trackers/${trackerId}/series/${seriesId}/values`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ time, value }),
  })
  if (!resp.ok) throw resp
}

export async function deleteValues(trackerId: string, seriesId: string): Promise<void> {
  const resp = await fetch(`/api/trackers/${trackerId}/series/${seriesId}/values`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

export async function patchValuesBatch(trackerId: string, seriesId: string, updates: ValueUpdate[], deletes: string[]): Promise<ValueModel[]> {
  const resp = await fetch(`/api/trackers/${trackerId}/series/${seriesId}/values`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ updates, deletes }),
  })
  if (!resp.ok) throw resp
  const data = await resp.json()
  return data.values ?? []
}

export async function likeTracker(trackerId: string): Promise<void> {
  const resp = await fetch(`/api/trackers/${trackerId}/like`, { method: 'POST' })
  if (!resp.ok) throw resp
}

export async function unlikeTracker(trackerId: string): Promise<void> {
  const resp = await fetch(`/api/trackers/${trackerId}/like`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

export async function patchTracker(trackerId: string, opts: { name?: string; visibility?: string; chart_config?: string; description?: string; body?: string }): Promise<TrackerResponse> {
  const resp = await fetch(`/api/trackers/${trackerId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

export async function deleteTracker(trackerId: string): Promise<void> {
  const resp = await fetch(`/api/trackers/${trackerId}`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

export async function loadTrackerDetail({ params }: LoaderFunctionArgs): Promise<TrackerDetailData> {
  if (!params.trackerId) throw new Error('trackerId is required')
  return listSeries(params.trackerId!)
}