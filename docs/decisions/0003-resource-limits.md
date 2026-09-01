# ADR 0003: Cap series and value resources on trackers

## Status

Accepted

## Context

Mora trackers could grow without bound: any number of series could be added to a
tracker and any number of values to a series. This risks unbounded database
growth and chart rendering load. The tracker count already has a limit for free
users (`core.FreeUserMaxTrackers`, issue #179), but it is user-type based and
only applies to trackers, not to the series/values they contain.

Issue #195 asks for hard limits on series and values that apply to all users
regardless of paid/free tier.

## Decision

Establish universal hard limits, enforced at insert time in the tracker store:

| Resource | Limit |
|----------|-------|
| Series per tracker | `10` |
| Values per series | `5000` |

- Defined as exported constants `tracker.MaxSeriesPerTracker` and
  `tracker.MaxValuesPerSeries` so they can be adjusted in one place.
- Checked inside `store.addSeries` and `store.addValue` before the INSERT, using
  the same `count >= max` convention as the existing tracker limit.
- A limit-reached creation returns HTTP 403 with a descriptive message.
- Limits apply to all user types (free, pro, admin).

## Rationale / sizing

- **Series (10/tracker)**: dashboards typically show a handful of series; 10
  comfortably covers multiple metrics, comparisons, and tests without meaningful
  DB cost (series rows are lightweight).
- **Values (5000/series)**: ~60 bytes per `tracker_value` row, so a full series
  is ~300 KB and a full tracker (10 x 5000) is ~50,000 rows (~3 MB). Charts query
  with a `?limit` parameter so long series do not hurt rendering. At one value
  per day this covers ~13.7 years; per hour ~7 months.

## Consequences

### Positive

- Bounded DB growth per tracker, predictable resource usage.
- Simple, central enforcement in the store; handlers only translate the error.
- Limits are easily tunable through the exported constants.

### Negative

- Users with long-running, high-frequency trackers may hit the value cap and
  must delete old data points to continue.
- Existing data or the one-time UDM->tracker migration (raw SQL) may exceed the
  caps; they are not retroactively trimmed or prevented.

### Alternatives considered

- No limits at all: rejected, unbounded growth.
- Tier-based series/value limits (free vs pro): deferred; the issue explicitly
  requests a single hard limit regardless of tier. The constant-based design
  leaves room to introduce tiers later.