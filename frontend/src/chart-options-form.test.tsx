import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { ChartOptionsForm } from './chart-options-form'

describe('ChartOptionsForm', () => {
  describe('onChange', () => {
    it('calls onChange with show_slider true when slider checkbox is checked', async () => {
      const onChange = vi.fn()
      render(
        <ChartOptionsForm
          initialConfig={{ show_slider: false }}
          onChange={onChange}
        />,
      )
      const slider = screen.getByLabelText('Slider')
      await act(() => {
        fireEvent.click(slider)
      })
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0]
      expect(lastCall.show_slider).toBe(true)
    })

    it('calls onChange with show_slider false when slider checkbox is unchecked', async () => {
      const onChange = vi.fn()
      render(
        <ChartOptionsForm
          initialConfig={{ show_slider: true }}
          onChange={onChange}
        />,
      )
      const slider = screen.getByLabelText('Slider')
      await act(() => {
        fireEvent.click(slider)
      })
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0]
      expect(lastCall.show_slider).toBe(false)
    })

    it('calls onChange with show_legend true when legend checkbox is checked', async () => {
      const onChange = vi.fn()
      render(
        <ChartOptionsForm
          initialConfig={{ show_legend: false }}
          onChange={onChange}
        />,
      )
      const legend = screen.getByLabelText('Legend')
      await act(() => {
        fireEvent.click(legend)
      })
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0]
      expect(lastCall.show_legend).toBe(true)
    })

    it('calls onChange with area false when area checkbox is unchecked', async () => {
      const onChange = vi.fn()
      render(
        <ChartOptionsForm
          initialConfig={{ area: true }}
          onChange={onChange}
        />,
      )
      const area = screen.getByLabelText('Area')
      await act(() => {
        fireEvent.click(area)
      })
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0]
      expect(lastCall.area).toBe(false)
    })

    it('calls onChange with show_toolbox true when toolbox checkbox is checked', async () => {
      const onChange = vi.fn()
      render(
        <ChartOptionsForm
          initialConfig={{ show_toolbox: false }}
          onChange={onChange}
        />,
      )
      const toolbox = screen.getByLabelText('Toolbox')
      await act(() => {
        fireEvent.click(toolbox)
      })
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0]
      expect(lastCall.show_toolbox).toBe(true)
    })
  })

  describe('buildConfig', () => {
    it('emits show_symbols false when symbols checkbox is unchecked', async () => {
      const onChange = vi.fn()
      render(
        <ChartOptionsForm
          initialConfig={{ show_symbols: true }}
          onChange={onChange}
        />,
      )
      const symbols = screen.getByLabelText('Symbols')
      await act(() => {
        fireEvent.click(symbols)
      })
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0]
      expect(lastCall.show_symbols).toBe(false)
    })

    it('includes all checkbox options in built config', async () => {
      const onChange = vi.fn()
      render(
        <ChartOptionsForm
          initialConfig={{
            area: false,
            show_legend: true,
            show_symbols: false,
            show_slider: true,
            show_toolbox: false,
          }}
          onChange={onChange}
        />,
      )
      const lastCall = onChange.mock.calls[0][0]
      expect(lastCall).toEqual(
        expect.objectContaining({
          area: false,
          show_legend: true,
          show_symbols: false,
          show_slider: true,
        }),
      )
    })
  })

  describe('onSave', () => {
    it('calls onSave with current config when save button is clicked', async () => {
      const onSave = vi.fn()
      render(
        <ChartOptionsForm
          initialConfig={{ show_slider: false }}
          onSave={onSave}
        />,
      )
      const slider = screen.getByLabelText('Slider')
      await act(() => {
        fireEvent.click(slider)
      })
      const saveButton = screen.getByRole('button', { name: /save/i })
      await act(() => {
        fireEvent.click(saveButton)
      })
      expect(onSave).toHaveBeenCalledWith(
        expect.objectContaining({ show_slider: true }),
      )
    })
  })
})
