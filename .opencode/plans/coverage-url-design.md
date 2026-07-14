# Coverage URL Refactoring Design

## Goal

Change coverage URLs from repository-centric (`/repos/:repo_id/coverages`) to tracker-centric (`/coverages/:trackerId`).

## New URL Structure

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
| `/api/coverages/:trackerId` | Coverage list |
| `/api/coverages/:trackerId/:index` | Coverage at index |
| `/api/coverages/:trackerId/:index/:entry/files` | File list |
| `/api/coverages/:trackerId/:index/:entry/files/*` | File view |

## Design Decisions

1. **Coverage type returns 404 on `/trackers/:trackerId`**: Only `/coverages/:trackerId` shows coverage detail
2. **Shared features stay on `/trackers/:trackerId`**: Like, visibility, members remain on tracker URLs (coverage type accessible via API)
3. **Upload uses `--tracker` flag**: Upload command specifies tracker ID, POSTs to `/api/coverages/:trackerId`
4. **Middleware approach**: New `injectTrackerCoverage` middleware resolves `trackerId` -> `repo_id` -> repo
5. **Backwards compatibility**: Old routes (`/repos/:repo_id/coverages`) kept during migration, can be removed later
6. **API URL flexibility**: Both `/api/trackers/:trackerId` and `/api/coverages/:trackerId` work for coverage type

## Implementation Phases

### Phase 1: Frontend URL Changes

1. Add `coverageTrackerRoute` to `coverage.tsx` with new loaders using `/api/coverages/:trackerId`
2. Add `/coverages` route to `main.tsx`
3. Update `tracker_coverage.tsx` to fetch from `/api/coverages/${tracker.id}`
4. Update `CoverageSegment` and `CoverageListContent` to support both URL modes

### Phase 4: 404 Behavior

1. `TrackerDetailRouter` returns 404 for coverage type
2. `TrackerCard` links to `/coverages/:trackerId` for coverage type
3. `TrackerDetailEdit` back link points to `/coverages/:trackerId` for coverage type
4. `TrackerCreate` redirects to `/coverages/:trackerId` after creating coverage tracker

### Phase 2: API Changes

1. Add `FindRepoIDByTrackerID()` to `tracker/service.go`
2. Add `injectTrackerCoverage` middleware to `server/server.go`
3. Mount `/api/coverages/{trackerId}` route

### Phase 3: Upload Changes

1. Add `--tracker` flag to upload command
2. Update `upload()` to POST to `/api/coverages/{trackerId}`

## Files to Change

| File | Changes |
|------|---------|
| `tracker/service.go` | Add `FindRepoIDByTrackerID()` |
| `server/server.go` | Add `injectTrackerCoverage` middleware, mount new routes |
| `frontend/src/coverage.tsx` | New route array, loaders, URL helpers |
| `frontend/src/main.tsx` | Add `/coverages` route |
| `frontend/src/tracker_coverage.tsx` | Update API URL |
| `coverage/upload.go` | Accept trackerID, POST to new URL |
| `coverage/command.go` | Add `--tracker` flag |

## Middleware Design

New `injectTrackerCoverage` middleware in `server/server.go`:

1. Parse `trackerId` from URL param
2. Look up tracker via `tracker.Service.FindRepoIDByTrackerID()`
3. Look up repo via `repos.Find(repoID)`
4. Look up `RepositoryManager` via `findRepositoryManager()`
5. Inject `core.Repository` and `core.RepositoryClient` into context
6. Handle access control (session or API key auth)

This middleware replaces `injectRepo` for the new coverage endpoints.

## URL Construction Helpers (Frontend)

```typescript
// New helpers for tracker-based URLs
export function makeCoverageTrackerPath(params: Params) {
  return `coverages/${params.trackerId}`
}

export function makeCoverageTrackerEntryPath(params: Params) {
  return `${makeCoverageTrackerPath(params)}/${params.index}/${params.entry}`
}

// Updated to support both modes
function buildEntryUrl(params: Params, cov: Coverage, entryName: string): string {
  if (params.trackerId) {
    return `/coverages/${params.trackerId}/${cov.index}/${entryName}`
  }
  return `/repos/${params.repo_id}/coverages/${cov.index}/${entryName}`
}
```
