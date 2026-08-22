import React, { useEffect, useMemo, useRef, useState } from 'react'
import ReactECharts from 'echarts-for-react'
import * as echarts from 'echarts'
import { ChartConfig, SeriesConfig, YAxisConfig } from './core'

// Must match --font-sans in index.css so chart text uses the page font.
export const CHART_FONT_FAMILY = "'Noto Sans JP Variable', 'Noto Sans JP', sans-serif"

export const CHART_THEME_NAME = 'mora'

echarts.registerTheme(CHART_THEME_NAME, {
  textStyle: { fontFamily: CHART_FONT_FAMILY },
})

export const PALETTE_MAP: Record<string, string[]> = {
  default:  ['#5470c6', '#91cc75', '#fac858', '#ee6666', '#73c0de', '#3ba272', '#fc8452', '#9a60b4', '#ea7ccc'],
  vintage:  ['#c23531', '#2f4554', '#61a0a8', '#d48265', '#91c7ae', '#749f83', '#ca8622', '#bda29a', '#6e7074', '#546570', '#c4ccd3'],
  dark:     ['#dd6b66', '#759aa0', '#e69d87', '#8dc1a9', '#ea7e53', '#eedd78', '#73a373', '#73b9bc', '#7289ab', '#91ca8c', '#f49f42'],
  infographic: ['#c1232b', '#27727b', '#fcce10', '#e87c25', '#b5c334', '#fe8463', '#9bca63', '#fad860', '#f3a43b', '#60c0dd', '#d7504b', '#c6e579', '#f4e001', '#f0805a', '#26c0c0'],
  macarons: ['#2ec7c9', '#b6a2de', '#5ab1ef', '#ffb980', '#d87a80', '#8d98b3', '#e5cf0d', '#97b552', '#95706d', '#dc69aa', '#07a2a4', '#9a7fd1', '#588dd5', '#f5994e', '#c05050'],
  essos:    ['#893448', '#d95850', '#eb8146', '#ffb248', '#f2d643', '#ebdba4'],
  halloween: ['#ff715e', '#ffaf51', '#ffee51', '#8c6ac4', '#715c87'],
  purple:   ['#9b8bba', '#e098c7', '#8fd3e8', '#71669e', '#cc70af', '#7cb4cc'],
}

export const PALETTE_NAMES = Object.keys(PALETTE_MAP)

export function resolvePalette(name?: string): string[] {
  if (!name) return PALETTE_MAP.default
  return PALETTE_MAP[name] ?? PALETTE_MAP.default
}

export function areaGradient(hex: string, topAlpha = 0.35): object {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return {
    color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
      { offset: 0, color: `rgba(${r}, ${g}, ${b}, ${topAlpha})` },
      { offset: 1, color: `rgba(${r}, ${g}, ${b}, 0)` },
    ]),
  }
}

export interface Dataset {
  data: Array<{ x: string; y: string; extra?: Record<string, any> }>
  label: string
  seriesConfig?: SeriesConfig
}

const NARROW_MEDIA_QUERY = '(max-width: 639px)'

function useIsNarrowViewport(): boolean {
  const [narrow, setNarrow] = useState(() =>
    typeof window !== 'undefined' && typeof window.matchMedia === 'function'
      ? window.matchMedia(NARROW_MEDIA_QUERY).matches
      : false,
  )
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return
    const mq = window.matchMedia(NARROW_MEDIA_QUERY)
    const onChange = (e: MediaQueryListEvent): void => setNarrow(e.matches)
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [])
  return narrow
}

export function formatDateTick(value: number): string {
  const d = new Date(value)
  const m = d.getMonth() + 1
  const day = d.getDate()
  if (d.getFullYear() === new Date().getFullYear()) {
    return `${m}/${day}`
  }
  return `${d.getFullYear()}/${m}/${day}`
}

export function formatDateTimeTick(value: number): string {
  const d = new Date(value)
  const base = `${d.getMonth() + 1}/${d.getDate()}`
  if (d.getHours() === 0 && d.getMinutes() === 0) return base
  return `${base} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

const DEFAULT_Y_AXIS: YAxisConfig = { id: 0, position: 'left' }

function buildYAxes(yAxes?: YAxisConfig[]): YAxisConfig[] {
  if (yAxes && yAxes.length > 0) return yAxes
  return [DEFAULT_Y_AXIS]
}

function echartYAxis(cfg: YAxisConfig, hasRightAxis: boolean, narrow: boolean): any {
  const axis: any = {
    type: 'value' as const,
    splitLine: {
      lineStyle: { type: 'dashed' as const, opacity: 0.3 },
      show: cfg.position === 'left',
    },
  }
  if (cfg.label) {
    axis.name = cfg.label
    if (narrow) {
      axis.nameLocation = 'end'
      axis.nameTextStyle = { fontSize: 10 }
    } else {
      axis.nameLocation = 'middle'
      axis.nameRotate = cfg.position === 'right' ? 90 : -90
      axis.nameGap = 40
    }
  }
  axis.position = cfg.position
  if (cfg.min !== undefined) axis.min = cfg.min
  if (cfg.max !== undefined) axis.max = cfg.max
  if (hasRightAxis && cfg.position === 'left') {
    axis.splitLine = { show: false }
  }
  return axis
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
  palette?: string[]
}

export const TrackerChart = (params: TrackerChartProps): React.JSX.Element => {
  const datasets = params.data?.datasets ?? []
  const cc = params.chartConfig
  const dataZoomAdded = useRef(false)
  const narrow = useIsNarrowViewport()
  const colors = useMemo(() => params.palette ?? resolvePalette(cc?.palette), [cc?.palette])

  const option = useMemo(() => {
    const showLegend = cc?.show_legend !== false && datasets.length > 1
    const showToolbox = !!cc?.show_toolbox
    const stackHeader = narrow && showLegend && showToolbox
    const showSlider = cc?.show_slider !== false
    const yAxes = buildYAxes(cc?.y_axes)
    const hasRightAxis = yAxes.some((a) => a.position === 'right')
    const isDateOnly = cc?.x_axis_type === 'date'

    const grid: any = { left: 40, right: 10, top: showLegend ? (stackHeader ? 70 : 40) : 20, bottom: showSlider ? 80 : 30 }
    if (hasRightAxis) grid.right = 50
    if (!narrow && yAxes.some((a) => a.position === 'left' && a.label)) grid.left = Math.max(grid.left, 50)
    if (narrow && yAxes.some((a) => a.label)) grid.top = Math.max(grid.top, 44)

    const xAxis: any = {
      type: 'time' as const,
      splitLine: { show: false },
      axisLabel: {
        hideOverlap: true,
        ...(narrow ? { fontSize: 10 } : {}),
        formatter: isDateOnly ? formatDateTick : formatDateTimeTick,
      },
    }

    const opt: any = {
      color: colors,
      grid,
      xAxis,
      yAxis: yAxes.map((a) => echartYAxis(a, hasRightAxis, narrow)),
      series: datasets.map((ds, i) => {
        const seriesType = ds.seriesConfig?.type ?? 'line'
        const yAxisIndex = ds.seriesConfig?.y_axis_index ?? 0
        const entry: any = {
          name: ds.label,
          type: seriesType,
          yAxisIndex,
          data: ds.data.map((p) => ({ value: [isDateOnly ? p.x.substring(0, 10) : p.x, Number(p.y)], ...p.extra })),
        }
        if (seriesType === 'line' && cc?.area !== false) {
          entry.areaStyle = areaGradient(colors[i % colors.length])
        }
        if (seriesType === 'line' && cc?.show_symbols === false) {
          entry.symbol = 'none'
        }
        if (seriesType === 'bar') {
          entry.barMaxWidth = '80%'
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
    if (params.animation === false) {
      opt.animation = false
    }
    if (!dataZoomAdded.current) {
      opt.dataZoom = [{ type: 'inside' as const, xAxisIndex: 0, filterMode: 'none' as const }]
      if (cc?.show_slider !== false) {
        opt.dataZoom.push({ type: 'slider' as const, xAxisIndex: 0, bottom: 10, filterMode: 'none' as const })
      }
    }
    if (showLegend) {
      opt.legend = { type: 'scroll' as const, top: stackHeader ? 30 : 0 }
    }
    if (showToolbox) {
      opt.toolbox = {
        right: 8,
        top: 0,
        padding: [2, 8],
        feature: {
          saveAsImage: { title: 'Save' },
          restore: { title: 'Reset' },
        },
      }
    }
    if (cc?.x_axis_label) opt.xAxis.name = cc.x_axis_label
    opt.xAxis.min = params.min
    opt.xAxis.max = params.max
    return opt
  }, [datasets, cc, params.min, params.max, params.animation, colors, narrow])

  useEffect(() => {
    dataZoomAdded.current = true
  }, [])

  return (
    <ReactECharts
      option={option}
      style={{ width: '100%', height: 300 }}
      onEvents={params.onChartClick ? { click: params.onChartClick } : undefined}
      opts={{ renderer: 'svg' }}
      theme={CHART_THEME_NAME}
    />
  )
}
