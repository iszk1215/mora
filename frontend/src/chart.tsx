import React, { useEffect, useMemo, useRef } from 'react'
import ReactECharts from 'echarts-for-react'
import { ChartConfig, SeriesConfig } from './core'

export interface Dataset {
  data: Array<{ x: string; y: string; extra?: Record<string, any> }>
  label: string
  seriesConfig?: SeriesConfig
}

export function formatValue(value: number, fmt: string | undefined): string {
  if (fmt === undefined || fmt === '') {
    return Number.isInteger(value) ? String(value) : value.toFixed(1)
  }
  return fmt.replace(/%(?:\.(\d+))?([df])/g, (_match, precision, type) => {
    if (type === 'd') return String(Math.round(value))
    const p = precision !== undefined ? parseInt(precision) : 6
    return value.toFixed(p)
  }).replace(/%%/g, '%')
}

export interface TrackerChartProps {
  data?: { datasets: Dataset[] }
  min?: Date | null
  max?: Date | null
  chartConfig?: ChartConfig | null
  animation?: boolean
  onChartClick?: (params: any) => void
}

export const TrackerChart = (params: TrackerChartProps): React.JSX.Element => {
  const datasets = params.data?.datasets ?? []
  const cc = params.chartConfig
  const dataZoomAdded = useRef(false)

  const option = useMemo(() => {
    const showLegend = cc?.show_legend !== false && datasets.length > 1
    const opt: any = {
      grid: { left: 60, right: 20, top: showLegend ? 40 : 20, bottom: 60 },
      xAxis: {
        type: 'time' as const,
        splitLine: { show: false },
      },
      yAxis: {
        type: 'value' as const,
        splitLine: { lineStyle: { type: 'dashed' as const, opacity: 0.3 } },
      },
      series: datasets.map((ds) => ({
        name: ds.label,
        type: 'line' as const,
        data: ds.data.map((p) => ({ value: [p.x, Number(p.y)], ...p.extra })),
        areaStyle: cc?.area !== false ? { opacity: 0.12 } : undefined,
      })),
      tooltip: {
        trigger: 'axis',
        formatter: (params: any) => {
          const items = Array.isArray(params) ? params : [params]
          return items.map((p: any) => {
            const fmt = datasets[p.seriesIndex]?.seriesConfig?.value_format
            return `${p.marker} ${p.seriesName}: ${formatValue(p.value[1], fmt)}`
          }).join('<br/>')
        },
      },
    }
    if (params.animation === false) {
      opt.animation = false
    }
    if (!dataZoomAdded.current) {
      opt.dataZoom = [
        { type: 'inside' as const, xAxisIndex: 0, filterMode: 'none' as const },
        { type: 'slider' as const, xAxisIndex: 0, bottom: 10, filterMode: 'none' as const },
      ]
    }
    if (showLegend) {
      opt.legend = { type: 'scroll' as const, top: 0 }
    }
    if (cc) {
      if (cc.x_axis_label) opt.xAxis.name = cc.x_axis_label
      if (cc.y_axis_label) opt.yAxis.name = cc.y_axis_label
      if (cc.y_max !== undefined && cc.y_max > 0) opt.yAxis.max = cc.y_max
    }
    if (params.min) opt.xAxis.min = params.min
    if (params.max) opt.xAxis.max = params.max
    return opt
  }, [datasets, cc, params.min, params.max, params.animation])

  useEffect(() => {
    dataZoomAdded.current = true
  }, [])

  return (
    <ReactECharts
      option={option}
      style={{ width: '100%', height: 300 }}
      onEvents={params.onChartClick ? { click: params.onChartClick } : undefined}
      opts={{ renderer: 'svg' }}
    />
  )
}
