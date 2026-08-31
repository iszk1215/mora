import React, { useEffect, useRef, useState } from 'react'

import { Button } from '@/components/ui/button'
import { ChartConfig, YAxisConfig } from './core'
import { PALETTE_NAMES } from './chart'

export interface ChartOptionsFormProps {
  initialConfig: ChartConfig
  baselineConfig?: ChartConfig
  onChange?: (config: ChartConfig) => void
  onSave?: (config: ChartConfig) => Promise<void> | void
  onCancel?: (config: ChartConfig) => void
  onYAxesChange?: (config: ChartConfig) => void
  onRemoveYAxis?: (removed: YAxisConfig) => void
}

export const ChartOptionsForm = ({ initialConfig, baselineConfig, onChange, onSave, onCancel, onYAxesChange, onRemoveYAxis }: ChartOptionsFormProps): React.JSX.Element => {
  const [xLabel, setXLabel] = useState(initialConfig.x_axis_label ?? '')
  const [xAxisType, setXAxisType] = useState<'date' | 'datetime'>(initialConfig.x_axis_type ?? 'date')
  const [area, setArea] = useState(initialConfig.area ?? true)
  const [showLegend, setShowLegend] = useState(initialConfig.show_legend ?? false)
  const [showSymbols, setShowSymbols] = useState(initialConfig.show_symbols ?? true)
  const [showSlider, setShowSlider] = useState(initialConfig.show_slider ?? false)
  const [showToolbox, setShowToolbox] = useState(initialConfig.show_toolbox ?? true)
  const [palette, setPalette] = useState(initialConfig.palette ?? 'default')
  const [yAxes, setYAxes] = useState<YAxisConfig[]>(() => initialConfig.y_axes?.length ? initialConfig.y_axes : [{ id: 0, position: 'left' }])

  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const baselineRef = useRef<ChartConfig>(baselineConfig ?? initialConfig)

  const resetFrom = (cc: ChartConfig) => {
    setXLabel(cc.x_axis_label ?? '')
    setXAxisType(cc.x_axis_type ?? 'date')
    setArea(cc.area ?? true)
    setShowLegend(cc.show_legend ?? false)
    setShowSymbols(cc.show_symbols ?? true)
    setShowSlider(cc.show_slider ?? false)
    setShowToolbox(cc.show_toolbox ?? true)
    setPalette(cc.palette ?? 'default')
    setYAxes(cc.y_axes?.length ? cc.y_axes : [{ id: 0, position: 'left' }])
    setSaved(false)
  }

  const buildConfig = (yAxesOverride?: YAxisConfig[]): ChartConfig => {
    const cc: ChartConfig = {}
    if (xLabel.trim()) cc.x_axis_label = xLabel.trim()
    if (xAxisType === 'date') cc.x_axis_type = 'date'
    if (!area) cc.area = false
    if (showLegend) cc.show_legend = true
    if (!showLegend) cc.show_legend = false
    if (!showSymbols) cc.show_symbols = false
    if (!showSlider) cc.show_slider = false
    if (showToolbox) cc.show_toolbox = true
    cc.palette = palette
    const axes = yAxesOverride ?? yAxes
    cc.y_axes = axes.map((a) => {
      const axis: YAxisConfig = { id: a.id, position: a.position }
      if (a.label?.trim()) axis.label = a.label.trim()
      if (a.min !== undefined) axis.min = a.min
      if (a.max !== undefined) axis.max = a.max
      return axis
    })
    return cc
  }

  useEffect(() => {
    onChange?.(buildConfig())
    setSaved(false)
  }, [xLabel, xAxisType, area, showLegend, showSymbols, showSlider, showToolbox, palette, yAxes])

  const handleSave = async () => {
    setSaving(true)
    try {
      const config = buildConfig()
      await onSave?.(config)
      baselineRef.current = config
      setSaved(true)
    } finally {
      setSaving(false)
    }
  }

  const handleCancel = () => {
    const baseline = baselineRef.current
    resetFrom(baseline)
    onChange?.(baseline)
    onCancel?.(baseline)
  }

  return (
    <div>
      <div className="flex flex-wrap items-center gap-3 mb-4">
        <label className="flex items-center gap-1 text-sm">
          <input type="checkbox" checked={area} onChange={(e) => setArea(e.target.checked)} />
          Area
        </label>
        <label className="flex items-center gap-1 text-sm">
          <input type="checkbox" checked={showLegend} onChange={(e) => setShowLegend(e.target.checked)} />
          Legend
        </label>
        <label className="flex items-center gap-1 text-sm">
          <input type="checkbox" checked={showSymbols} onChange={(e) => setShowSymbols(e.target.checked)} />
          Symbols
        </label>
        <label className="flex items-center gap-1 text-sm">
          <input type="checkbox" checked={showSlider} onChange={(e) => setShowSlider(e.target.checked)} />
          Slider
        </label>
        <label className="flex items-center gap-1 text-sm">
          <input type="checkbox" checked={showToolbox} onChange={(e) => setShowToolbox(e.target.checked)} />
          Toolbox
        </label>
      </div>

      <div className="flex flex-wrap items-center gap-3 mb-4">
        <span className="text-lg">Color</span>
        <select
          value={palette}
          onChange={(e) => setPalette(e.target.value)}
          className="border rounded px-2 py-1"
        >
          {PALETTE_NAMES.map((name) => (
            <option key={name} value={name}>{name}</option>
          ))}
        </select>
      </div>

      <div className="flex flex-wrap items-center gap-3 mb-4">
        <span className="text-lg">X-Axis</span>
        <select
          value={xAxisType}
          onChange={(e) => setXAxisType(e.target.value as 'date' | 'datetime')}
          className="border rounded px-2 py-1"
        >
          <option value="date">Date</option>
          <option value="datetime">Datetime</option>
        </select>
        <input
          type="text"
          value={xLabel}
          onChange={(e) => setXLabel(e.target.value)}
          placeholder="X-axis label"
          className="border rounded px-2 py-1 w-40"
        />
      </div>

      <h3 className="text-lg my-2">Y-Axes</h3>
      <div className="flex flex-col gap-2 mb-4">
        {(['left', 'right'] as const).map((pos) => {
          const axis = yAxes.find((a) => a.position === pos)
          const active = !!axis
          const canRemove = yAxes.length > 1

          const updateAxis = (patch: Partial<YAxisConfig>) => {
            setYAxes((prev) => prev.map((a) => a.position === pos ? { ...a, ...patch } : a))
          }

          const commitYAxes = (next: YAxisConfig[]) => {
            const config = buildConfig(next)
            setYAxes(next)
            setSaved(false)
            onYAxesChange?.(config)
          }

          return (
            <div key={pos} className="flex items-center gap-2">
              <span className="text-sm font-medium w-12">{pos === 'left' ? 'Left' : 'Right'}</span>
              {active ? (
                <>
                  <input
                    type="text"
                    value={axis.label ?? ''}
                    onChange={(e) => updateAxis({ label: e.target.value || undefined })}
                    placeholder="Label"
                    className="border rounded px-2 py-1 w-32 text-sm"
                  />
                  <input
                    type="number"
                    value={axis.min ?? ''}
                    onChange={(e) => updateAxis({ min: e.target.value ? parseFloat(e.target.value) : undefined })}
                    placeholder="Min"
                    className="border rounded px-2 py-1 w-20 text-sm"
                  />
                  <input
                    type="number"
                    value={axis.max ?? ''}
                    onChange={(e) => updateAxis({ max: e.target.value ? parseFloat(e.target.value) : undefined })}
                    placeholder="Max"
                    className="border rounded px-2 py-1 w-20 text-sm"
                  />
                  <Button
                    variant="destructive"
                    size="sm"
                    disabled={!canRemove}
                    onClick={() => {
                      const next = yAxes.filter((a) => a.position !== pos)
                      commitYAxes(next.length ? next : [{ id: 0, position: 'left' }])
                      onRemoveYAxis?.(axis)
                    }}
                  >
                    Remove
                  </Button>
                </>
              ) : (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    const id = yAxes.length > 0 ? Math.max(...yAxes.map((a) => a.id)) + 1 : 0
                    commitYAxes([...yAxes, { id, position: pos }])
                  }}
                >
                  Add
                </Button>
              )}
            </div>
          )
        })}
      </div>

      <div className="flex items-center gap-2">
        <Button onClick={handleSave} disabled={saving}>
          {saving ? 'Saving...' : saved ? 'Saved' : 'Save'}
        </Button>
        {onCancel && (
          <Button variant="outline" onClick={handleCancel} disabled={saving}>
            Cancel
          </Button>
        )}
      </div>
    </div>
  )
}
