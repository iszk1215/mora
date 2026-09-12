# Tracker API Specification

## Overview

Repository-independent time-series data tracking. Provides CRUD for trackers, series, and values with visibility-based access control.

- **Package**: `tracker`
- **Go files**: `tracker/store.go`, `tracker/handler.go`, `tracker/service.go`
- **Frontend**: `frontend/src/tracker.tsx`, `frontend/src/tracker.test.tsx`

## Data Model

```
tracker
  ├── tracker_series (tracker_id FK)
  │    └── tracker_value (series_id FK)
  ├── tracker_member (user_id, tracker_id) — editor access
  └── tracker_like   (user_id, tracker_id)
```

- `tracker.owner_id`: FK to `user.id`, `NOT NULL`, `ON DELETE CASCADE`. The owner is the sole authority for DELETE and PATCH on the tracker; `tracker_member` rows grant editor (edit-only) access.
- `tracker.created_at` / `tracker.last_updated_at`: RFC3339 timestamps. `last_updated_at` is set at creation and bumped whenever a value is added, updated, or deleted (`POST`/`PATCH`/`DELETE values`) or coverage data is uploaded for a linked repository.

- **type=`tracker`**: Normal time-series data (tracker -> series -> values)
- **type=`coverage`**: Links to a repository via the `tracker_coverage` table, owned and managed by the coverage package. No series/values of its own. Created via the coverage API, not `POST /api/trackers`.

## Limits

Hard limits on the number of series and values, applied regardless of user type (free/pro/admin, issue #195). Exceeded creation requests return 403.

| Resource | Limit | Enforced by | 403 message |
|----------|-------|-------------|-------------|
| Series per tracker | `10` (`tracker.MaxSeriesPerTracker`) | `store.addSeries` | `series limit reached (max 10 series per tracker)` |
| Values per series | `5000` (`tracker.MaxValuesPerSeries`) | `store.addValue` | `value limit reached (max 5000 values per series)` |

- Limits are checked before insert; the 10th series and the 5000th value are allowed, and subsequent inserts are rejected.
- Deleting a series (or all values of a series) frees capacity.
- Coverage-type trackers have no series/values and are unaffected.
- The limits only guard new inserts through the tracker store. Existing data and the one-time UDM->tracker data migration (which writes via raw SQL) are not retroactively capped.
- The tracker-count limit (`core.FreeUserMaxTrackers`, free users only) is a separate, user-type-based limit.

## API Endpoints

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/api/trackers` | List trackers (paginated, `?q=&page=&per_page=`) | optional |
| POST | `/api/trackers` | Create tracker | required |
| DELETE | `/api/trackers/{trackerId}` | Delete tracker | owner |
| PATCH | `/api/trackers/{trackerId}` | Update visibility/chart_config/description | owner |
| GET | `/api/trackers/{trackerId}/preview` | Preview data (latest 20 values per series) | read perm |
| GET | `/api/trackers/{trackerId}/series` | List series | read perm |
| POST | `/api/trackers/{trackerId}/series` | Create series | edit perm |
| PATCH | `/api/trackers/{trackerId}/series/{seriesId}` | Update series (rename via `name`, data_type, config) | edit perm |
| DELETE | `/api/trackers/{trackerId}/series/{seriesId}` | Delete series | edit perm |
| GET | `/api/trackers/{trackerId}/series/{seriesId}/values` | List values (`?limit=N`) | read perm |
| POST | `/api/trackers/{trackerId}/series/{seriesId}/values` | Add value | edit perm |
| PATCH | `/api/trackers/{trackerId}/series/{seriesId}/values` | Batch update/delete values | edit perm |
| DELETE | `/api/trackers/{trackerId}/series/{seriesId}/values` | Delete all values | edit perm |
| POST | `/api/trackers/{trackerId}/like` | Like | authenticated |
| DELETE | `/api/trackers/{trackerId}/like` | Unlike | authenticated |

POST returns 201, DELETE returns 204.

PATCH `/api/trackers/{trackerId}/series/{seriesId}` accepts optional `name`, `data_type`, and `config` fields; only provided fields are updated. A series `name` must be non-empty (after trim), at most 200 characters, and unique within the tracker. A rename to an already-used name returns `409 Conflict`.

PATCH `/api/trackers/{trackerId}/series/{seriesId}/values` performs an atomic batch update: all value updates are applied and all requested deletions are executed in a single transaction (`applyValueChanges` in `tracker/store.go`), so the batch either succeeds completely or not at all. The whole batch is subject to the values-per-series limit (5000).

### PatchValuesRequest

```json
{
  "updates": [
    { "id": 3, "time": "2024-01-02T00:00:00Z", "value": 45.0 },
    { "id": 5, "time": "2024-01-04T00:00:00Z", "value": 12 }
  ],
  "deletes": [7, 9]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `updates` | Array of `{id, time, value}` | Values to update. Each `id` must reference an existing value of the series, otherwise the batch fails with `404` |
| `deletes` | Array of number | Value IDs to delete. Unknown IDs are silently ignored |

Error behavior: on a `409` conflict the batch fails with a message like `value already exists at time 2024-01-02T00:00:00Z` naming the first conflicting time, and no changes are applied (transaction rolled back). Coverage-type trackers have no value endpoints and return `400`; values with a time already present within the updates themselves (two updates for the same new time) also conflict.

On success the response body is the updated values list:

```json
{
  "values": [
    { "id": 3, "time": "2024-01-02T00:00:00Z", "value": 45.0 },
    { "id": 5, "time": "2024-01-04T00:00:00Z", "value": 12 }
  ]
}
```

### ValueModel

Values carry an `id` in list (`GET values`, `PATCH values`) and preview responses:

```json
{ "id": 3, "time": "2024-01-02T00:00:00Z", "value": 45.0 }
```

The `id` is needed by the batch PATCH endpoint and by the frontend edit card to track per-value edits and deletions.

## Authentication & Authorization

Three-layer middleware in `tracker/handler.go`:

```
requireAuth -> requireReadPermission -> requireEditPermission -> requireOwnerPermission
```

### requireReadPermission

| visibility | anonymous | logged-in (non-member) | member | superuser (admin type) |
|------------|-----------|----------------------|--------|-------------------|
| public | yes | yes | yes | yes |
| private | no | no | yes | yes |

### requireEditPermission

- Anonymous users cannot edit
- Superuser (admin type): full access
- Members (owner/editor): can edit

### requireOwnerPermission

Applied to DELETE and PATCH on `/api/trackers/{trackerId}`.

- Anonymous users and non-members: 404
- Superuser (admin type): full access
- Owner (`tracker.owner_id == uid`): allowed
- Editors and other members: 404

Series and value endpoints remain edit-permission only (owner or editor).

### Authentication sources (server.go)

1. Session cookie (`MoraSession.IsLoggedIn()`)
2. API Key (`Authorization: Bearer <token>`)
3. Neither -> anonymous (pass-through, no 401)

## Visibility

Only two values are allowed: `"public"` and `"private"`.

- `public`: readable by anyone
- `private`: readable only by members and superuser

## Request/Response Types

### CreateTrackerRequest

```json
{ "name": "string", "description": "string", "body": "string", "visibility": "public|private", "chart_config": "{\"y_axes\":[{\"id\":0,\"position\":\"left\"}]}" }
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Tracker name |
| `description` | string | no | One-line description (max 200 characters) |
| `body` | string | no | Free-form Markdown body shown below the chart (max 100000 characters) |
| `visibility` | "public" \| "private" | yes | Access control |
| `chart_config` | string | no | JSON string of ChartConfig |

The tracker type is always `"tracker"`. Coverage-type trackers cannot be created through this endpoint (returns 400); use `POST /api/coverages` instead.

### PatchTrackerRequest

```json
{ "visibility": "public|private", "chart_config": "{\"x_axis_label\":\"Date\",\"y_axes\":[{\"id\":0,\"label\":\"Count\",\"position\":\"left\"}]}", "description": "Updated description", "body": "## Notes\n\nUpdated body" }
```

All fields are optional. Only provided fields are updated.

| Field | Type | Description |
|-------|------|-------------|
| `visibility` | "public" \| "private" | Access control |
| `chart_config` | string | JSON string of ChartConfig |
| `description` | string | One-line description (max 200 characters) |
| `body` | string | Free-form Markdown body (max 100000 characters) |

### TrackerResponse

```json
{ "id": 1, "name": "string", "description": "string", "body": "string", "visibility": "public", "type": "tracker", "chart_config": "{}", "owner_id": 1, "owner_name": "string", "created_at": "2024-01-01T00:00:00Z", "last_updated_at": "2024-01-01T00:00:00Z", "role": "owner", "liked": false }
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | number | Tracker ID |
| `name` | string | Tracker name |
| `description` | string | One-line description |
| `body` | string | Free-form Markdown body shown below the chart |
| `visibility` | "public" \| "private" | Access control |
| `type` | "tracker" \| "coverage" | Tracker type |
| `chart_config` | string | JSON string of ChartConfig |
| `owner_id` | number | Owner user ID |
| `owner_name` | string | Owner username |
| `created_at` | string | Creation timestamp (RFC3339) |
| `last_updated_at` | string | Last value-added timestamp (RFC3339) |
| `role` | string | User's role: "" (none), "owner", "editor" |
| `liked` | boolean | Whether the current user liked this tracker |
| `like_count` | number | Total like count |

### ChartConfig (JSON stored in `chart_config`)

```json
{
  "x_axis_label": "Date",
  "area": true,
  "show_legend": true,
  "palette": "default",
  "y_axes": [
    { "id": 0, "label": "Count", "position": "left", "min": 0 },
    { "id": 1, "label": "Rate (%)", "position": "right", "min": 0, "max": 100 }
  ]
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `x_axis_label` | string | — | X-axis label |
| `area` | boolean | true | Show area fill under line series |
| `show_legend` | boolean | true | Show legend (also for a single series; set to false to hide) |
| `palette` | string | "random" | Named color palette |
| `y_axes` | YAxisConfig[] | `[{id:0,position:"left"}]` | Y-axis definitions |

### YAxisConfig

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | number | yes | 0-based axis index |
| `label` | string | no | Axis label |
| `position` | "left" \| "right" | yes | Axis side |
| `min` | number | no | Minimum value |
| `max` | number | no | Maximum value |

### SeriesConfig (JSON stored in `series.config`)

```json
{
  "value_format": "%.1f%%",
  "type": "line",
  "y_axis_index": 0
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `value_format` | string | — | Printf-style format for tooltip (e.g. `%.1f%%`) |
| `type` | "line" \| "bar" | "line" | Chart series type |
| `y_axis_index` | number | 0 | Which Y-axis this series maps to |

### PreviewResponse

```json
{
  "tracker": { "id": 1, "name": "string", "type": "tracker", "chart_config": "{}", "owner_id": 1, "owner_name": "string", "created_at": "2024-01-01T00:00:00Z", "last_updated_at": "2024-01-01T00:00:00Z", "role": "owner", "liked": false },
  "series": [
    { "series": { "id": 1, "name": "string", "data_type": "float", "config": "{}" },
      "values": [{ "time": "2024-01-01T00:00:00Z", "value": 45.0 }] }
  ]
}
```

## Coverage Type

Coverage-type trackers (`type="coverage"`) are created exclusively through the coverage API (`POST /api/coverages`, see [coverage.md](coverage.md)). The tracker package has no coverage knowledge; it only stores the row.

- A coverage tracker links to a repository via the `tracker_coverage` table, owned and managed by the coverage package.
- Preview data is served by the coverage handler at `GET /api/coverages/{trackerId}/preview`, which fetches from `CoverageStore.Timeline(repoID, 20)` and maps coverage entries as series.
- Series/values endpoints return empty or 400 for non-`tracker` types
- Frontend routes to `/coverages/:trackerId` for coverage detail view

## Frontend Routes

| Path | Component | Description |
|------|-----------|-------------|
| `/trackers` | TrackerView | Card grid with preview charts |
| `/trackers/new` | TrackerCreate | Create form |
| `/trackers/:trackerId` | TrackerDetailView | Detail (tracker type). Owner menu offers a "Data Points" card (Add Data Points) with a date picker (defaults to today), a value input, and an Add button per series; values are added live to the chart without a page reload. Clicking a series name in that card opens an inline data-point edit card (owner only): it lists that series' values newest-first (paginable, 10/20/50/100 per page, default 20) with editable date/time and value inputs and a delete button per row; edits and deletes reflect instantly in the chart. "Save" commits all pending edits/deletes to the backend in a single `PATCH values` batch. Owner menu also opens a "Settings" card with chart options and visibility |

## Key Files

| File | Purpose |
|------|---------|
| `tracker/store.go` | SQLite store, SQL queries |
| `tracker/handler.go` | HTTP handlers, middleware, models |
| `tracker/service.go` | Service wrapper, convenience methods |
| `tracker/store_test.go` | Store unit tests |
| `tracker/handler_test.go` | Handler unit tests |
| `tracker/service_test.go` | Service unit tests |
| `frontend/src/tracker.tsx` | Frontend components |
| `frontend/src/chart.tsx` | Chart rendering (TrackerChart, ECharts) |
| `frontend/src/tracker.test.tsx` | Frontend tests |
| `frontend/src/chart.test.tsx` | Chart tests |
