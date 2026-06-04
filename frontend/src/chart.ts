import { Coverage } from './core'

interface Point {
  x: string
  y: number
  index: number
}

export function getChartData () {
  const chartData = {
    type: 'line',
    data: {
      datasets: [],
      labels: null
    },
    options: {
      onClick: function (_ev: any, elements: any, chart: any) {
        // console.log(chart.data)
        if (elements.length === 1) {
          const e = elements[0]
          const dataset = chart.data.datasets[e.datasetIndex]
          if (dataset.label !== 'total') {
            const d = dataset.data[e.index] as Point
            const url = `${window.location}/${d.index}/${dataset.entry}`;
            // window.location = url;
            (window as Window).location = url
          }
        }
      },
      scales: {
        x: {
          type: 'time' as const,
          position: 'bottom' as const,
          title: {},
          min: undefined as string | undefined,
          max: undefined as string | undefined,
        },
        y: {
          type: 'linear' as const,
          position: 'left' as const,
          title: {
            display: true,
            text: 'Coverage %'
          }
        }
      },
      animation: {
        duration: 0
      },
      plugins: {
        colorschemes: {
          scheme: 'tableau.Classic10'
        },
        // datalabels: {
        //     align: "top",
        //     // backgroundColor: function(context) {
        //     //     return context.dataset.backgroundColor
        //     // },
        //     // borderRadius: 4,
        //     formatter: function(value, context) {
        //         return `#${value.index}`
        //     },
        // },
        tooltip: {
          callbacks: {
            label: function (context: any) {
              const data = context.dataset.data[context.dataIndex] as Point
              const label = context.dataset.label
              const y = context.raw.y
              return `#${data.index}: ${label} ${y.toFixed(1)}%`
            }
          }
        }
      }
    }
  }

  return chartData
}

export function makeDataset (coverages: Coverage[]) {
  const map: { [name: string]: Point[] } = {}

  const hasMultiEntries = coverages.reduce(
    (flag: boolean, cov: Coverage) => flag || cov.entries.length > 1,
    false
  )
  if (hasMultiEntries) {
    map.total = []
  }

  for (const cov of coverages) {
    for (const e of cov.entries) {
      if (!(e.name in map)) {
        map[e.name] = []
      }
      map[e.name].push(
        { x: cov.time, y: e.hits * 100.0 / e.lines, index: cov.index }
      )
    }
    if (hasMultiEntries) {
      map.total.push(
        { x: cov.time, y: cov.hits * 100.0 / cov.lines, index: cov.index }
      )
    }
  }
  const datasets = []
  for (const k in map) {
    const label = k === '_default' ? 'coverage' : k
    datasets.push({ borderWidth: 1, label, data: map[k], entry: k })
  }

  return datasets
}
