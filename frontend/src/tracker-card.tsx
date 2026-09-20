import React, { useMemo } from 'react'
import { Link } from 'react-router'
import { Star } from 'lucide-react'

import { ChartConfig, SeriesConfig, TrackerResponse, normalizeChartConfig } from './core'
import { ReactECharts, CHART_THEME_NAME, areaGradient, formatValue, resolvePalette } from './chart'
import type { PreviewData } from './tracker-api'

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