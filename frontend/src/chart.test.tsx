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

  it('includes dataZoom options', () => {
    render(<TrackerChart data={{ datasets }} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.dataZoom).toHaveLength(2)
    expect(option.dataZoom[0].type).toBe('inside')
    expect(option.dataZoom[1].type).toBe('slider')
  })

  it('applies chartConfig axis labels', () => {
    const chartConfig = {
      x_axis_label: 'Date',
      y_axis_label: 'Coverage %',
      y_max: 100,
    }
    render(<TrackerChart data={{ datasets }} chartConfig={chartConfig} />)
    const el = screen.getByTestId('echart')
    const option = JSON.parse(el.getAttribute('data-option')!)
    expect(option.xAxis.name).toBe('Date')
    expect(option.yAxis.name).toBe('Coverage %')
    expect(option.yAxis.max).toBe(100)
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
})
