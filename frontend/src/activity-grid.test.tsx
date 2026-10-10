import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'

import { ActivityGridChart, ActivityGridLegend, addDaysUTC, gridTooltipFormatter } from './activity-grid'
import { ACTIVITY_GRID_EMPTY_COLOR, DEFAULT_GRID_LEVELS } from './chart'
import { Dataset } from './chart'

vi.mock('echarts-for-react/esm/core', () => ({
  default: ({ option }: any) => <div data-testid="echart" data-option={JSON.stringify(option)} />,
}))

const datasets: Dataset[] = [
  {
    label: 's1',
    data: [
      { x: '2024-02-01T00:00:00.000Z', y: '1' },
      { x: '2024-02-02T00:00:00.000Z', y: '4' },
    ],
  },
]

const getOption = (): any => {
  const el = screen.getByTestId('echart')
  return JSON.parse(el.getAttribute('data-option') ?? '{}')
}

describe('addDaysUTC', () => {
  it('adds whole days across month and year boundaries', () => {
    expect(addDaysUTC('2024-02-28', 1)).toBe('2024-02-29')
    expect(addDaysUTC('2024-03-01', -1)).toBe('2024-02-29')
    expect(addDaysUTC('2024-01-01', -1)).toBe('2023-12-31')
  })
})

describe('ActivityGridChart', () => {
  it('renders the empty state when there are no datasets', () => {
    render(<ActivityGridChart data={{ datasets: [] }} />)
    expect(screen.getByText('No data to display')).toBeInTheDocument()
  })

  it('renders a real date when the x value is not a full DateTime', () => {
    render(<ActivityGridChart data={{ datasets: [
      { label: 's1', data: [{ x: '2024-02-01', y: '1' }] },
    ] }} />)
    const option = getOption()
    expect(option.xAxis.data).toEqual(['2024-02-01'])
  })

  it('fills every day cell from the first to last value date', () => {
    render(<ActivityGridChart data={{ datasets }} />)
    const option = getOption()
    expect(option.xAxis.data).toEqual(['2024-02-01', '2024-02-02'])
    expect(option.series[0].data).toEqual([
      [0, 0, 1],
      [1, 0, 4],
    ])
  })

  it('clamps values to the configured number of levels', () => {
    render(<ActivityGridChart data={{ datasets }} chartConfig={{ grid_levels: 3 }} />)
    const option = getOption()
    const colors = option.visualMap.pieces.slice(1).map((p: any) => p.color)
    expect(colors).toHaveLength(3)
    const gridData: Array<Array<number>> = option.series[0].data
    expect(gridData.find((cell) => cell[0] === 0)![2]).toBe(1)
  })

  it('maps -1 to the empty color via piecewise', () => {
    render(<ActivityGridChart data={{ datasets }} />)
    const option = getOption()
    const pieces = option.visualMap.pieces as Array<any>
    expect(pieces[0]).toEqual({ lt: 0, color: ACTIVITY_GRID_EMPTY_COLOR })
    const innerColors = pieces.slice(1)
    const expectedColors = [
      { gte: 0, lt: 1, color: expect.any(String) },
      { gte: 1, lt: 2, color: expect.any(String) },
      { gte: 2, lt: 3, color: expect.any(String) },
      { gte: 3, lt: 4, color: expect.any(String) },
      { gte: 4, color: expect.any(String) },
    ]
    expect(innerColors).toEqual(expect.arrayContaining(expectedColors))
    expect(innerColors).toHaveLength(DEFAULT_GRID_LEVELS)
  })

  it('does not expose raw numbers in the tooltip', () => {
    render(<ActivityGridChart data={{ datasets }} />)
    expect(getOption().tooltip.trigger).toBe('item')

    const html = gridTooltipFormatter({ marker: '<span>x</span>', seriesName: 's1', name: '2024-02-02' })
    expect(html).not.toContain('4')
    expect(html).toContain('2/2')
  })

  it('caps the date range at 1000 days', () => {
const sparse = [
    { label: 's1', data: [
      { x: '2024-01-01T00:00:00.000Z', y: '1' },
      { x: '2030-01-10T00:00:00.000Z', y: '2' },
    ] },
  ]
    render(<ActivityGridChart data={{ datasets: sparse }} />)
    const option = getOption()
    expect(option.xAxis.data).toHaveLength(1000)
    expect(option.xAxis.data[0]).toBe('2027-04-17')
    expect(option.xAxis.data[999]).toBe('2030-01-10')
  })

  it('renders a legend with the empty swatch plus palette colors', () => {
    render(<ActivityGridLegend colors={['#a', '#b']} />)
    const swatches = document.querySelectorAll('[aria-hidden="true"] span')
    expect(swatches).toHaveLength(3)
    expect(swatches[0]).toHaveStyle({ backgroundColor: ACTIVITY_GRID_EMPTY_COLOR })
  })
})