import { DateTime } from 'luxon'
import React from 'react'
import { Chart, registerables } from 'chart.js'
import { color } from 'chart.js/helpers'
import 'chartjs-adapter-luxon'
import 'hw-chartjs-plugin-colorschemes'
import { Line } from 'react-chartjs-2'
import { getChartData, makeDataset } from './chart'

import { Coverage } from './core'
(Chart as any).helpers = { color }
Chart.register(...registerables)

export const CoverageChart = (params: any): JSX.Element => {
  const coverages = params.coverages as Coverage[]

  const tmp = getChartData()
  const options = tmp.options

  if (params.min) {
    options.scales.x.min = DateTime.fromJSDate(params.min).toISO()
  }

  if (params.max) {
    options.scales.x.max = DateTime.fromJSDate(params.max).toISO()
  }

  let datasets: any = []
  if (coverages.length > 0) {
    datasets = makeDataset(coverages)
  }

  return (
    <Line data={{ datasets }} options={options} width={400} height={100} id="chart-cov" />)
}
