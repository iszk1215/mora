# Static Asset Caching

## Overview

The frontend bundle is embedded into the binary (`//go:embed static` in
`server/server.go`) and served by one handler that backs both the `/assets/*`
route and the SPA fallback. Before this policy existed the handler was a bare
`http.FileServer(http.FS(embed.FS))`: `embed.FS` reports a zero modification
time, so `net/http` emitted neither `Last-Modified` nor `ETag`, and the browser
re-downloaded every asset (about 1.4MB of JS/CSS/fonts) on each reload.

## Cache Policy

| Path | Served by | `Cache-Control` | Validator |
|------|-----------|-----------------|-----------|
| `/assets/*` | `http.FileServer` | `public, max-age=31536000, immutable` (200 only) | none needed |
| everything else (SPA entry point) | `http.ServeContent` over the embedded `index.html` | `no-cache` | strong `ETag` (sha256 of `index.html`) |

**Why `immutable` is safe under `/assets/`**: Vite/rolldown emit a content hash
in every file name (`index-BBgaulyO.js`, `noto-sans-jp-104-wght-normal-DU15_Tmk.woff2`).
A changed body always means a changed URL, so a cached copy can never be stale.
Fonts live under `/assets/` too and therefore need no separate handling.

**Why the entry point must revalidate**: its URL carries no hash, so it is
always the freshest HTML that must be served. `no-cache` still allows caching
but forces revalidation, and the `ETag` lets the revalidation answer with `304`
and an empty body instead of the full document.

## Implementation

`newFrontendHandler(fsys fs.FS)` in `server/server.go` builds the handler;
`initFrontendFileServer` only resolves `fs.Sub(staticFS, "static/public")` and
delegates to it. Taking an `fs.FS` is the test seam: `server/static_test.go`
feeds a `testing/fstest.MapFS` with fixed file names, so the tests do not depend
on the committed build output or on hash names that change on every rebuild.

```go
index, err := fs.ReadFile(fsys, "index.html")   // read once at startup
etag := fmt.Sprintf("\"%x\"", sha256.Sum256(index))
```

Dispatch is a path prefix check (`isAssetPath`):

- asset path -> `http.FileServer` wrapped in `immutableCache`
- anything else -> `index.html`

`http.ServeContent` already implements the RFC 7232 preconditions against the
`ETag` set on the response, so `If-None-Match` (a matching tag, a tag inside a
list, or `*`) is answered with `304` by the standard library rather than by
hand-written code. It also strips `Last-Modified` when an `ETag` is present,
which is why the response carries no `Last-Modified`.

### Two deliberate details

- **Only successful responses get `immutableCache`.** `immutableCache` wraps
  `WriteHeader` and sets `Cache-Control` only for `200`. Setting it
  unconditionally would let a `404` or a redirect be pinned in the browser
  cache for a year.
- **The bare `/assets/` directory is not an asset.** `isAssetPath` rejects
  `/assets` and `/assets/` so they fall through to the entry point. Handing
  them to the file server used to produce a `301` plus an HTML listing of all
  141 built file names.

## Known Behavior

| Request | Response | Note |
|---------|----------|------|
| `GET /vite.svg` | `200` + `index.html` (776B) | The SPA fallback rewrites every unmatched path to `/`, so root-level files that do not exist in `static/public` return the entry point. Pre-existing, out of scope. |
| `GET /assets/missing.js` | `404`, no `Cache-Control` | Never cached immutably. |
| `GET /assets`, `GET /assets/` | `200` + `index.html` | Directory listing removed. |

## Tests

`server/static_test.go`:

| Test | Asserts |
|------|---------|
| `TestNewFrontendHandler_AssetsAreImmutable` | 200 + `immutable`; css/font share the policy; 404 has no `Cache-Control`; `/assets` and `/assets/` return the entry point |
| `TestNewFrontendHandler_EntryPointRevalidation` | 200 + `no-cache` + quoted `ETag`; 304 for a match, `*`, and a list; 200 for a stale tag; SPA paths return the entry point; `HEAD` has no body |
| `TestNewFrontendHandler_ETagTracksContent` | Same content -> same ETag, changed content -> changed ETag |
| `TestNewFrontendHandler_IndexMissing` | A filesystem without `index.html` returns an error |
| `TestHandler_StaticCaching` | Routing through `Handler()` keeps the policy on `/assets/*` and `/` and sets no session cookie |

## Manual Verification

```sh
bin/mora web -c mora.conf &

# immutable asset
curl -sSD - -o /dev/null http://localhost:4000/assets/index-BBgaulyO.js | grep -i cache-control

# entry point, then revalidate
etag=$(curl -sSD - -o /dev/null http://localhost:4000/ | awk '/[Ee]tag/{print $2}' | tr -d '\r')
curl -sSD - -o /dev/null -H "If-None-Match: $etag" http://localhost:4000/ | head -1   # 304
```

A browser reload should issue no asset requests at all on the second load; the
entry point is revalidated with `If-None-Match` and answered with `304`.
