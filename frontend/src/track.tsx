import React, { useEffect, useState } from 'react'

import DatePicker from 'react-datepicker'

import ReactECharts from 'echarts-for-react'

import {
  Link,
  LoaderFunctionArgs,
  Params,
  useLoaderData,
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

interface TrackModel {
  id: number
  name: string
}

interface SeriesModel {
  id: number
  track_id: number
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

async function listTracks(): Promise<TrackModel[]> {
  const resp = await fetch('/api/track')
  if (!resp.ok) throw resp
  const data = await resp.json()
  return data.tracks ?? []
}

async function createTrack(name: string): Promise<TrackModel> {
  const resp = await fetch('/api/track', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

async function deleteTrack(id: number): Promise<void> {
  const resp = await fetch(`/api/track/${id}`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

async function listSeries(trackId: number): Promise<{ track: TrackModel; series: SeriesModel[] }> {
  const resp = await fetch(`/api/track/${trackId}/series`)
  if (!resp.ok) throw resp
  return resp.json()
}

async function createSeries(trackId: number, name: string, dataType: string): Promise<SeriesModel> {
  const resp = await fetch(`/api/track/${trackId}/series`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, data_type: dataType }),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

async function deleteSeries(trackId: number, seriesId: number): Promise<void> {
  const resp = await fetch(`/api/track/${trackId}/series/${seriesId}`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

async function createValue(trackId: number, seriesId: number, time: string, value: number): Promise<void> {
  const resp = await fetch(`/api/track/${trackId}/series/${seriesId}/values`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ time, value }),
  })
  if (!resp.ok) throw resp
}

async function deleteValues(trackId: number, seriesId: number): Promise<void> {
  const resp = await fetch(`/api/track/${trackId}/series/${seriesId}/values`, { method: 'DELETE' })
  if (!resp.ok) throw resp
}

export async function loadTrackList(): Promise<TrackModel[]> {
  return listTracks()
}

export async function loadTrackDetail({ params }: LoaderFunctionArgs): Promise<{ track: TrackModel; series: SeriesModel[] }> {
  if (!params.trackId) throw new Error('trackId is required')
  return listSeries(parseInt(params.trackId))
}

export const TrackList = (): React.JSX.Element => {
  const initial = useLoaderData() as TrackModel[]
  const [tracks, setTracks] = useState<TrackModel[]>(initial)
  const [newName, setNewName] = useState('')

  const handleCreate = async () => {
    if (!newName.trim()) return
    try {
      const created = await createTrack(newName.trim())
      setTracks((prev) => [...prev, created])
      setNewName('')
    } catch {
      // ignore
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await deleteTrack(id)
      setTracks((prev) => prev.filter((t) => t.id !== id))
    } catch {
      // ignore
    }
  }

  return (
    <div>
      <h1 className="text-3xl my-4">Tracks</h1>

      <div className="flex items-center gap-2 mb-4">
        <input
          type="text"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          placeholder="New track name"
          className="border rounded px-2 py-1"
          onKeyDown={(e) => { if (e.key === 'Enter') handleCreate() }}
        />
        <Button onClick={handleCreate} disabled={!newName.trim()}>Add Track</Button>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead className="w-24">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {tracks.length === 0 ? (
            <TableRow>
              <TableCell colSpan={2} className="text-center text-muted-foreground">
                No tracks yet
              </TableCell>
            </TableRow>
          ) : (
            tracks.map((t) => (
              <TableRow key={t.id}>
                <TableCell>
                  <Link
                    to={`/track/${t.id}`}
                    className="text-blue-600 dark:text-blue-500 hover:underline"
                  >
                    {t.name}
                  </Link>
                </TableCell>
                <TableCell>
                  <Button variant="destructive" size="sm" onClick={() => handleDelete(t.id)}>
                    Delete
                  </Button>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  )
}

interface Dataset {
  data: Array<{ x: string; y: string }>
  label: string
}

interface TrackChartProps {
  data?: { datasets: Dataset[] }
  min?: Date | null
  max?: Date | null
}

const TrackChart = (params: TrackChartProps): React.JSX.Element => {
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

export const TrackDetail = (): React.JSX.Element => {
  const data = useLoaderData() as { track: TrackModel; series: SeriesModel[] }
  const track = data.track
  const [seriesList, setSeriesList] = useState<SeriesModel[]>(data.series)
  const [seriesValues, setSeriesValues] = useState<SeriesValues[]>([])
  const [newSeriesName, setNewSeriesName] = useState('')
  const [newSeriesDataType, setNewSeriesDataType] = useState('float')

  const [selectedSeriesId, setSelectedSeriesId] = useState<number | null>(null)
  const [newValueTime, setNewValueTime] = useState('')
  const [newValueNumber, setNewValueNumber] = useState('')

  const [startDate, setStartDate] = useState<Date | null>(null)
  const [endDate, setEndDate] = useState<Date | null>(null)

  useEffect(() => {
    Promise.all(
      seriesList.map((s) =>
        fetch(`/api/track/${track.id}/series/${s.id}/values`)
          .then((r) => r.json() as Promise<{ values: ValueModel[] }>)
          .then((d) => ({ series: s, values: d.values ?? [] }))
      )
    ).then(setSeriesValues).catch(() => {})
  }, [seriesList, track.id])

  const handleCreateSeries = async () => {
    if (!newSeriesName.trim()) return
    try {
      const created = await createSeries(track.id, newSeriesName.trim(), newSeriesDataType)
      setSeriesList((prev) => [...prev, created])
      setNewSeriesName('')
    } catch {
      // ignore
    }
  }

  const handleDeleteSeries = async (seriesId: number) => {
    try {
      await deleteSeries(track.id, seriesId)
      setSeriesList((prev) => prev.filter((s) => s.id !== seriesId))
      setSeriesValues((prev) => prev.filter((sv) => sv.series.id !== seriesId))
    } catch {
      // ignore
    }
  }

  const handleAddValue = async () => {
    if (selectedSeriesId === null || !newValueTime || !newValueNumber) return
    try {
      await createValue(track.id, selectedSeriesId, new Date(newValueTime).toISOString(), parseFloat(newValueNumber))
      setNewValueTime('')
      setNewValueNumber('')
      const resp = await fetch(`/api/track/${track.id}/series/${selectedSeriesId}/values`)
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
      await deleteValues(track.id, seriesId)
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
        <Link to="/track" className="text-blue-600 dark:text-blue-500 hover:underline">
          &larr; Back to Tracks
        </Link>
      </div>

      <h1 className="text-3xl my-4">{track.name}</h1>

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
        <TrackChart data={{ datasets }} min={startDate} max={endDate} />
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
    </div>
  )
}

export const trackRoute = [
  {
    index: true,
    element: <TrackList />,
    loader: loadTrackList,
  },
  {
    path: ':trackId',
    handle: {
      crumb: (params: Params, data: any) => ({ label: data?.track?.name ?? 'Track' }),
    },
    loader: loadTrackDetail,
    element: <TrackDetail />,
  },
]
