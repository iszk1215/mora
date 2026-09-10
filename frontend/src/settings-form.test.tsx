import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { SettingsForm } from './settings-form'

describe('SettingsForm', () => {
  describe('onChange', () => {
    it('calls onChange with show_slider true when slider checkbox is checked', async () => {
      const onChange = vi.fn()
      render(
        <SettingsForm
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
        <SettingsForm
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
        <SettingsForm
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
        <SettingsForm
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
        <SettingsForm
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
    it('emits x_axis_type date by default', async () => {
      const onChange = vi.fn()
      render(
        <SettingsForm
          initialConfig={{}}
          onChange={onChange}
        />,
      )
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0]
      expect(lastCall.x_axis_type).toBe('date')
    })

    it('emits x_axis_type datetime when datetime is selected', async () => {
      const onChange = vi.fn()
      render(
        <SettingsForm
          initialConfig={{}}
          onChange={onChange}
        />,
      )
      const select = screen.getByDisplayValue('Date')
      await act(() => {
        fireEvent.change(select, { target: { value: 'datetime' } })
      })
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0]
      expect(lastCall.x_axis_type).toBe('datetime')
    })

    it('emits explicit x_axis_type date when date is selected', async () => {
      const onChange = vi.fn()
      render(
        <SettingsForm
          initialConfig={{ x_axis_type: 'datetime' }}
          onChange={onChange}
        />,
      )
      const select = screen.getByDisplayValue('Datetime')
      await act(() => {
        fireEvent.change(select, { target: { value: 'date' } })
      })
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0]
      expect(lastCall.x_axis_type).toBe('date')
    })

    it('emits show_symbols false when symbols checkbox is unchecked', async () => {
      const onChange = vi.fn()
      render(
        <SettingsForm
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
        <SettingsForm
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
        <SettingsForm
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
        expect.any(String),
      )
    })
  })

  describe('visibility', () => {
    it('renders visibility select with initial value', () => {
      render(
        <SettingsForm
          initialConfig={{}}
          initialVisibility="public"
        />,
      )
      expect(screen.getByText('Visibility')).toBeInTheDocument()
      expect(screen.getByDisplayValue('Public')).toBeInTheDocument()
    })

    it('passes visibility to onSave when changed', async () => {
      const onSave = vi.fn()
      render(
        <SettingsForm
          initialConfig={{}}
          initialVisibility="private"
          onSave={onSave}
        />,
      )
      const select = screen.getByDisplayValue('Private')
      await act(() => {
        fireEvent.change(select, { target: { value: 'public' } })
      })
      const saveButton = screen.getByRole('button', { name: /save/i })
      await act(() => {
        fireEvent.click(saveButton)
      })
      expect(onSave).toHaveBeenCalledWith(
        expect.any(Object),
        'public',
      )
    })
  })
})
