import React, { useEffect, useImperativeHandle, useRef, useState } from 'react'

import { Pencil } from 'lucide-react'
import { patchSeries, deleteSeries } from './tracker-api'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SeriesConfig, SeriesModel, YAxisConfig } from './core'

export interface SeriesTableHandle {
  reassignYAxes: (removedIndex: number) => Promise<void>
}

interface SeriesTableProps {
  trackerId: number
  seriesList: SeriesModel[]
  yAxes: YAxisConfig[]
  onSelectSeries?: (seriesId: number) => void
  onDeleteSeries?: (seriesId: number) => void
  onRenameSeries?: (seriesId: number, name: string) => Promise<boolean>
  ref?: React.Ref<SeriesTableHandle>
}

export const SeriesTable = ({ trackerId, seriesList, yAxes, onSelectSeries, onDeleteSeries, onRenameSeries, ref }: SeriesTableProps): React.JSX.Element => {
  const [seriesValueFormats, setSeriesValueFormats] = useState<Record<number, string>>(() => {
    const map: Record<number, string> = {}
    for (const s of seriesList) {
      try {
        const cfg = JSON.parse(s.config) as SeriesConfig
        if (cfg.value_format) map[s.id] = cfg.value_format
      } catch { /* ignore */ }
    }
    return map
  })

  const [seriesTypes, setSeriesTypes] = useState<Record<number, 'line' | 'bar'>>(() => {
    const map: Record<number, 'line' | 'bar'> = {}
    for (const s of seriesList) {
      try {
        const cfg = JSON.parse(s.config) as SeriesConfig
        if (cfg.type) map[s.id] = cfg.type
      } catch { /* ignore */ }
    }
    return map
  })

  const [seriesYAxisIndices, setSeriesYAxisIndices] = useState<Record<number, number>>(() => {
    const map: Record<number, number> = {}
    for (const s of seriesList) {
      try {
        const cfg = JSON.parse(s.config) as SeriesConfig
        if (cfg.y_axis_index !== undefined) map[s.id] = cfg.y_axis_index
      } catch { /* ignore */ }
    }
    return map
  })

  useEffect(() => {
    setSeriesValueFormats((prev) => {
      const next: Record<number, string> = {}
      for (const s of seriesList) {
        try {
          const cfg = JSON.parse(s.config) as SeriesConfig
          if (cfg.value_format) next[s.id] = cfg.value_format
        } catch { /* ignore */ }
      }
      for (const id of Object.keys(prev)) {
        if (!seriesList.some((s) => s.id === Number(id))) delete next[Number(id)]
      }
      return { ...prev, ...next }
    })
    setSeriesTypes((prev) => {
      const next: Record<number, 'line' | 'bar'> = {}
      for (const s of seriesList) {
        try {
          const cfg = JSON.parse(s.config) as SeriesConfig
          if (cfg.type) next[s.id] = cfg.type
        } catch { /* ignore */ }
      }
      for (const id of Object.keys(prev)) {
        if (!seriesList.some((s) => s.id === Number(id))) delete next[Number(id)]
      }
      return { ...prev, ...next }
    })
      setSeriesYAxisIndices((prev) => {
        const next: Record<number, number> = {}
        for (const s of seriesList) {
          try {
            const cfg = JSON.parse(s.config) as SeriesConfig
            if (cfg.y_axis_index !== undefined) next[s.id] = cfg.y_axis_index
          } catch { /* ignore */ }
        }
        for (const id of Object.keys(prev)) {
          if (!seriesList.some((s) => s.id === Number(id))) delete next[Number(id)]
        }
        return { ...prev, ...next }
      })
  }, [seriesList])

  const seriesYAxisIndicesRef = useRef(seriesYAxisIndices)
  useEffect(() => { seriesYAxisIndicesRef.current = seriesYAxisIndices }, [seriesYAxisIndices])

  useImperativeHandle(ref, () => ({
    reassignYAxes: async (removedIndex: number) => {
      const current = seriesYAxisIndicesRef.current
      for (const [sidStr, yi] of Object.entries(current)) {
        const sid = Number(sidStr)
        if (yi === removedIndex) {
          await handleSeriesYAxisChange(sid, 0)
        } else if (yi > removedIndex) {
          await handleSeriesYAxisChange(sid, yi - 1)
        }
      }
    },
  }))

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

  const patchSeriesConfig = async (seriesId: number, config: SeriesConfig): Promise<SeriesModel | null> => {
    try {
      return await patchSeries(trackerId, seriesId, { config: JSON.stringify(config) })
    } catch {
      return null
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
    const config: SeriesConfig = { ...buildSeriesConfig(seriesId), value_format: fmt || undefined }
    await patchSeriesConfig(seriesId, config)
  }

  const handleSeriesTypeChange = async (seriesId: number, type: 'line' | 'bar') => {
    setSeriesTypes((prev) => ({ ...prev, [seriesId]: type }))
    const config: SeriesConfig = { ...buildSeriesConfig(seriesId), type }
    await patchSeriesConfig(seriesId, config)
  }

  const handleSeriesYAxisChange = async (seriesId: number, yAxisIndex: number) => {
    setSeriesYAxisIndices((prev) => ({ ...prev, [seriesId]: yAxisIndex }))
    const config: SeriesConfig = { ...buildSeriesConfig(seriesId), y_axis_index: yAxisIndex }
    await patchSeriesConfig(seriesId, config)
  }

  const handleDeleteSeries = async (seriesId: number) => {
    try {
      await deleteSeries(trackerId, seriesId)
      if (onDeleteSeries) onDeleteSeries(seriesId)
    } catch {
      // ignore
    }
  }

  return (
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
                {onSelectSeries ? (
                  <button
                    className="text-blue-600 dark:text-blue-500 hover:underline"
                    onClick={() => onSelectSeries(s.id)}
                  >
                    {s.name}
                  </button>
                ) : (
                  <NameCell
                    seriesId={s.id}
                    initialName={s.name}
                    existingNames={seriesList.map((x) => x.name)}
                    onRename={onRenameSeries}
                  />
                )}
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
                    .sort((a) => (a.position === 'left' ? -1 : 1))
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
              </TableCell>
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
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

function NameCell({ seriesId, initialName, existingNames, onRename }: { seriesId: number; initialName: string; existingNames: string[]; onRename?: (seriesId: number, name: string) => Promise<boolean> }): React.JSX.Element {
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState(initialName)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => { setValue(initialName); setSaved(false); setError(null) }, [initialName])

  const handleSave = async () => {
    const trimmed = value.trim()
    if (!trimmed) {
      setError('Name must not be empty')
      return
    }
    if (trimmed === initialName) {
      setEditing(false)
      setValue(initialName)
      setError(null)
      return
    }
    if (existingNames.some((n) => n === trimmed)) {
      setError('Name already exists')
      return
    }
    if (!onRename) return
    setSaving(true)
    setError(null)
    const ok = await onRename(seriesId, trimmed)
    setSaving(false)
    if (ok) {
      setSaved(true)
      setEditing(false)
    } else {
      setValue(initialName)
      setError('Failed to rename. Please try again.')
    }
  }

  const handleCancel = () => {
    setValue(initialName)
    setError(null)
    setSaved(false)
    setEditing(false)
  }

  if (!editing) {
    return (
      <div className="flex items-center gap-1">
        <span className="text-sm">{initialName}</span>
        {onRename && (
          <button
            onClick={() => { setValue(initialName); setEditing(true) }}
            className="text-muted-foreground hover:text-foreground"
            aria-label={`Rename series ${initialName}`}
          >
            <Pencil className="w-3.5 h-3.5" />
          </button>
        )}
      </div>
    )
  }

  return (
    <div className="flex items-center gap-1">
      <input
        type="text"
        value={value}
        onChange={(e) => { setValue(e.target.value); setSaved(false); setError(null) }}
        maxLength={200}
        autoFocus
        className="border rounded px-1 py-0.5 w-40 text-sm"
        onKeyDown={(e) => { if (e.key === 'Enter') handleSave(); if (e.key === 'Escape') handleCancel() }}
        aria-label={`Rename series ${initialName}`}
      />
      <Button size="sm" variant="outline" onClick={handleSave} disabled={saving}>
        {saving ? '...' : saved ? 'Saved' : 'Save'}
      </Button>
      <Button size="sm" variant="ghost" onClick={handleCancel} disabled={saving}>
        Cancel
      </Button>
      {error && <span className="text-xs text-red-600">{error}</span>}
    </div>
  )
}
