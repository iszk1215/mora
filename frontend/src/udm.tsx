import React, {
  useEffect,
  useState,
} from 'react'

import DatePicker from "react-datepicker";

import ReactECharts from 'echarts-for-react'

import {
  Link,
  LoaderFunctionArgs,
  Params,
  redirect,
  useLoaderData,
} from 'react-router'

import { Repo } from './core'

export interface UdmMetric {
  id: number,
  repod_id: number,
  name: string,
}

interface UdmItem {
  id: number,
  metric_id: number,
  name: string,
  type: number,
}

interface UdmValue {
  id: number,
  item_id: number,
  revision: string,
  time: string,
  value: string,
}

interface MetricsResponse {
  repo: Repo,
  metrics: UdmMetric[],
}

interface ItemsResponse {
  repo: Repo,
  metric: UdmMetric,
  items: UdmItem[],
}

interface ValuesResponse {
  repo: Repo,
  item: UdmItem,
  values: UdmValue[],
}

async function _loadUdmMetrics(repo_id: number): Promise<MetricsResponse> {
  const url = `/api/repos/${repo_id}/udm/metrics`
  const resp = await fetch(url)
  if (!resp.ok)
    throw resp

  return resp.json()
}

export async function loadUdmMetrics(repo_id: number): Promise<UdmMetric[]> {
  const resp = await _loadUdmMetrics(repo_id)
  return resp.metrics
}

async function loadUdmMetricsFromParam({ params }: LoaderFunctionArgs): Promise<MetricsResponse> {
  if (params.repo_id)
    return _loadUdmMetrics(parseInt(params.repo_id))

  throw new Error("repo_id is undefined")
}

export async function loadMetricItems({ params }: LoaderFunctionArgs): Promise<Response> {
  const url = `/api/repos/${params.repo_id}/udm/metrics/${params.metric_id}/items`
  const resp = await fetch(url)
  if (resp.status == 403) {
    return redirect("/scms")
  }

  if (!resp.ok)
    throw resp

  return resp
}


export async function loadMetricValues(
  repo_id: number, metric_id: number, item_id: number): Promise<ValuesResponse> {
  const url = `/api/repos/${repo_id}/udm/metrics/${metric_id}/items/${item_id}/values`
  const resp = await fetch(url)
  if (!resp.ok)
    throw resp

  return await resp.json()
}

export const UdmRoot = (): React.JSX.Element => {
  const data = useLoaderData() as MetricsResponse
  const repo = data.repo
  const metrics = data.metrics
  const elems: React.JSX.Element[] = []
  metrics.forEach((metric: UdmMetric, i: number) => {
    elems.push(
      <div className="ui item" key={i}>
        <Link to={`/repos/${repo.id}/udm/metrics/${metric.id}`}>{metric.name}</Link>
      </div>)
  })

  return (
    <div>
      <h2>User Defined Metrics</h2>
      <div className="ui list">{elems}</div>
    </div>
  )
}

export const UdmChart = (params: any): React.JSX.Element => {
  const datasets = params.data?.datasets ?? []

  const option: any = {
    grid: { left: 60, right: 20, top: 20, bottom: 40 },
    xAxis: {
      type: 'time' as const,
    },
    yAxis: {
      type: 'value' as const,
      name: params.ylabel,
    },
    series: datasets.map((ds: any) => ({
      name: ds.label,
      type: 'line' as const,
      data: ds.data.map((p: any) => [p.x, p.y]),
    })),
    tooltip: {
      trigger: 'axis' as const,
    },
  }

  if (params.min) {
    option.xAxis.min = params.min
  }
  if (params.max) {
    option.xAxis.max = params.max
  }

  return (
    <ReactECharts option={option} style={{ width: '100%', height: 300 }} id="udm-chart" />)
}

export const UdmMetricRoot = (): React.JSX.Element => {
  const data = useLoaderData() as ItemsResponse

  const [valuesList, setValuesList] = useState<UdmValue[][]>([])

  const repo = data.repo
  const metric = data.metric
  const items = data.items

  useEffect(() => {
    Promise.all(
      items.map((item: UdmItem) => loadMetricValues(repo.id, metric.id, item.id))
    ).then((responses) => responses.map((r: ValuesResponse) => r.values))
      .then(setValuesList)
  }, [items, repo.id, metric.id])

  const valuesToDataset = (values: UdmValue[], metric: UdmItem) => {
    return {
      data: values.map((value: UdmValue) => ({ x: value.time, y: value.value })),
      label: metric.name,
    }
  }

  const datasets: any = valuesList.map(
    (values, i) => valuesToDataset(values, items[i]))

  const [startDate, setStartDate] = useState<Date | null>(null);
  const [endDate, setEndDate] = useState<Date | null>(null)

  const onStartDateChange = (date: Date | null) => { setStartDate(date) }
  const onEndDateChange = (date: Date | null) => { setEndDate(date) }

  const chart = (
    <UdmChart
      data={{ datasets }}
      ylabel={metric.name}
      min={startDate}
      max={endDate}
    />
  )

  const elem = items.map((item: UdmItem, i: number) => (<li key={i}>{item.name}</li>))

  return (
    <div>
      <div className="pt-2 flex items-center">
        <span className="mr-1">From</span>
        <div className="w-1/4">
          <DatePicker
            selected={startDate}
            onChange={onStartDateChange}
            className="border rounded px-2 py-1 w-full"
            placeholderText="Select date"
            dateFormat="yyyy-MM-dd"
          />
        </div>
        <span className="px-2">To</span>
        <div className="w-1/4">
          <DatePicker
            selected={endDate}
            onChange={onEndDateChange}
            className="border rounded px-2 py-1 w-full"
            placeholderText="Select date"
            dateFormat="yyyy-MM-dd"
          />
        </div>
      </div>
      {chart}
    </div>
  )
}

export const udmRoute = [
  {
    index: true,
    element: <UdmRoot />,
    loader: loadUdmMetricsFromParam,
  },
  {
    path: 'metrics/:metric_id',
    handle: {
      crumb: (_params: Params, data: any) => ({ label: data.metric.name })
    },
    loader: loadMetricItems,
    element: <UdmMetricRoot />,
  },
]
