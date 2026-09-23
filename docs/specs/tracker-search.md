# Tracker Search Specification

## Overview

Tracker search on the top page (`/`). Users search trackers by name. Replaces the previous `RepoList` view.

## Search Behavior

| State | Query | Display |
|-------|-------|---------|
| Not logged in | None | Empty |
| Not logged in | Provided | Public trackers matching name |
| Logged in | None | User's trackers (owner + members + liked) |
| Logged in | Provided | User's trackers + public trackers, filtered by name |

## API

`GET /api/trackers?q=<query>&page=N&per_page=N&include=preview`

- `q`: partial match on tracker name (`LIKE '%query%'`)
- `include=preview`: embed each tracker's preview data (`series` + latest values, up to 20 per series) so cards render without extra round trips
- Omitted: user-scoped list (default behavior)
- Response format unchanged (`ListTrackersResponse`), with `series` omitted per tracker unless `include=preview`

## Backend

### server/tracker_list.go (server package)

The list handler now lives in the `server` package because the fold needs both the
tracker service and the coverage service:

- `handleTrackersList`: serves `GET /api/trackers` (mounted as a static route before
  the tracker handler's mount in `server/server.go`)
- `attachPreviews`: when `include=preview`, batched queries embed preview data:
  - tracker-type trackers via `Service.BatchPreview` (2 round trips total)
  - coverage-type trackers via `TimelineByTrackerIDs` (1 round trip), converted with `TimelineToPreviewSeries`
- Access scoping is inherent in the list WHERE clause, so no per-tracker permission
  re-checks are needed

### tracker/store.go

`listTrackers` adds `query string` parameter. SQL WHERE clause branches:

- No query + logged in: `t.owner_id = ? OR EXISTS(member) OR EXISTS(like)`
- Query + logged in: `(owner OR member OR liked OR public) AND name LIKE ?`
- Query + not logged in: `WHERE public AND name LIKE ?`

New batched queries for `include=preview`:

- `listSeriesByTrackerIDs`: all series for the given trackers in 1 round trip
- `listLatestValuesByTrackerIDs`: latest up-to-`limit` values per series in 1
  round trip (window function over `tracker_value`, joined to `tracker_series`)

### server/handler.go

- Parse `?q=` parameter
- Anonymous + no query: return empty list immediately

## Frontend

### Top page (`/`)

- Replace `RepoList` with `TrackerSearchPage`
- Search input + card grid
- `TrackerCard` component shared with `/trackers` page
- The list is fetched with `include=preview`; preview maps are built from the
  response (`previewsFromTrackers`) instead of one `fetchPreview` call per tracker

### tracker.tsx

- Export `TrackerCard` (previously module-private)
- Add `query` + `includePreview` parameters to `listTrackers` API function
- Remove `fetchPreview` (replaced by the fold)

## Files

| File | Change |
|------|--------|
| `tracker/store.go` | `listTrackers` query param, SQL branching, batched series/value queries |
| `tracker/service.go` | `ListTrackers` + `BatchPreview` |
| `tracker/handler.go` | Remove local list handler (moved to server package) |
| `server/tracker_list.go` | `handleTrackersList` + `attachPreviews` (include=preview fold) |
| `coverage/coverage_store.go` | `TimelineByTrackerIDs` |
| `frontend/src/tracker-api.ts` | `listTrackers` query/include params, `previewsFromTrackers`, remove `fetchPreview` |
| `frontend/src/main.tsx` | `TrackerSearchPage` fetches with `include=preview` |
| `tracker/store_test.go` | Search query tests |
| `server/tracker_list_test.go` | List, search, and preview-fold handler tests |

## Tests

- Query empty + logged in: user's trackers
- Query empty + not logged in: empty list
- Query + logged in: user's matching + public matching
- Query + not logged in: public only matching
