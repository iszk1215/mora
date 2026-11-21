# Activity Grid Spec

Activity grid ("contribution graph") tracker type for tracking coarse-grained daily activities with a color intensity level instead of precise numbers.

## Data model

- A tracker with `type = "activity_grid"`.
- Series are created through `POST /api/trackers/{trackerId}/series` like normal trackers, but the **data type is forced to `int`** server-side surface area: the frontend hides the Data Type / Chart Type selects in the Add Series dialog and sends `data_type: "int"` with no series `config`.
- Series order is controlled by `PUT /api/trackers/{trackerId}/series/order`; each series is one rendered row, so dragging a row by its leftmost handle changes the row order in both the settings table and the chart. The handle supports touch input and keyboard arrow keys.
- Values are **non-negative integers** (`0 <= value`, integer only). The level count is defined by the chart config, not by the stored value, so any non-negative integer is accepted (clamping is a display concern).
- `chart_config` validation for activity grid accepts only:
  - `palette` (string) — named palette, same values as `tracker` type
  - `grid_levels` (integer, 1-10, default 5) — number of color levels

## Chart

- Rendering: Cartesian heatmap, **one series = one row**, day is the column. Each cell is a fixed-size square.
- Day bucketing: UTC `YYYY-MM-DD` from the value timestamp (`time.slice(0,10)`). `x_axis_type` is ignored for grid trackers.
- Value -> level: clamped to `[0, grid_levels)`, i.e. `min(grid_levels-1, round(value))`.
- Colors: first `grid_levels` colors of the chosen palette; cells with no value use the fixed empty color `#ebedf0`.
- The visible day span is the data date range, extended to the selected time range (`all` = data extent), capped at 1000 days.

## Value entry UX

- All value entry UIs show **color swatches only** (one per level) — no numeric input, and the tooltip never exposes the raw number:
  - Quick add row (`ValueInput`)
  - Data Points card (per-series row)
  - Data point edit card (`DataPointEditCard`)
- Clicking the level `i` swatch submits value `i` (0-based) for the selected series/date.

## API validation

| Condition | Response |
|-----------|----------|
| `chart_config` contains keys other than `palette`/`grid_levels` for an activity grid tracker | 400 |
| `grid_levels` not an integer or outside 1-10 | 400 |
| Value added/updated is not a non-negative integer | 400 `activity_grid values must be non-negative integers` |

## Frontend

- `frontend/src/activity-grid.tsx`: `ActivityGridChart`, `ActivityGridLegend`, `addDaysUTC`, `gridTooltipFormatter`
- `frontend/src/chart.tsx`: `resolveGridColors`, `isActivityGrid`, `DEFAULT_GRID_LEVELS`, `MIN_GRID_LEVELS`, `MAX_GRID_LEVELS`, `ACTIVITY_GRID_EMPTY_COLOR` (ECharts heatmap + piecewise visualMap are registered on the shared `echarts` core)
- `frontend/src/tracker.tsx`: detail view renders `ActivityGridChart`, settings/Add Series/value UIs branch on `isGrid`
- `frontend/src/settings-form.tsx`: color palette + Levels (1-10) + swatch preview; line chart options, X-Axis, and Y-Axes are hidden for grid trackers
- `frontend/src/tracker-card.tsx`: "Activity Grid" badge and compact heatmap preview
- `frontend/src/tracker-api.ts`: `createTracker(name, visibility, description?, type?)`