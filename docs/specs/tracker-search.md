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

`GET /api/trackers?q=<query>&page=N&per_page=N`

- `q`: partial match on tracker name (`LIKE '%query%'`)
- Omitted: user-scoped list (default behavior)
- Response format unchanged (`ListTrackersResponse`)

## Backend

### tracker/store.go

`listTrackers` adds `query string` parameter. SQL WHERE clause branches:

- No query + logged in: `t.owner_id = ? OR EXISTS(member) OR EXISTS(like)`
- Query + logged in: `(owner OR member OR liked OR public) AND name LIKE ?`
- Query + not logged in: `WHERE public AND name LIKE ?`

### tracker/handler.go

- Parse `?q=` parameter
- Anonymous + no query: return empty list immediately

## Frontend

### Top page (`/`)

- Replace `RepoList` with `TrackerSearchPage`
- Search input + card grid
- `TrackerCard` component shared with `/trackers` page

### tracker.tsx

- Export `TrackerCard` (previously module-private)
- Add `query` parameter to `listTrackers` API function

## Files

| File | Change |
|------|--------|
| `tracker/store.go` | `listTrackers` query param, SQL branching |
| `tracker/handler.go` | Parse `?q=`, anonymous + no query handling |
| `frontend/src/trackers.tsx` | Export `TrackerCard`, query param |
| `frontend/src/main.tsx` | `TrackerSearchPage` replaces `RepoList` |
| `tracker/store_test.go` | Search query tests |
| `tracker/handler_test.go` | `?q=` param tests |

## Tests

- Query empty + logged in: user's trackers
- Query empty + not logged in: empty list
- Query + logged in: user's matching + public matching
- Query + not logged in: public only matching
