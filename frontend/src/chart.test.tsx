import React from 'react'
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { formatValue, TrackerChart } from './chart'

vi.mock('echarts-for-react', () => ({
  default: ({ option, onEvents }: any) => (
    <div
      data-testid="echart"
      data-option={JSON.stringify(option)}
      onClick={() => onEvents?.click?.({ seriesName: 'go', data: { value: ['2024-01-15', 90], index: 1 } })}
    />
  ),
}))

describe('formatValue', () => {
  it('returns default format when fmt is undefined', () => {
    expect(formatValue(42, undefined)).toBe('42')
    expect(formatValue(42.5, undefined)).toBe('42.5')
    expect(formatValue(42.567, undefined)).toBe('42.6')
  })

  it('returns default format when fmt is empty string', () => {
    expect(formatValue(42, '')).toBe('42')
    expect(formatValue(42.5, '')).toBe('42.5')
  })

  it('formats with .Nf pattern', () => {
    expect(formatValue(42.567, '%.1f')).toBe('42.6')
    expect(formatValue(42.567, '%.2f')).toBe('42.57')
    expect(formatValue(42.567, '%.3f')).toBe('42.567')
    expect(formatValue(42.567, '%.0f')).toBe('43')
  })

  it('formats with %d pattern', () => {
    expect(formatValue(42.567, '%d')).toBe('43')
    expect(formatValue(42.4, '%d')).toBe('42')
  })

  it('includes literal % via %%', () => {
    expect(formatValue(42.567, '%.1f%%')).toBe('42.6%')
    expect(formatValue(42.567, 'Value: %.2f%%')).toBe('Value: 42.57%')
  })

  it('handles integer values', () => {
    expect(formatValue(0, '%.2f')).toBe('0.00')
    expect(formatValue(100, '%.1f')).toBe('100.0')
  })
})

describe('TrackerChart', () => {
  const datasets = [
    {
      label: 'go',
      data: [
        { x: '2024-01-15', y: '90.0', extra: { index: 1 } },
        { x: '2024-06-15', y: '85.0', extra: { index: 2 } },
      ],
      seriesConfig: { value_format: '%.1f%%' },
    },
  ]

  it('renders echart element', () => {
    render(<TrackerChart data={{ datasets }} />)
    expect(screen.getByTestId('echart')).toBeInTheDocument()
  })

  it('passes datasets to echart option', () => {
    render(<TrackerChart data={{ datasets }} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series).toHaveLength(1)
    expect(option.series[0].name).toBe('go')
    expect(option.series[0].data[0].value).toEqual(['2024-01-15', 90])
    expect(option.series[0].data[0].index).toBe(1)
  })

  it('sets animation false when animation prop is false', () => {
    render(<TrackerChart data={{ datasets }} animation={false} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.animation).toBe(false)
  })

  it('does not set animation when animation prop is omitted', () => {
    render(<TrackerChart data={{ datasets }} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.animation).toBeUndefined()
  })

  it('includes dataZoom options by default', () => {
    render(<TrackerChart data={{ datasets }} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.dataZoom).toHaveLength(2)
    expect(option.dataZoom[0].type).toBe('inside')
    expect(option.dataZoom[1].type).toBe('slider')
  })

  it('reserves larger bottom margin when slider is shown', () => {
    render(<TrackerChart data={{ datasets }} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.grid.bottom).toBe(80)
    expect(option.dataZoom[1].bottom).toBe(10)
  })

  it('excludes slider when show_slider is false', () => {
    render(<TrackerChart data={{ datasets }} chartConfig={{ show_slider: false }} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.dataZoom).toHaveLength(1)
    expect(option.dataZoom[0].type).toBe('inside')
  })

  it('reduces bottom margin when slider is hidden', () => {
    render(<TrackerChart data={{ datasets }} chartConfig={{ show_slider: false }} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.grid.bottom).toBe(30)
  })

  it('applies chartConfig axis labels', () => {
    const chartConfig = {
      x_axis_label: 'Date',
      y_axes: [{ id: 0, label: 'Coverage %', max: 100, position: 'left' as const }],
    }
    render(<TrackerChart data={{ datasets }} chartConfig={chartConfig} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.xAxis.name).toBe('Date')
    expect(option.yAxis).toHaveLength(1)
    expect(option.yAxis[0].name).toBe('Coverage %')
    expect(option.yAxis[0].max).toBe(100)
  })

  it('applies min and max to xAxis', () => {
    const min = new Date('2024-01-01')
    const max = new Date('2024-12-31')
    render(<TrackerChart data={{ datasets }} min={min} max={max} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.xAxis.min).toBe(min.toISOString())
    expect(option.xAxis.max).toBe(max.toISOString())
  })

  it('calls onChartClick when click event fires', () => {
    const onChartClick = vi.fn()
    render(<TrackerChart data={{ datasets }} onChartClick={onChartClick} />)
    fireEvent.click(screen.getByTestId('echart'))
    expect(onChartClick).toHaveBeenCalledWith(
      expect.objectContaining({ seriesName: 'go', data: expect.objectContaining({ index: 1 }) })
    )
  })

  it('shows legend when multiple datasets', () => {
    const multiDatasets = [
      { label: 'go', data: [{ x: '2024-01-15', y: '90' }], seriesConfig: undefined },
      { label: 'py', data: [{ x: '2024-01-15', y: '80' }], seriesConfig: undefined },
    ]
    render(<TrackerChart data={{ datasets: multiDatasets }} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.legend).toEqual({ type: 'scroll', top: 0 })
  })

  it('hides legend when single dataset', () => {
    render(<TrackerChart data={{ datasets }} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.legend).toBeUndefined()
  })

  it('sets xAxis.min/max to null when min and max are null', () => {
    render(<TrackerChart data={{ datasets }} min={null} max={null} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.xAxis.min).toBeNull()
    expect(option.xAxis.max).toBeNull()
  })

  it('sets xAxis.min to null when switching from a date range to null', () => {
    const { rerender } = render(
      <TrackerChart data={{ datasets }} min={new Date('2024-01-01')} max={null} />
    )
    const el = screen.getByTestId('echart')
    const option1 = JSON.parse(el.getAttribute('data-option')!)
    expect(option1.xAxis.min).toBeTruthy()

    rerender(<TrackerChart data={{ datasets }} min={null} max={null} />)
    const option2 = JSON.parse(el.getAttribute('data-option')!)
    expect(option2.xAxis.min).toBeNull()
  })

  it('renders bar series when seriesConfig.type is bar', () => {
    const barDatasets = [
      { label: 'count', data: [{ x: '2024-01-15', y: '10' }], seriesConfig: { type: 'bar' as const } },
    ]
    render(<TrackerChart data={{ datasets: barDatasets }} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series[0].type).toBe('bar')
    expect(option.series[0].barMaxWidth).toBe('80%')
    expect(option.series[0].areaStyle).toBeUndefined()
  })

  it('renders mixed line and bar series', () => {
    const mixedDatasets = [
      { label: 'count', data: [{ x: '2024-01-15', y: '10' }], seriesConfig: { type: 'bar' as const } },
      { label: 'rate', data: [{ x: '2024-01-15', y: '80' }], seriesConfig: { type: 'line' as const } },
    ]
    render(<TrackerChart data={{ datasets: mixedDatasets }} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series[0].type).toBe('bar')
    expect(option.series[1].type).toBe('line')
    expect(option.series[1].areaStyle).toBeDefined()
  })

  it('omits areaStyle for line series when chartConfig.area is false', () => {
    const chartConfig = { area: false }
    render(<TrackerChart data={{ datasets }} chartConfig={chartConfig} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series[0].areaStyle).toBeUndefined()
  })

  it('omits symbol for line series when chartConfig.show_symbols is false', () => {
    const chartConfig = { show_symbols: false }
    render(<TrackerChart data={{ datasets }} chartConfig={chartConfig} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series[0].symbol).toBe('none')
  })

  it('renders multiple Y-axes from chartConfig', () => {
    const chartConfig = {
      y_axes: [
        { id: 0, label: 'Count', position: 'left' as const },
        { id: 1, label: 'Rate (%)', position: 'right' as const, min: 0, max: 100 },
      ],
    }
    render(<TrackerChart data={{ datasets }} chartConfig={chartConfig} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.yAxis).toHaveLength(2)
    expect(option.yAxis[0].name).toBe('Count')
    expect(option.yAxis[0].position).toBe('left')
    expect(option.yAxis[1].name).toBe('Rate (%)')
    expect(option.yAxis[1].position).toBe('right')
    expect(option.yAxis[1].max).toBe(100)
  })

  it('assigns series to correct Y-axis via y_axis_index', () => {
    const chartConfig = {
      y_axes: [
        { id: 0, label: 'Count', position: 'left' as const },
        { id: 1, label: 'Rate', position: 'right' as const },
      ],
    }
    const multiDatasets = [
      { label: 'count', data: [{ x: '2024-01-15', y: '10' }], seriesConfig: { type: 'bar' as const, y_axis_index: 0 } },
      { label: 'rate', data: [{ x: '2024-01-15', y: '80' }], seriesConfig: { type: 'line' as const, y_axis_index: 1 } },
    ]
    render(<TrackerChart data={{ datasets: multiDatasets }} chartConfig={chartConfig} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series[0].yAxisIndex).toBe(0)
    expect(option.series[1].yAxisIndex).toBe(1)
  })

  it('sets explicit position on both Y-axes', () => {
    const chartConfig = {
      y_axes: [
        { id: 0, label: 'Left Axis', position: 'left' as const },
        { id: 1, label: 'Right Axis', position: 'right' as const },
      ],
    }
    render(<TrackerChart data={{ datasets }} chartConfig={chartConfig} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.yAxis[0].position).toBe('left')
    expect(option.yAxis[1].position).toBe('right')
  })

  it('defaults to single Y-axis when y_axes is not set', () => {
    render(<TrackerChart data={{ datasets }} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.yAxis).toHaveLength(1)
    expect(option.yAxis[0].type).toBe('value')
  })

  it('strips time portion from x values when x_axis_type is date', () => {
    const dateDatasets = [
      { label: 'go', data: [{ x: '2024-01-15T10:30:00Z', y: '90.0' }] },
    ]
    const chartConfig = { x_axis_type: 'date' as const }
    render(<TrackerChart data={{ datasets: dateDatasets }} chartConfig={chartConfig} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series[0].data[0].value[0]).toBe('2024-01-15')
  })

  it('preserves time portion in x values when x_axis_type is datetime', () => {
    const dateDatasets = [
      { label: 'go', data: [{ x: '2024-01-15T10:30:00Z', y: '90.0' }] },
    ]
    const chartConfig = { x_axis_type: 'datetime' as const }
    render(<TrackerChart data={{ datasets: dateDatasets }} chartConfig={chartConfig} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.series[0].data[0].value[0]).toBe('2024-01-15T10:30:00Z')
  })
})
