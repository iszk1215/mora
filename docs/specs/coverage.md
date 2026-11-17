# Coverage URL Specification

## Overview

Coverage data is accessed via tracker-based URLs. Each coverage tracker links to a repository through the `tracker_coverage` table, which is owned and managed by the coverage package. `:trackerId` in every URL is the tracker external ID (`trk_<uuidv7>`), and `:coverageId` is the coverage external ID (`cov_<uuidv7>`). Neither is the internal integer primary key.

## URL Structure

### Frontend

| Pattern | Description |
|---------|-------------|
| `/coverages/:trackerId` | Coverage list view |
| `/coverages/:trackerId/:coverageId` | Coverage detail |
| `/coverages/:trackerId/:coverageId/:entry` | Entry detail |
| `/coverages/:trackerId/:coverageId/:entry/*` | File view |

The coverage list displays a per-tracker ordinal (`#1`, `#2`, ...) for readability; the ordinal is not part of the URL or API. Old numeric URLs are not accepted.

### API

| Pattern | Description |
|---------|-------------|
| `POST /api/coverages` | Create a coverage tracker for a repository |
| `/api/coverages/:trackerId` | Coverage list |
| `/api/coverages/:trackerId/preview` | Preview (timeline as virtual series) |
| `/api/coverages/:trackerId/:coverageId` | Coverage detail |
| `/api/coverages/:trackerId/:coverageId/:entry/files` | File list |
| `/api/coverages/:trackerId/:coverageId/:entry/files/*` | File view |

Coverage list responses use `coverages[].id` (`cov_<uuidv7>`). List, file-list, and code responses return only the repository `{url, namespace, name}`; internal coverage IDs, tracker IDs, and repository IDs are not exposed. Upload responses return the coverage external ID, revision, time, and entries.

## Middleware

`injectTrackerCoverage` in `server/server.go` (runs after `InjectTracker`, which resolves the external ID):

1. Parse `trackerId` from URL (external ID `trk_<uuidv7>`; invalid format -> 400)
2. Resolve the internal tracker ID -> repository via `coverage.FindRepoByTrackerID()`
3. Load repository and repository manager
4. Verify access (session or API key)
5. Inject `core.Repository` and `core.RepositoryClient` into context

Access is verified directly with `checkRepoAccessByRepositoryManager(session, manager, namespace, name)`. Coverage paths do not use the per-request `core.Repository.Id` access cache because that ID is not the tracker ID.

The `tracker_coverage` table is owned by the coverage package and stores the repository description (scm, namespace, name, url) directly, keyed by `tracker_id` with a foreign key to `tracker(id) ON DELETE CASCADE`. The `coverage` table is keyed by `tracker_id` (foreign key to `tracker(id) ON DELETE CASCADE`) instead of `repo_id`; on startup the coverage store migrates databases that still use the old `repo_id` schema (renaming the column, rebuilding the table, carrying over existing `tracker_coverage` links, and marking rows without a link with a negative repository id that is resolved to the tracker created for the repository at startup). It also adds `coverage.external_id` (bare UUIDv7 in the DB) to existing rows and creates a unique index on it.

`GET /api/coverages/{trackerId}/preview` uses only `requireTrackerAuth` + `InjectTracker` + `RequireReadPermission` (no SCM access needed). Its series and values are virtual (not stored in `tracker_series` / `tracker_value`), so their `id` fields are omitted.

## Coverage Type Trackers

Coverage trackers are created through the coverage API; the tracker package has no coverage knowledge.

### `POST /api/coverages`

Creates a coverage-type tracker and links it to a repository. Requires authentication.

Request:

```json
{ "name": "string", "visibility": "public|private", "repo_id": "repo_<uuidv7>" }
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Tracker name (user-supplied) |
| `visibility` | "public" \| "private" | yes | Access control |
| `repo_id` | string | yes | Repository external ID to link (`repo_<uuidv7>`; invalid format -> 400, unknown -> 404) |

Responses:

| Status | Condition |
|--------|-----------|
| 201 | Tracker created and linked (`tracker.TrackerModel`) |
| 400 | Invalid body, missing/invalid fields |
| 403 | Not authenticated |
| 404 | Repository not found |
| 409 | Repository already has a coverage tracker |

- On startup, `coverage.MigrateCoverageTrackers` creates a coverage tracker for every repository that has none, creating trackers via the tracker service and linking them through `coverage.Link`. It then re-keys coverage history that the schema migration marked with a negative repository id to the linked tracker id, and deletes rows whose repository no longer exists. Coverage trackers are owned by the superuser (`coverage.CoverageTrackerOwnerID`, id=1).
- Preview: served by `CoverageHandler.HandleCoveragePreview` at `/api/coverages/{trackerId}/preview`, fetching from `CoverageStore.Timeline(trackerID, 20)`
- Series/values endpoints return 400 (no direct data management)
- Detail view: `/coverages/:trackerId` shows coverage charts and file browser

## Upload

Upload command specifies the tracker external ID via `--tracker` flag and POSTs to `/api/coverages/:trackerId`.

A successful upload (`CoverageStore.Put`) returns the coverage external ID (`cov_<uuidv7>`), which is stable across re-uploads of the same revision. It also bumps the `last_updated_at` of the linked coverage tracker, so the tracker list reflects the latest upload. It is a no-op if the `tracker`/`tracker_coverage` tables do not exist.

## Key Files

| File | Purpose |
|------|---------|
| `coverage/external_id.go` | Coverage external ID (`cov_<uuidv7>`) type and helpers |
| `coverage/coverage_store.go` | SQLite store for coverage/coverage_entry/coverage_block |
| `coverage/coverage_handler.go` | HTTP handlers |
| `coverage/upload.go` | Coverage upload logic |
| `coverage/command.go` | CLI commands |
| `server/server.go` | Route mounting, injectTrackerCoverage middleware |
| `frontend/src/coverage.tsx` | Coverage frontend pages |
