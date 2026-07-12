# Tracker Search Feature Design

## Overview

Add a tracker search feature to the top page (`/`). Users can search trackers by name. The top page currently shows `RepoList` (repository + coverage links), which will be replaced with a tracker search page.

## Behavior

| State | Search Query | Display |
|-------|-------------|---------|
| Not logged in | None | Empty (nothing to show) |
| Not logged in | Provided | Public trackers matching name |
| Logged in | None | User's trackers (members + liked) — all |
| Logged in | Provided | User's trackers matching name + public trackers matching name |

### Search Scope Logic

- **No query + logged in**: Show all user's trackers (members + liked). Same as current My Trackers.
- **Query provided + logged in**: Filter user's trackers by name AND include public trackers matching name.
- **Query provided + not logged in**: Public trackers matching name only.
- **No query + not logged in**: Empty list.

## API Change

`GET /api/tracker` — add `q` query parameter:

```
GET /api/tracker?q=mora&page=1&per_page=12
```

- `q`: Text search (partial match on tracker name, `LIKE '%query%'`)
- Omitted: Current behavior (user-scoped list)

Response format unchanged (`ListTrackersResponse`).

## Backend Changes

### `tracker/store.go` — `listTrackers` signature change

```go
func (s *trackerStore) listTrackers(userID int64, query string, page, perPage int) ([]TrackerResponse, int, error)
```

SQL WHERE clause branching:

**query empty + logged in** (initial load):
```sql
WHERE m.user_id IS NOT NULL OR l.user_id IS NOT NULL
```

**query provided + logged in**:
```sql
WHERE (
    (m.user_id IS NOT NULL OR l.user_id IS NOT NULL)
    OR t.visibility = 'public'
) AND t.name LIKE '%' || ? || '%'
```

**query provided + not logged in**:
```sql
WHERE t.visibility = 'public' AND t.name LIKE '%' || ? || '%'
```

Both count and select queries are updated accordingly. When query is empty, the LIKE parameter is not bound.

### `tracker/handler.go` — `listTrackers` handler

```go
uid, ok := UserIDFromContext(r.Context())
if !ok {
    uid = 0
}

q := r.URL.Query().Get("q")

// Not logged in + no query → empty list
if q == "" && !ok {
    render.JSON(w, ListTrackersResponse{
        Trackers: []TrackerResponse{}, Total: 0, Page: 1, PerPage: 0,
    }, http.StatusOK)
    return
}

trackers, total, err := h.store.listTrackers(uid, q, page, perPage)
```

Key change: Previously, anonymous users always got an empty list. Now they can search public trackers when `q` is provided.

## Frontend Changes

### `frontend/src/tracker.tsx`

**Export TrackerCard** (currently module-private):

```diff
- const TrackerCard = ({
+ export const TrackerCard = ({
```

**Add `query` parameter to `listTrackers` API function**:

```typescript
async function listTrackers(page?: number, perPage?: number, query?: string): Promise<PaginatedTrackers> {
  const params = new URLSearchParams()
  if (page) params.set('page', String(page))
  if (perPage) params.set('per_page', String(perPage))
  if (query) params.set('q', query)
  const qs = params.toString()
  const url = qs ? `/api/tracker?${qs}` : '/api/tracker'
  const resp = await fetch(url)
  if (!resp.ok) throw resp
  return resp.json()
}
```

### `frontend/src/main.tsx`

**Remove** `RepoList` component and `loadRepoList` function.

**Add** `TrackerSearchPage` component:

```tsx
const TrackerSearchPage = (): React.JSX.Element => {
  const [query, setQuery] = useState('')
  const [trackers, setTrackers] = useState<TrackerResponse[]>([])
  const [previews, setPreviews] = useState<Map<number, PreviewData>>(new Map())
  const [searching, setSearching] = useState(false)
  const [initial, setInitial] = useState(true)

  // Initial load: fetch user's trackers if logged in
  useEffect(() => {
    listTrackers(1, 100).then(data => {
      setTrackers(data.trackers)
      setInitial(false)
    }).catch(() => setInitial(false))
  }, [])

  const handleSearch = async () => {
    const q = query.trim()
    setSearching(true)
    try {
      const data = await listTrackers(1, 12, q || undefined)
      setTrackers(data.trackers)
      // Fetch preview data for each tracker...
    } finally { setSearching(false) }
  }

  return (
    <div>
      <div className="flex gap-2 mb-6">
        <input type="search" value={query}
               onChange={e => setQuery(e.target.value)}
               onKeyDown={e => e.key === 'Enter' && handleSearch()}
               placeholder="Search trackers..."
               className="flex-1 border rounded px-3 py-2" />
        <Button onClick={handleSearch} disabled={searching}>Search</Button>
      </div>
      {initial && <p>Loading...</p>}
      {!initial && !searching && trackers.length === 0 && !query && (
        <p>No trackers. Create one from the menu.</p>
      )}
      {!searching && trackers.length === 0 && query && (
        <p>No trackers found.</p>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {trackers.map(t => (
          <TrackerCard key={t.id} tracker={t} preview={previews.get(t.id)} />
        ))}
      </div>
    </div>
  )
}
```

**Router change**:

```diff
  {
    index: true,
-   element: <RepoList />,
-   loader: loadRepoList,
+   element: <TrackerSearchPage />,
  },
```

## UI Layout

```
Top Page (/)
┌─────────────────────────────────┐
│ [🔍 Search trackers...    ] [Search] │
├─────────────────────────────────┤
│ ┌──────────┐  ┌──────────┐     │
│ │ Track A  │  │ Track B  │     │  ← TrackerCard (shared with My Trackers)
│ │ [charts] │  │ [charts] │     │
│ └──────────┘  └──────────┘     │
└─────────────────────────────────┘
```

- Logged in, no query: Shows user's trackers (members + liked) in card grid
- After searching: Shows filtered results (user's matching + public matching)
- TrackerCard component is shared with the `/tracker` (My Trackers) page

## Files Changed

| File | Change |
|------|--------|
| `tracker/store.go` | Add `query string` param to `listTrackers`, SQL branching |
| `tracker/handler.go` | Parse `?q=` param, handle anonymous + no query |
| `frontend/src/tracker.tsx` | Export `TrackerCard`, add `query` param to `listTrackers` API function |
| `frontend/src/main.tsx` | Replace `RepoList` with `TrackerSearchPage`, remove `loadRepoList` |
| `tracker/store_test.go` | Add tests for search query |
| `tracker/handler_test.go` | Add tests for `?q=` param and anonymous behavior |

## Tests

### store_test.go

- query empty + logged in: user's trackers all (same as before)
- query empty + not logged in: empty list
- query provided + logged in: user's trackers (name match) + public (name match)
- query provided + not logged in: public only (name match)

### handler_test.go

- `?q=` parameter parsing
- Anonymous + no query → empty list
- Anonymous + query → public trackers only
