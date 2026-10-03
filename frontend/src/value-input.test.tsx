import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import type { SeriesModel } from './core'
import { ValueInput, shiftDate } from './value-input'

const series: SeriesModel[] = [
  { id: 1, tracker_id: 1, name: 's1', data_type: 'float', config: '' },
  { id: 2, tracker_id: 1, name: 's2', data_type: 'float', config: '' },
]

const renderInput = (props: Partial<React.ComponentProps<typeof ValueInput>> = {}) => {
  const onAdd = props.onAdd ?? vi.fn().mockResolvedValue(undefined)
  render(
    <ValueInput seriesList={series} isDate defaultDate="2024-02-15" {...props} onAdd={onAdd} />
  )
  return { onAdd, group: screen.getByRole('group', { name: 'Add value' }) }
}

describe('shiftDate', () => {
  it('shifts a date string by whole days', () => {
    expect(shiftDate('2024-02-28', 1, true)).toBe('2024-02-29')
    expect(shiftDate('2024-03-01', -1, true)).toBe('2024-02-29')
    expect(shiftDate('2024-01-01', -1, true)).toBe('2023-12-31')
  })

  it('keeps the time part when shifting a datetime string', () => {
    expect(shiftDate('2024-02-15T10:30', 1, false)).toBe('2024-02-16T10:30')
    expect(shiftDate('2024-02-15T10:30', -1, false)).toBe('2024-02-14T10:30')
  })
})

describe('ValueInput', () => {
  it('defaults the date to the given value and selects the first series', () => {
    renderInput()
    expect(screen.getByLabelText('Value date')).toHaveValue('2024-02-15')
    expect(screen.getByRole('button', { name: 'Series' })).toHaveTextContent('s1')
  })

  it('renders a datetime input when the x axis is not date based', () => {
    renderInput({ isDate: false, defaultDate: '2024-02-15T10:30' })
    const input = screen.getByLabelText('Value date')
    expect(input).toHaveAttribute('type', 'datetime-local')
    expect(input).toHaveValue('2024-02-15T10:30')
  })

  it('shifts the date by one day with the previous/next buttons', async () => {
    const user = userEvent.setup()
    renderInput()

    await user.click(screen.getByRole('button', { name: 'Next day' }))
    expect(screen.getByLabelText('Value date')).toHaveValue('2024-02-16')

    await user.click(screen.getByRole('button', { name: 'Previous day' }))
    await user.click(screen.getByRole('button', { name: 'Previous day' }))
    expect(screen.getByLabelText('Value date')).toHaveValue('2024-02-14')
  })

  it('lists every series in the dropdown and switches the selection', async () => {
    const user = userEvent.setup()
    renderInput()

    await user.click(screen.getByRole('button', { name: 'Series' }))
    const items = await screen.findAllByRole('menuitemradio')
    expect(items.map((el) => el.textContent)).toEqual(['s1', 's2'])

    await user.click(items[1])
    expect(screen.getByRole('button', { name: 'Series' })).toHaveTextContent('s2')
  })

  it('adds the value for the selected series at the selected date', async () => {
    const user = userEvent.setup()
    const { onAdd } = renderInput()

    await user.click(screen.getByRole('button', { name: 'Series' }))
    await user.click((await screen.findAllByRole('menuitemradio'))[1])
    await user.type(screen.getByLabelText('Value'), '42')
    await user.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => {
      expect(onAdd).toHaveBeenCalledWith({ seriesId: 2, time: '2024-02-15T00:00:00.000Z', value: 42 })
    })
  })

  it('keeps the date and series, then clears and refocuses the value input', async () => {
    const user = userEvent.setup()
    const { onAdd } = renderInput()

    await user.type(screen.getByLabelText('Value'), '7')
    await user.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => {
      expect(onAdd).toHaveBeenCalledTimes(1)
    })
    await waitFor(() => {
      expect(screen.getByLabelText('Value')).toHaveValue(null)
    })
    expect(screen.getByLabelText('Value')).toHaveFocus()
    expect(screen.getByLabelText('Value date')).toHaveValue('2024-02-15')
    expect(screen.getByRole('button', { name: 'Series' })).toHaveTextContent('s1')
  })

  it('disables Add while the value is empty or a submission is in flight', async () => {
    const user = userEvent.setup()
    let resolveAdd: () => void = () => {}
    const onAdd = vi.fn().mockImplementation(() => new Promise<void>((resolve) => { resolveAdd = resolve }))
    renderInput({ onAdd })

    const add = screen.getByRole('button', { name: 'Add' })
    expect(add).toBeDisabled()

    await user.type(screen.getByLabelText('Value'), '1')
    expect(add).toBeEnabled()

    await user.click(add)
    expect(await screen.findByRole('button', { name: 'Adding...' })).toBeDisabled()

    resolveAdd()
    expect(await screen.findByRole('button', { name: 'Add' })).toBeInTheDocument()
    expect(onAdd).toHaveBeenCalledTimes(1)
  })

  it('shows an error message when adding fails', async () => {
    const user = userEvent.setup()
    const onAdd = vi.fn().mockRejectedValue(new Error('boom'))
    renderInput({ onAdd })

    await user.type(screen.getByLabelText('Value'), '3')
    await user.click(screen.getByRole('button', { name: 'Add' }))

    expect(await screen.findByText('Failed to add value. Please try again.')).toBeInTheDocument()
    expect(screen.getByLabelText('Value')).toHaveValue(3)
  })
})