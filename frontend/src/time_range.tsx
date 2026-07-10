export type TimeRangeKey = 'all' | '1y' | '1m' | '1w' | '1d'

const ranges: { key: TimeRangeKey; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: '1y', label: '1Y' },
  { key: '1m', label: '1M' },
  { key: '1w', label: '1W' },
  { key: '1d', label: '1D' },
]

export function computeDateRange(range: TimeRangeKey): { min: Date | null; max: null } {
  const now = Date.now()
  switch (range) {
    case 'all': return { min: null, max: null }
    case '1y': return { min: new Date(now - 365 * 24 * 60 * 60 * 1000), max: null }
    case '1m': return { min: new Date(now - 30 * 24 * 60 * 60 * 1000), max: null }
    case '1w': return { min: new Date(now - 7 * 24 * 60 * 60 * 1000), max: null }
    case '1d': return { min: new Date(now - 24 * 60 * 60 * 1000), max: null }
  }
}

export const TimeRangeSelector = ({ value, onChange }: {
  value: TimeRangeKey
  onChange: (key: TimeRangeKey) => void
}): React.JSX.Element => (
  <div className="flex gap-1 mb-2 justify-end">
    {ranges.map((r) => (
      <button
        key={r.key}
        onClick={() => onChange(r.key)}
        className={`px-2 py-0.5 text-xs rounded border cursor-pointer ${
          value === r.key
            ? 'bg-muted text-foreground border-border'
            : 'text-muted-foreground border-transparent hover:bg-accent'
        }`}
      >
        {r.label}
      </button>
    ))}
  </div>
)
