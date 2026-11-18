import React, { useEffect, useImperativeHandle, useRef, useState } from 'react'

import { GripVertical, Pencil } from 'lucide-react'
import { patchSeries, deleteSeries, reorderSeries } from './tracker-api'
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
  trackerId: string
  seriesList: SeriesModel[]
  yAxes: YAxisConfig[]
  onSelectSeries?: (seriesId: string) => void
  onDeleteSeries?: (seriesId: string) => void
  onRenameSeries?: (seriesId: string, name: string) => Promise<boolean>
  onReorderSeries?: (seriesList: SeriesModel[]) => void
  ref?: React.Ref<SeriesTableHandle>
}

export const SeriesTable = ({ trackerId, seriesList, yAxes, onSelectSeries, onDeleteSeries, onRenameSeries, onReorderSeries, ref }: SeriesTableProps): React.JSX.Element => {
  const [seriesValueFormats, setSeriesValueFormats] = useState<Record<string, string>>(() => {
    const map: Record<string, string> = {}
    for (const s of seriesList) {
      try {
        const cfg = JSON.parse(s.config) as SeriesConfig
        if (cfg.value_format) map[s.id] = cfg.value_format
      } catch { /* ignore */ }
    }
    return map
  })

  const [seriesTypes, setSeriesTypes] = useState<Record<string, 'line' | 'bar'>>(() => {
    const map: Record<string, 'line' | 'bar'> = {}
    for (const s of seriesList) {
      try {
        const cfg = JSON.parse(s.config) as SeriesConfig
        if (cfg.type) map[s.id] = cfg.type
      } catch { /* ignore */ }
    }
    return map
  })

  const [seriesYAxisIndices, setSeriesYAxisIndices] = useState<Record<string, number>>(() => {
    const map: Record<string, number> = {}
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
      const next: Record<string, string> = {}
      for (const s of seriesList) {
        try {
          const cfg = JSON.parse(s.config) as SeriesConfig
          if (cfg.value_format) next[s.id] = cfg.value_format
        } catch { /* ignore */ }
      }
      for (const id of Object.keys(prev)) {
        if (!seriesList.some((s) => s.id === id)) delete next[id]
      }
      return { ...prev, ...next }
    })
    setSeriesTypes((prev) => {
      const next: Record<string, 'line' | 'bar'> = {}
      for (const s of seriesList) {
        try {
          const cfg = JSON.parse(s.config) as SeriesConfig
          if (cfg.type) next[s.id] = cfg.type
        } catch { /* ignore */ }
      }
      for (const id of Object.keys(prev)) {
        if (!seriesList.some((s) => s.id === id)) delete next[id]
      }
      return { ...prev, ...next }
    })
      setSeriesYAxisIndices((prev) => {
        const next: Record<string, number> = {}
        for (const s of seriesList) {
          try {
            const cfg = JSON.parse(s.config) as SeriesConfig
            if (cfg.y_axis_index !== undefined) next[s.id] = cfg.y_axis_index
          } catch { /* ignore */ }
        }
        for (const id of Object.keys(prev)) {
          if (!seriesList.some((s) => s.id === id)) delete next[id]
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
        const sid = sidStr
        if (yi === removedIndex) {
          await handleSeriesYAxisChange(sid, 0)
        } else if (yi > removedIndex) {
          await handleSeriesYAxisChange(sid, yi - 1)
        }
      }
    },
  }))

  const buildSeriesConfig = (seriesId: string): SeriesConfig => {
    const config: SeriesConfig = {}
    const fmt = seriesValueFormats[seriesId]
    if (fmt) config.value_format = fmt
    const t = seriesTypes[seriesId]
    if (t) config.type = t
    const yi = seriesYAxisIndices[seriesId]
    if (yi !== undefined) config.y_axis_index = yi
    return config
  }

  const patchSeriesConfig = async (seriesId: string, config: SeriesConfig): Promise<SeriesModel | null> => {
    try {
      return await patchSeries(trackerId, seriesId, { config: JSON.stringify(config) })
    } catch {
      return null
    }
  }

  const handleSaveValueFormat = async (seriesId: string, fmt: string) => {
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

  const handleSeriesTypeChange = async (seriesId: string, type: 'line' | 'bar') => {
    setSeriesTypes((prev) => ({ ...prev, [seriesId]: type }))
    const config: SeriesConfig = { ...buildSeriesConfig(seriesId), type }
    await patchSeriesConfig(seriesId, config)
  }

  const handleSeriesYAxisChange = async (seriesId: string, yAxisIndex: number) => {
    setSeriesYAxisIndices((prev) => ({ ...prev, [seriesId]: yAxisIndex }))
    const config: SeriesConfig = { ...buildSeriesConfig(seriesId), y_axis_index: yAxisIndex }
    await patchSeriesConfig(seriesId, config)
  }

  const handleDeleteSeries = async (seriesId: string) => {
    try {
      await deleteSeries(trackerId, seriesId)
      if (onDeleteSeries) onDeleteSeries(seriesId)
    } catch {
      // ignore
    }
  }

  const [reordering, setReordering] = useState(false)
  const [draggedId, setDraggedId] = useState<string | null>(null)
  const [dragOverId, setDragOverId] = useState<string | null>(null)
  const orderRef = useRef(seriesList)
  const initialOrderRef = useRef<SeriesModel[] | null>(null)
  const dropCommittedRef = useRef(false)

  useEffect(() => { orderRef.current = seriesList }, [seriesList])

  const commitReorder = async () => {
    const initialOrder = initialOrderRef.current
    setDraggedId(null)
    setDragOverId(null)
    if (!initialOrder) return

    const finalOrder = orderRef.current
    const changed = finalOrder.map((s) => s.id).join(',') !== initialOrder.map((s) => s.id).join(',')
    if (changed) {
      setReordering(true)
      try {
        await reorderSeries(trackerId, finalOrder.map((s) => s.id))
      } catch {
        onReorderSeries?.(initialOrder)
        orderRef.current = initialOrder
      } finally {
        setReordering(false)
      }
    }
    initialOrderRef.current = null
  }

  const handleDragStart = (event: React.DragEvent<HTMLTableRowElement>, seriesId: string) => {
    if (!onReorderSeries || reordering) {
      event.preventDefault()
      return
    }
    initialOrderRef.current = seriesList
    orderRef.current = seriesList
    dropCommittedRef.current = false
    setDraggedId(seriesId)
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData('text/plain', seriesId)
    }
  }

  const handleDragOver = (event: React.DragEvent<HTMLTableRowElement>, seriesId: string, targetIndex: number) => {
    if (!draggedId || !onReorderSeries || reordering) return
    event.preventDefault()
    if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
    setDragOverId(seriesId)

    const current = orderRef.current
    const from = current.findIndex((s) => s.id === draggedId)
    if (from < 0 || from === targetIndex) return

    const reordered = [...current]
    const [moved] = reordered.splice(from, 1)
    reordered.splice(targetIndex, 0, moved)
    orderRef.current = reordered
    onReorderSeries(reordered)
  }

  const handleDrop = (event: React.DragEvent<HTMLTableRowElement>) => {
    if (!draggedId) return
    event.preventDefault()
    if (dropCommittedRef.current) return
    dropCommittedRef.current = true
    void commitReorder()
  }

  const handleDragEnd = () => {
    if (!dropCommittedRef.current && initialOrderRef.current) {
      onReorderSeries?.(initialOrderRef.current)
      orderRef.current = initialOrderRef.current
      initialOrderRef.current = null
    }
    setDraggedId(null)
    setDragOverId(null)
  }

  return (
    <Table>
      <TableHeader className="hidden sm:table-header-group">
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
          <TableRow className="block sm:table-row">
            <TableCell colSpan={6} className="block text-center text-muted-foreground sm:table-cell">
              No series yet
            </TableCell>
          </TableRow>
        ) : (
          seriesList.map((s, index) => (
            <TableRow
              key={s.id}
              className={`block py-2 sm:table-row sm:py-0 ${draggedId === s.id ? 'opacity-50' : ''} ${dragOverId === s.id && draggedId !== s.id ? 'bg-muted/50' : ''}`}
              draggable={!!onReorderSeries && !reordering}
              onDragStart={(event) => handleDragStart(event, s.id)}
              onDragOver={(event) => handleDragOver(event, s.id, index)}
              onDrop={handleDrop}
              onDragEnd={handleDragEnd}
              onDragLeave={() => setDragOverId((current) => (current === s.id ? null : current))}
            >
              <TableCell className="flex flex-wrap items-center gap-2 whitespace-normal sm:table-cell sm:whitespace-nowrap">
                <span className="text-sm font-medium text-muted-foreground sm:hidden">Name</span>
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
              <TableCell className="flex flex-wrap items-center gap-2 whitespace-normal sm:table-cell sm:whitespace-nowrap">
                <span className="text-sm font-medium text-muted-foreground sm:hidden">Data Type</span>
                {s.data_type}
              </TableCell>
              <TableCell className="flex flex-wrap items-center gap-2 whitespace-normal sm:table-cell sm:whitespace-nowrap">
                <span className="text-sm font-medium text-muted-foreground sm:hidden">Chart Type</span>
                <select
                  value={seriesTypes[s.id] ?? 'line'}
                  onChange={(e) => handleSeriesTypeChange(s.id, e.target.value as 'line' | 'bar')}
                  className="border rounded px-1 py-0.5 text-sm"
                >
                  <option value="line">Line</option>
                  <option value="bar">Bar</option>
                </select>
              </TableCell>
              <TableCell className="flex flex-wrap items-center gap-2 whitespace-normal sm:table-cell sm:whitespace-nowrap">
                <span className="text-sm font-medium text-muted-foreground sm:hidden">Y-Axis</span>
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
              <TableCell className="flex flex-wrap items-center gap-2 whitespace-normal sm:table-cell sm:whitespace-nowrap">
                <span className="text-sm font-medium text-muted-foreground sm:hidden">Value Format</span>
                <ValueFormatCell
                  seriesId={s.id}
                  initialFormat={seriesValueFormats[s.id] ?? ''}
                  onSave={handleSaveValueFormat}
                />
              </TableCell>
              <TableCell className="flex flex-wrap items-center gap-2 whitespace-normal sm:table-cell sm:whitespace-nowrap">
                <span className="text-sm font-medium text-muted-foreground sm:hidden">Actions</span>
                {onReorderSeries && (
                  <span
                    aria-label={`Drag ${s.name} to reorder`}
                    title="Drag to reorder"
                    className="cursor-grab text-muted-foreground"
                  >
                    <GripVertical className="w-4 h-4" />
                  </span>
                )}
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

function ValueFormatCell({ seriesId, initialFormat, onSave }: { seriesId: string; initialFormat: string; onSave: (seriesId: string, fmt: string) => void }): React.JSX.Element {
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

function NameCell({ seriesId, initialName, existingNames, onRename }: { seriesId: string; initialName: string; existingNames: string[]; onRename?: (seriesId: string, name: string) => Promise<boolean> }): React.JSX.Element {
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
