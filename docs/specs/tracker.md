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
  ├── tracker_member (user_id, tracker_id) — access control
  └── tracker_like   (user_id, tracker_id)
```

- **type=`tracker`**: Normal time-series data (tracker -> series -> values)
- **type=`coverage`**: Links to a repository via the `tracker_coverage` table, owned and managed by the coverage package. No series/values of its own. Created via the coverage API, not `POST /api/trackers`.

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
| PATCH | `/api/trackers/{trackerId}/series/{seriesId}` | Update series | edit perm |
| DELETE | `/api/trackers/{trackerId}/series/{seriesId}` | Delete series | edit perm |
| GET | `/api/trackers/{trackerId}/series/{seriesId}/values` | List values (`?limit=N`) | read perm |
| POST | `/api/trackers/{trackerId}/series/{seriesId}/values` | Add value | edit perm |
| DELETE | `/api/trackers/{trackerId}/series/{seriesId}/values` | Delete all values | edit perm |
| POST | `/api/trackers/{trackerId}/like` | Like | authenticated |
| DELETE | `/api/trackers/{trackerId}/like` | Unlike | authenticated |

POST returns 201, DELETE returns 204.

## Authentication & Authorization

Three-layer middleware in `tracker/handler.go`:

```
requireAuth -> requireReadPermission -> requireEditPermission
```

### requireReadPermission

| visibility | anonymous | logged-in (non-member) | member | superuser (id=1) |
|------------|-----------|----------------------|--------|-------------------|
| public | yes | yes | yes | yes |
| private | no | no | yes | yes |

### requireEditPermission

- Anonymous users cannot edit
- Superuser (id=1): full access
- Members (owner/editor): can edit

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
{ "name": "string", "description": "string", "visibility": "public|private", "chart_config": "{\"y_axes\":[{\"id\":0,\"position\":\"left\"}]}" }
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Tracker name |
| `description` | string | no | One-line description (max 200 characters) |
| `visibility` | "public" \| "private" | yes | Access control |
| `chart_config` | string | no | JSON string of ChartConfig |

The tracker type is always `"tracker"`. Coverage-type trackers cannot be created through this endpoint (returns 400); use `POST /api/coverages` instead.

### PatchTrackerRequest

```json
{ "visibility": "public|private", "chart_config": "{\"x_axis_label\":\"Date\",\"y_axes\":[{\"id\":0,\"label\":\"Count\",\"position\":\"left\"}]}", "description": "Updated description" }
```

All fields are optional. Only provided fields are updated.

| Field | Type | Description |
|-------|------|-------------|
| `visibility` | "public" \| "private" | Access control |
| `chart_config` | string | JSON string of ChartConfig |
| `description` | string | One-line description (max 200 characters) |

### TrackerResponse

```json
{ "id": 1, "name": "string", "description": "string", "visibility": "public", "type": "tracker", "chart_config": "{}", "role": "owner", "liked": false }
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | number | Tracker ID |
| `name` | string | Tracker name |
| `description` | string | One-line description |
| `visibility` | "public" \| "private" | Access control |
| `type` | "tracker" \| "coverage" | Tracker type |
| `chart_config` | string | JSON string of ChartConfig |
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
| `show_legend` | boolean | true | Show legend (when >1 series) |
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
  "tracker": { "id": 1, "name": "string", "type": "tracker", "chart_config": "{}", "role": "owner", "liked": false },
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
| `/trackers/:trackerId` | TrackerDetailView | Detail (tracker type) |
| `/trackers/:trackerId/edit` | TrackerDetailEdit | Edit visibility/chart/description |

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
