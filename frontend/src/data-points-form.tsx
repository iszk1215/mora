import React from 'react'

import { Trash } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ValueModel } from './tracker-api'

export interface EditDraft {
  time: string
  value: string
}

export interface DataPointEditCardProps {
  seriesName: string
  values: ValueModel[]
  total: number
  page: number
  perPage: number
  isDate: boolean
  dirty: boolean
  saving: boolean
  error: string | null
  pendingEdits: Record<string, EditDraft>
  onEdit: (valueId: string, which: 'time' | 'value', raw: string) => void
  onDelete: (valueId: string) => void
  onPerPageChange: (n: number) => void
  onPageChange: (n: number) => void
  onSave: () => void
  onCancel: () => void
  onClose: () => void
  levels?: string[]
}

const PER_PAGE_OPTIONS = [10, 20, 50, 100]

const pad = (n: number) => String(n).padStart(2, '0')

export function isoToInputValue(iso: string, isDate: boolean): string {
  if (isDate) {
    return iso.slice(0, 10)
  }
  const d = new Date(iso)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const displayTime = (v: ValueModel, isDate: boolean, pendingEdits: Record<string, EditDraft>): string =>
  pendingEdits[v.id]?.time ?? isoToInputValue(v.time, isDate)

const displayValue = (v: ValueModel, pendingEdits: Record<string, EditDraft>): string =>
  pendingEdits[v.id]?.value ?? String(v.value)

export const DataPointEditCard = ({
  seriesName,
  values,
  total,
  page,
  perPage,
  isDate,
  dirty,
  saving,
  error,
  pendingEdits,
  onEdit,
  onDelete,
  onPerPageChange,
  onPageChange,
  onSave,
  onCancel,
  onClose,
  levels,
}: DataPointEditCardProps): React.JSX.Element => {
  const totalPages = Math.max(1, Math.ceil(total / perPage))
  const inputType = isDate ? 'date' : 'datetime-local'

  return (
    <div className="mt-6 bg-card border rounded-lg p-4 shadow-md">
      <div className="flex flex-wrap items-center justify-between gap-2 mb-4">
        <h2 className="min-w-0 flex-1 truncate text-xl">{seriesName}</h2>
        <div className="flex shrink-0 items-center gap-2">
          <Button variant="outline" size="sm" onClick={onCancel} disabled={!dirty || saving}>
            Cancel
          </Button>
          <Button variant="outline" size="sm" onClick={onSave} disabled={!dirty || saving}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
          <Button variant="outline" size="sm" onClick={onClose} disabled={saving}>
            Close
          </Button>
        </div>
      </div>

      {values.length === 0 ? (
        <p className="text-muted-foreground">No data points yet. Add a value from the Data Points card.</p>
      ) : (
        <Table>
          <TableHeader className="hidden sm:table-header-group">
            <TableRow>
              <TableHead>{isDate ? 'Date' : 'Date and time'}</TableHead>
              <TableHead>Value</TableHead>
              <TableHead className="w-16">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {values.map((v) => (
              <TableRow key={v.id} className="block py-2 sm:table-row sm:py-0">
                <TableCell className="flex flex-wrap items-center gap-2 whitespace-normal sm:table-cell sm:whitespace-nowrap">
                  <span className="text-sm font-medium text-muted-foreground sm:hidden">{isDate ? 'Date' : 'Date and time'}</span>
                  <input
                    type={inputType}
                    value={displayTime(v, isDate, pendingEdits)}
                    onChange={(e) => onEdit(v.id, 'time', e.target.value)}
                    aria-label={`Data point time for ${seriesName}`}
                    className="border rounded px-2 py-1"
                  />
                </TableCell>
                <TableCell className="flex flex-wrap items-center gap-2 whitespace-normal sm:table-cell sm:whitespace-nowrap">
                  <span className="text-sm font-medium text-muted-foreground sm:hidden">Value</span>
                  {levels ? (
                    <div role="group" aria-label={`Value levels for ${seriesName}`} className="flex items-center gap-1">
                      {levels.map((c, i) => (
                        <button
                          key={i}
                          type="button"
                          aria-label={`Set value to level ${i + 1}`}
                          onClick={() => onEdit(v.id, 'value', String(i))}
                          className="inline-block w-5 h-5 rounded-sm border border-black/10 cursor-pointer"
                          style={{ backgroundColor: c }}
                        />
                      ))}
                    </div>
                  ) : (
                    <input
                      type="number"
                      step="any"
                      value={displayValue(v, pendingEdits)}
                      onChange={(e) => onEdit(v.id, 'value', e.target.value)}
                      aria-label={`Data point value for ${seriesName}`}
                      className="border rounded px-2 py-1 w-32"
                    />
                  )}
                </TableCell>
                <TableCell className="flex flex-wrap items-center gap-2 whitespace-normal sm:table-cell sm:whitespace-nowrap">
                  <span className="text-sm font-medium text-muted-foreground sm:hidden">Actions</span>
                  <Button
                    variant="destructive"
                    size="icon-sm"
                    onClick={() => onDelete(v.id)}
                    aria-label={`Delete data point ${v.id}`}
                  >
                    <Trash />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <div className="flex items-center justify-between gap-2 mt-4">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>Per page</span>
          <select
            value={perPage}
            onChange={(e) => onPerPageChange(parseInt(e.target.value))}
            className="border rounded px-1 py-0.5 text-sm"
            aria-label="Data points per page"
          >
            {PER_PAGE_OPTIONS.map((n) => (
              <option key={n} value={n}>{n}</option>
            ))}
          </select>
        </div>
        <div className="flex items-center gap-1">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
            Prev
          </Button>
          <span className="text-sm text-muted-foreground px-1">
            {page} of {totalPages}
          </span>
          <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => onPageChange(page + 1)}>
            Next
          </Button>
        </div>
      </div>

      {error && <p className="mt-3 text-sm text-red-600">{error}</p>}
    </div>
  )
}