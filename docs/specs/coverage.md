# Coverage URL Specification

## Overview

Coverage data is accessed via tracker-based URLs. Each coverage tracker links to a repository through the `tracker_coverage` table, which is owned and managed by the coverage package.

## URL Structure

### Frontend

| Pattern | Description |
|---------|-------------|
| `/coverages/:trackerId` | Coverage list view |
| `/coverages/:trackerId/:index` | Coverage at index |
| `/coverages/:trackerId/:index/:entry` | Entry detail |
| `/coverages/:trackerId/:index/:entry/*` | File view |

### API

| Pattern | Description |
|---------|-------------|
| `POST /api/coverages` | Create a coverage tracker for a repository |
| `/api/coverages/:trackerId` | Coverage list |
| `/api/coverages/:trackerId/preview` | Preview (timeline as virtual series) |
| `/api/coverages/:trackerId/:index` | Coverage at index |
| `/api/coverages/:trackerId/:index/:entry/files` | File list |
| `/api/coverages/:trackerId/:index/:entry/files/*` | File view |

## Middleware

`injectTrackerCoverage` in `server/server.go`:

1. Parse `trackerId` from URL
2. Resolve `trackerId` -> `repo_id` via `coverage.FindRepoIDByTrackerID()`
3. Load repository and repository manager
4. Verify access (session or API key)
5. Inject `core.Repository` and `core.RepositoryClient` into context

`GET /api/coverages/{trackerId}/preview` uses only `requireTrackerAuth` + `InjectTracker` + `RequireReadPermission` (no SCM access needed).

## Coverage Type Trackers

Coverage trackers are created through the coverage API; the tracker package has no coverage knowledge.

### `POST /api/coverages`

Creates a coverage-type tracker and links it to a repository. Requires authentication.

Request:

```json
{ "name": "string", "visibility": "public|private", "repo_id": 1 }
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Tracker name (user-supplied) |
| `visibility` | "public" \| "private" | yes | Access control |
| `repo_id` | number | yes | Repository ID to link |

Responses:

| Status | Condition |
|--------|-----------|
| 201 | Tracker created and linked (`tracker.TrackerModel`) |
| 400 | Invalid body, missing/invalid fields |
| 403 | Not authenticated |
| 404 | Repository not found |
| 409 | Repository already has a coverage tracker |

- On startup, `coverage.MigrateCoverageTrackers` creates a coverage tracker for every repository that has none, creating trackers via the tracker service and linking them through `coverage.Link`
- Preview: served by `CoverageHandler.HandleCoveragePreview` at `/api/coverages/{trackerId}/preview`, fetching from `CoverageStore.Timeline(repoID, 20)`
- Series/values endpoints return 400 (no direct data management)
- Detail view: `/coverages/:trackerId` shows coverage charts and file browser

## Upload

Upload command specifies tracker ID via `--tracker` flag and POSTs to `/api/coverages/:trackerId`.

## Key Files

| File | Purpose |
|------|---------|
| `coverage/coverage_store.go` | SQLite store for coverage/coverage_entry/coverage_block |
| `coverage/coverage_handler.go` | HTTP handlers |
| `coverage/upload.go` | Coverage upload logic |
| `coverage/command.go` | CLI commands |
| `server/server.go` | Route mounting, injectTrackerCoverage middleware |
| `frontend/src/coverage.tsx` | Coverage frontend pages |
