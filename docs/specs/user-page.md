# User Page Specification

## Overview

User page (`/users/:userName`) shows the trackers owned by a user. Anyone can view a user's page; visibility of each tracker follows its setting.

## Visibility Rules

| Viewer | Display |
|--------|---------|
| Anonymous / other user | Public trackers only |
| The owner (logged in) | Public + private trackers |

## API

`GET /api/users/:userName`

- Returns the public user profile (`server.User`)

`GET /api/users/:userName/trackers?q=<query>&page=N&per_page=N&include=preview`

- `q`: partial match on tracker name, scoped to this user's trackers
- `include=preview`: embed each tracker's preview data (`series` + latest values)
  so cards render without per-card round trips (always sent by the frontend)
- Visibility: public only for everyone; private also for the owner
- Response format: `tracker.ListTrackersResponse`

## Backend

### tracker/store.go

`listTrackersByOwner(ownerID, viewerID int64, searchQuery string, page, perPage int)`:

- `WHERE t.owner_id = ? AND (t.visibility = 'public' OR t.owner_id = viewerID)`
- Applies `name LIKE ?` when a search query is present
- Same pagination and `owner_name` / `role` / `liked` / `like_count` fields as `listTrackers`

### server/userpage.go

- `handleUserGet`: resolve username via `UserStore.FindByUsername`, 404 when missing
- `handleUserTrackers`: resolve owner, read viewerID from context, delegate to `Service.ListTrackersByOwner`
- When `include=preview`, embed preview data via the shared `attachPreviews`
  helper (`server/tracker_list.go`), using batched queries

## Frontend

### user.tsx

- `UserPage` component: search box at the top, username heading, tracker card grid, Prev/Next pagination (no breadcrumb, no avatar)
- `loadUserPage` loader: fetch `/api/users/:userName`, throw 404 response when missing
- Cards pass `fromUser` navigation state so the tracker detail breadcrumb can return to this user's search
- The trackers request always sends `include=preview`; cards get their data from the list response (`previewsFromTrackers`), no separate preview fetches

### main.tsx

- Route `/users/:userName` with crumb `user.username` (no `@` prefix)
- Tracker detail/edit breadcrumb: when navigated from a user page search, "Search Results" links back to `/users/:userName?q=...`

## Files

| File | Change |
|------|--------|
| `tracker/store.go` | `listTrackersByOwner` |
| `tracker/service.go` | Expose `ListTrackersByOwner` |
| `server/userpage.go` | User profile + user trackers handlers |
| `server/server.go` | `/api/users` routes |
| `frontend/src/user.tsx` | `UserPage` + route |
| `frontend/src/main.tsx` | User page route + breadcrumb handling |
| `frontend/src/tracker.tsx` | `TrackerCard` `fromUser` prop |

## Tests

- Store: visibility (anonymous / owner / other), search scoping, pagination
- Server: profile lookup, 404, visibility, search, pagination
- Frontend: user page render, search, pagination, breadcrumbs
