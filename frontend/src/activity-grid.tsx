import React, { useMemo } from 'react'

import {
  ACTIVITY_GRID_EMPTY_COLOR,
  CHART_THEME_NAME,
  DEFAULT_GRID_LEVELS,
  Dataset,
  MAX_GRID_LEVELS,
  ReactECharts,
  resolveGridColors,
} from './chart'
import { ChartConfig } from './core'

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

const MAX_GRID_DAYS = 1000

const pad = (n: number): string => String(n).padStart(2, '0')

export function addDaysUTC(base: string, delta: number): string {
  const d = new Date(`${base}T00:00:00.000Z`)
  d.setUTCDate(d.getUTCDate() + delta)
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`
}

function dayList(first: string, last: string): string[] {
  const days: string[] = []
  let cur = first
  while (cur <= last && days.length < MAX_GRID_DAYS) {
    days.push(cur)
    cur = addDaysUTC(cur, 1)
  }
  return days
}

const monthAxisLabel = (value: string): string => {
  if (!value.endsWith('-01')) return ''
  return MONTHS[parseInt(value.slice(5, 7), 10) - 1] ?? ''
}

const dayAxisLabel = (value: string): string => `${parseInt(value.slice(5, 7), 10)}/${parseInt(value.slice(8, 10), 10)}`

export const gridTooltipFormatter = (params: any): string => {
  const p = Array.isArray(params) ? params[0] : params
  const day = p.name ?? ''
  const label = day ? `<b>${dayAxisLabel(day)}</b>` : ''
  return `${p.marker} ${p.seriesName}${label ? `<br/>${label}` : ''}`
}

const firstDayOf = (value: Date | null | undefined): string => {
  if (!value) return ''
  return value.toISOString().slice(0, 10)
}

export interface ActivityGridChartProps {
  data?: { datasets: Dataset[] }
  min?: Date | null
  max?: Date | null
  chartConfig?: ChartConfig | null
  compact?: boolean
}

export const ActivityGridChart = ({ data, min, max, chartConfig, compact }: ActivityGridChartProps): React.JSX.Element => {
  const datasets = data?.datasets ?? []

  const option = useMemo(() => {
    if (datasets.length === 0) return null

    const levels = Math.min(chartConfig?.grid_levels ?? DEFAULT_GRID_LEVELS, MAX_GRID_LEVELS)
    const colors = resolveGridColors(chartConfig ?? undefined)

    const dataDays = [...new Set(datasets.flatMap((ds) => ds.data.map((p) => p.x.substring(0, 10))))].sort()
    if (dataDays.length === 0) return null

    const minDay = firstDayOf(min)
    const maxDay = firstDayOf(max)
    let first = minDay && minDay < dataDays[0] ? minDay : dataDays[0]
    let last = maxDay && maxDay > dataDays[dataDays.length - 1] ? maxDay : dataDays[dataDays.length - 1]
    if (first > last) {
      const tmp = first
      first = last
      last = tmp
    }
    if (addDaysUTC(first, MAX_GRID_DAYS) < last) {
      first = addDaysUTC(last, -(MAX_GRID_DAYS - 1))
    }

    const days = dayList(first, last)
    const seriesNames = datasets.map((ds) => ds.label)

    const byDay = datasets.map((ds) => {
      const map = new Map<string, number>()
      for (const p of ds.data) {
        const raw = Number(p.y)
        let level = 0
        if (!Number.isNaN(raw)) {
          level = Math.max(0, Math.min(Math.round(raw), levels - 1))
        }
        map.set(p.x.substring(0, 10), level)
      }
      return map
    })

    const gridData: Array<[number, number, number]> = []
    days.forEach((day, xi) => {
      byDay.forEach((map, yi) => {
        const level = map.get(day)
        gridData.push([xi, yi, level !== undefined ? level : -1])
      })
    })

    const pieces: any[] = [{ lt: 0, color: ACTIVITY_GRID_EMPTY_COLOR }]
    colors.forEach((color, i) => {
      if (i === colors.length - 1) {
        pieces.push({ gte: i, color })
      } else {
        pieces.push({ gte: i, lt: i + 1, color })
      }
    })

    return {
      animation: false,
      grid: {
        left: 'auto',
        right: 8,
        top: compact ? 4 : 18,
        bottom: 0,
        containLabel: true,
      },
      xAxis: {
        type: 'category' as const,
        data: days,
        splitLine: { show: false },
        splitArea: { show: false },
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: compact
          ? { show: false }
          : {
              formatter: monthAxisLabel,
              hideOverlap: true,
              fontSize: 10,
              color: '#888',
            },
      },
      yAxis: {
        type: 'category' as const,
        data: seriesNames,
        inverse: true,
        splitLine: { show: false },
        splitArea: { show: false },
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { fontSize: 10, hideOverlap: true },
      },
      visualMap: {
        type: 'piecewise' as const,
        show: false,
        dimension: 2,
        pieces,
      },
      tooltip: {
        trigger: 'item',
        formatter: gridTooltipFormatter,
      },
      series: [
        {
          type: 'heatmap',
          data: gridData,
          itemStyle: {
            borderWidth: 2,
            borderColor: '#fff',
            borderRadius: 3,
          },
          emphasis: {
            itemStyle: { borderColor: '#000', borderWidth: 1 },
          },
        },
      ],
    }
  }, [datasets, min, max, chartConfig, compact])

  const outer = useMemo(() => {
    const cell = compact ? 13 : 17
    const seriesCount = datasets.length
    const daysCount = option ? (option.xAxis.data as string[]).length : 0
    return {
      width: Math.max(daysCount * cell + (compact ? 40 : 70), 160),
      height: Math.max(seriesCount * cell + (compact ? 4 : 26), 32),
    }
  }, [option, compact, datasets.length])

  if (!option) {
    return (
      <div className="flex items-center justify-center py-4 text-center">
        <p className="text-muted-foreground">No data to display</p>
      </div>
    )
  }

  return (
    <div className="overflow-x-auto">
      <div style={{ width: outer.width, height: outer.height }}>
        <ReactECharts
          option={option}
          notMerge
          style={{ width: '100%', height: '100%' }}
          opts={{ renderer: 'svg' }}
          theme={CHART_THEME_NAME}
        />
      </div>
      {!compact && <ActivityGridLegend colors={resolveGridColors(chartConfig ?? undefined)} />}
    </div>
  )
}

export const ActivityGridLegend = ({ colors }: { colors: string[] }): React.JSX.Element => (
  <div className="mt-2 flex items-end justify-end gap-1" aria-hidden="true">
    <span
      className="inline-block w-3 h-3 rounded-sm"
      style={{ backgroundColor: ACTIVITY_GRID_EMPTY_COLOR }}
    />
    {colors.map((c, i) => (
      <span key={i} className="inline-block w-3 h-3 rounded-sm" style={{ backgroundColor: c }} />
    ))}
  </div>
)