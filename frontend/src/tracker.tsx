import React, { useEffect, useState } from 'react'

import DatePicker from 'react-datepicker'

import ReactECharts from 'echarts-for-react'

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
import { TrackerResponse } from './core'

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
}

interface TrackerDetailData {
  tracker: TrackerResponse
  series: SeriesModel[]
}

async function listTrackers(): Promise<TrackerResponse[]> {
  const resp = await fetch('/api/tracker')
  if (!resp.ok) throw resp
  const data = await resp.json()
  return data.trackers ?? []
}

async function createTracker(name: string, visibility: string): Promise<TrackerResponse> {
  const resp = await fetch('/api/tracker', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, visibility }),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

async function deleteTracker(id: number): Promise<void> {
  const resp = await fetch(`/api/tracker/${id}`, { method: 'DELETE' })
  if (!resp.ok) throw resp
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

export async function patchTracker(trackerId: number, visibility: string): Promise<TrackerResponse> {
  const resp = await fetch(`/api/tracker/${trackerId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ visibility }),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

export async function loadTrackerList(): Promise<TrackerResponse[]> {
  return listTrackers()
}

export async function loadTrackerDetail({ params }: LoaderFunctionArgs): Promise<TrackerDetailData> {
  if (!params.trackerId) throw new Error('trackerId is required')
  return listSeries(parseInt(params.trackerId))
}

const TrackerChart = (params: TrackerChartProps): React.JSX.Element => {
  const datasets = params.data?.datasets ?? []

  const option: any = {
    grid: { left: 60, right: 20, top: 20, bottom: 40 },
    xAxis: {
      type: 'time' as const,
    },
    yAxis: {
      type: 'value' as const,
    },
    series: datasets.map((ds) => ({
      name: ds.label,
      type: 'line' as const,
      data: ds.data.map((p) => [p.x, Number(p.y)]),
    })),
    tooltip: {
      trigger: 'axis',
      valueFormatter: (value: number) =>
        Number.isInteger(value) ? String(value) : value.toFixed(1),
    },
  }

  if (params.min) {
    option.xAxis.min = params.min
  }
  if (params.max) {
    option.xAxis.max = params.max
  }

  return (
    <ReactECharts option={option} style={{ width: '100%', height: 300 }} />
  )
}

export const TrackerView = (): React.JSX.Element => {
  const initial = useLoaderData() as TrackerResponse[]
  const [trackers, setTrackers] = useState<TrackerResponse[]>(initial)

  const handleDelete = async (id: number) => {
    try {
      await deleteTracker(id)
      setTrackers((prev) => prev.filter((t) => t.id !== id))
    } catch {
      // ignore
    }
  }

  const myTrackers = trackers.filter((t) => t.role !== '')
  const likedTrackers = trackers.filter((t) => t.liked)

  const renderTrackerTable = (rows: TrackerResponse[], showActions: boolean) => (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          {showActions && <TableHead className="w-24">Actions</TableHead>}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.length === 0 ? (
          <TableRow>
            <TableCell colSpan={showActions ? 2 : 1} className="text-center text-muted-foreground">
              None
            </TableCell>
          </TableRow>
        ) : (
          rows.map((t) => (
            <TableRow key={t.id}>
              <TableCell>
                <Link
                  to={`/tracker/${t.id}`}
                  className="text-blue-600 dark:text-blue-500 hover:underline"
                >
                  {t.name}
                </Link>
                {t.liked && <span className="ml-2 text-sm text-muted-foreground">&#9829;</span>}
              </TableCell>
              {showActions && (
                <TableCell>
                  <Button variant="destructive" size="sm" onClick={() => handleDelete(t.id)}>
                    Delete
                  </Button>
                </TableCell>
              )}
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
  )

  return (
    <div>
      <h1 className="text-3xl my-4">Trackers</h1>

      <div className="mb-4">
        <Button asChild><Link to="/tracker/new">Create Tracker</Link></Button>
      </div>

      {myTrackers.length > 0 && (
        <div className="mb-6">
          <h2 className="text-xl my-2">My Trackers</h2>
          {renderTrackerTable(myTrackers, true)}
        </div>
      )}

      <div>
        <h2 className="text-xl my-2">Liked Trackers</h2>
        {renderTrackerTable(likedTrackers, false)}
      </div>
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

  const datasets: Dataset[] = seriesValues.map(valuesToDataset)

  return (
    <div>
      <div className="flex items-center gap-2 mb-4">
        <Link to="/tracker" className="text-blue-600 dark:text-blue-500 hover:underline">
          &larr; Back to Trackers
        </Link>
      </div>

      <div className="flex items-center gap-3 my-4">
        <h1 className="text-3xl">{tracker.name}</h1>
        <Button
          variant={liked ? 'default' : 'secondary'}
          size="sm"
          onClick={handleLikeToggle}
          disabled={likeLoading}
        >
          {liked ? 'Unlike' : 'Like'}
        </Button>
        {tracker.role !== '' && (
          <Button variant="outline" size="sm" asChild>
            <Link to={`/tracker/${tracker.id}/edit`}>Edit</Link>
          </Button>
        )}
      </div>

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
        <TrackerChart data={{ datasets }} min={startDate} max={endDate} />
      ) : (
        <p className="text-muted-foreground">No data to display</p>
      )}

      {/* Series list (read-only) */}
      <h2 className="text-xl my-2">Series</h2>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Data Type</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {seriesList.length === 0 ? (
            <TableRow>
              <TableCell colSpan={2} className="text-center text-muted-foreground">
                No series yet
              </TableCell>
            </TableRow>
          ) : (
            seriesList.map((s) => (
              <TableRow key={s.id}>
                <TableCell>{s.name}</TableCell>
                <TableCell>{s.data_type}</TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
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

  const [visibility, setVisibility] = useState(tracker.visibility)

  const handleVisibilityChange = async (newVisibility: string) => {
    try {
      await patchTracker(tracker.id, newVisibility)
      setVisibility(newVisibility)
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

  const datasets: Dataset[] = seriesValues.map(valuesToDataset)

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
        <TrackerChart data={{ datasets }} min={startDate} max={endDate} />
      ) : (
        <p className="text-muted-foreground">No data to display</p>
      )}

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
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const handleCreate = async () => {
    if (!name.trim()) return
    setLoading(true)
    setError(null)
    try {
      const created = await createTracker(name.trim(), visibility)
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
    handle: {
      crumb: (params: Params, data: any) => ({ label: data?.tracker?.name ?? 'Tracker' }),
    },
    loader: loadTrackerDetail,
    element: <TrackerDetailView />,
  },
  {
    path: ':trackerId/edit',
    handle: {
      crumb: (params: Params, data: any) => ({ label: data?.tracker?.name ?? 'Tracker' }),
    },
    loader: loadTrackerDetail,
    element: <TrackerDetailEdit />,
  },
]
