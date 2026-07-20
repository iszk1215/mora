# Tracker API Specification

## Overview

Repository-independent time-series data tracking. Provides CRUD for trackers, series, and values with visibility-based access control.

- **Package**: `tracker`
- **Go files**: `tracker/store.go`, `tracker/handler.go`, `tracker/service.go`, `tracker/provider.go`
- **Frontend**: `frontend/src/tracker.tsx`, `frontend/src/tracker.test.tsx`

## Data Model

```
tracker
  ├── tracker_series (tracker_id FK)
  │    └── tracker_value (series_id FK)
  ├── tracker_member (user_id, tracker_id) — access control
  ├── tracker_like   (user_id, tracker_id)
  └── tracker_coverage (tracker_id PK → repository)
```

- **type=`tracker`**: Normal time-series data (tracker -> series -> values)
- **type=`coverage`**: Links to a repository via `tracker_coverage`. No series/values of its own.

## API Endpoints

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/api/trackers` | List trackers (paginated, `?q=&page=&per_page=`) | optional |
| POST | `/api/trackers` | Create tracker | required |
| DELETE | `/api/trackers/{trackerId}` | Delete tracker | owner |
| PATCH | `/api/trackers/{trackerId}` | Update visibility/chart_config | owner |
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
{ "name": "string", "visibility": "public|private", "type": "tracker|coverage", "repo_id": 1, "chart_config": "{}" }
```

### PatchTrackerRequest

```json
{ "visibility": "public|private", "chart_config": "{\"x_axis_label\":\"Date\"}" }
```

### TrackerResponse

```json
{ "id": 1, "name": "string", "visibility": "public", "type": "tracker", "chart_config": "{}", "role": "owner", "liked": false }
```

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

When `type="coverage"`, the tracker links to a repository via `tracker_coverage`. The preview endpoint fetches data from `CoverageTimelineProvider.Timeline(repoID, 20)` and maps coverage entries as series.

- Series/values endpoints return empty or 400 for coverage trackers
- Frontend routes to `/coverages/:trackerId` for coverage detail view

## Frontend Routes

| Path | Component | Description |
|------|-----------|-------------|
| `/trackers` | TrackerView | Card grid with preview charts |
| `/trackers/new` | TrackerCreate | Create form |
| `/trackers/:trackerId` | TrackerDetailView | Detail (tracker type) |
| `/trackers/:trackerId/edit` | TrackerDetailEdit | Edit visibility/chart |

## Key Files

| File | Purpose |
|------|---------|
| `tracker/store.go` | SQLite store, SQL queries |
| `tracker/handler.go` | HTTP handlers, middleware, models |
| `tracker/service.go` | Service wrapper, convenience methods |
| `tracker/provider.go` | `CoverageTimelineProvider` interface |
| `tracker/store_test.go` | Store unit tests |
| `tracker/handler_test.go` | Handler unit tests |
| `tracker/service_test.go` | Service unit tests |
| `frontend/src/tracker.tsx` | Frontend components |
| `frontend/src/tracker.test.tsx` | Frontend tests |
