import React, { useCallback, useEffect, useMemo, useRef } from 'react'
import ReactECharts from 'echarts-for-react'
import { getCoverageOption, makeCoverageSeries } from './chart'
import { Coverage } from './core'

export const CoverageChart = (params: any): React.JSX.Element => {
  const coverages = params.coverages as Coverage[]
  const min = params.min as Date | undefined
  const max = params.max as Date | undefined
  const dataZoomAdded = useRef(false)

  useEffect(() => {
    dataZoomAdded.current = true
  }, [])

  const option = useMemo(() => {
    const opt: any = getCoverageOption()
    if (coverages.length > 0) {
      opt.series = makeCoverageSeries(coverages)
      for (const s of opt.series) {
        s.areaStyle = { opacity: 0.12 }
      }
    } else {
      opt.series = []
    }
    if (min) opt.xAxis.min = min
    if (max) opt.xAxis.max = max
    if (!dataZoomAdded.current) {
      opt.dataZoom = [
        { type: 'inside' as const, xAxisIndex: 0, filterMode: 'none' as const },
        { type: 'slider' as const, xAxisIndex: 0, bottom: 10, filterMode: 'none' as const },
      ]
    }
    return opt
  }, [coverages, min, max])

  const onChartClick = useCallback((rawParams: any) => {
    if (rawParams.seriesName !== 'total') {
      const d = rawParams.data
      if (d?.index !== undefined) {
        const url = `${window.location}/${d.index}/${rawParams.seriesName}`
        window.location.assign(url)
      }
    }
  }, [])

  return (
    <ReactECharts
      option={option}
      style={{ width: '100%', height: 300 }}
      onEvents={{ click: onChartClick }}
      opts={{ renderer: 'svg' }}
    />
  )
}
