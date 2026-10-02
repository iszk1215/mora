import React, { useMemo, useRef, useState } from 'react'

import { Check, ChevronDown, ChevronLeft, ChevronRight } from 'lucide-react'
import { DropdownMenu } from 'radix-ui'

import { Button } from '@/components/ui/button'
import type { SeriesModel } from './core'

export type QuickAddValue = {
  seriesId: number
  time: string
  value: number
}

export type ValueInputProps = {
  seriesList: SeriesModel[]
  isDate: boolean
  defaultDate: string
  onAdd: (added: QuickAddValue) => Promise<void>
}

export const shiftDate = (base: string, deltaDays: number, isDate: boolean): string => {
  const pad = (n: number) => String(n).padStart(2, '0')
  if (isDate) {
    const [y, m, d] = base.split('-').map(Number)
    const dt = new Date(y, m - 1, d + deltaDays)
    return `${dt.getFullYear()}-${pad(dt.getMonth() + 1)}-${pad(dt.getDate())}`
  }
  const dt = new Date(base)
  dt.setDate(dt.getDate() + deltaDays)
  return `${dt.getFullYear()}-${pad(dt.getMonth() + 1)}-${pad(dt.getDate())}T${pad(dt.getHours())}:${pad(dt.getMinutes())}`
}

export const ValueInput = ({ seriesList, isDate, defaultDate, onAdd }: ValueInputProps): React.JSX.Element => {
  const [date, setDate] = useState(defaultDate)
  const [selectedId, setSelectedId] = useState<number | null>(seriesList[0]?.id ?? null)
  const [value, setValue] = useState('')
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const valueRef = useRef<HTMLInputElement>(null)

  const seriesId = useMemo(
    () => (selectedId != null && seriesList.some((s) => s.id === selectedId) ? selectedId : seriesList[0]?.id ?? null),
    [selectedId, seriesList]
  )
  const selected = seriesId != null ? seriesList.find((s) => s.id === seriesId) : undefined

  const submit = async () => {
    if (adding || seriesId == null || !date || !value.trim()) return
    setAdding(true)
    setError(null)
    try {
      await onAdd({ seriesId, time: new Date(date).toISOString(), value: parseFloat(value) })
      setValue('')
      valueRef.current?.focus()
    } catch {
      setError('Failed to add value. Please try again.')
    } finally {
      setAdding(false)
    }
  }

  return (
    <div role="group" aria-label="Add value" className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center">
      <div className="flex w-full items-center gap-2 sm:w-auto">
        <Button
          variant="outline"
          size="icon-sm"
          aria-label="Previous day"
          onClick={() => setDate((cur) => shiftDate(cur, -1, isDate))}
        >
          <ChevronLeft className="w-4 h-4" />
        </Button>
        <input
          type={isDate ? 'date' : 'datetime-local'}
          value={date}
          onChange={(e) => setDate(e.target.value)}
          aria-label="Value date"
          className="border rounded px-2 py-1 flex-1 sm:flex-none"
        />
        <Button
          variant="outline"
          size="icon-sm"
          aria-label="Next day"
          onClick={() => setDate((cur) => shiftDate(cur, 1, isDate))}
        >
          <ChevronRight className="w-4 h-4" />
        </Button>
      </div>

      <div className="w-full sm:w-44">
        <DropdownMenu.Root>
          <DropdownMenu.Trigger asChild>
            <button
              type="button"
              aria-label="Series"
              className="flex w-full items-center justify-between gap-1 border rounded px-2 py-1 text-sm"
            >
              <span className="truncate">{selected?.name ?? ''}</span>
              <ChevronDown className="w-4 h-4 shrink-0" />
            </button>
          </DropdownMenu.Trigger>
          <DropdownMenu.Portal>
            <DropdownMenu.Content align="start" className="bg-popover text-popover-foreground rounded-md border shadow-md p-1 min-w-[12rem] z-50">
              <DropdownMenu.RadioGroup
                value={seriesId != null ? String(seriesId) : ''}
                onValueChange={(v) => setSelectedId(parseInt(v))}
              >
                {seriesList.map((s) => (
                  <DropdownMenu.RadioItem
                    key={s.id}
                    value={String(s.id)}
                    className="flex items-center gap-2 rounded px-2 py-1.5 text-sm cursor-pointer outline-none data-[highlighted]:bg-accent"
                  >
                    <DropdownMenu.ItemIndicator>
                      <Check className="w-4 h-4" />
                    </DropdownMenu.ItemIndicator>
                    <span className="truncate">{s.name}</span>
                  </DropdownMenu.RadioItem>
                ))}
              </DropdownMenu.RadioGroup>
            </DropdownMenu.Content>
          </DropdownMenu.Portal>
        </DropdownMenu.Root>
      </div>

      <div className="flex items-center gap-2 sm:flex-1">
        <input
          ref={valueRef}
          type="number"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder="Value"
          aria-label="Value"
          className="border rounded px-2 py-1 flex-1 min-w-0 sm:flex-none sm:w-32"
        />
        <Button size="sm" onClick={submit} disabled={adding || seriesId == null || !date || !value.trim()}>
          {adding ? 'Adding...' : 'Add'}
        </Button>
      </div>

      {error && <p className="text-sm text-red-600">{error}</p>}
    </div>
  )
}