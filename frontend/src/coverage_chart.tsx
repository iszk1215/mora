import { DateTime } from 'luxon'
import React, { useCallback } from 'react'
import ReactECharts from 'echarts-for-react'
import { getCoverageOption, makeCoverageSeries } from './chart'
import { Coverage } from './core'

export const CoverageChart = (params: any): React.JSX.Element => {
  const coverages = params.coverages as Coverage[]

  const option: any = getCoverageOption()

  if (params.min) {
    option.xAxis.min = DateTime.fromJSDate(params.min).toISO()
  }
  if (params.max) {
    option.xAxis.max = DateTime.fromJSDate(params.max).toISO()
  }

  if (coverages.length > 0) {
    option.series = makeCoverageSeries(coverages)
  } else {
    option.series = []
  }

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
    />
  )
}
