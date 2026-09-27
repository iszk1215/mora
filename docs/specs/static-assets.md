# Static File Serving and Caching

## Overview

The frontend is embedded into the binary (`//go:embed all:static` in
`server/server.go`) and served by one handler registered as the last route,
`GET /*`. It resolves each request itself: a path that names a file is served as
that file, and every other path is a client-side route and gets `index.html`.

Two problems motivated the current design.

1. **Caching.** The handler used to be a bare `http.FileServer(http.FS(embed.FS))`.
   `embed.FS` reports a zero modification time, so `net/http` emitted neither
   `Last-Modified` nor `ETag`, and the browser re-downloaded every asset (about
   1.4MB of JS/CSS/fonts) on each reload.
2. **Addressing.** The router rewrote every unmatched path to `/` before handing
   it to the file server, so `index.html` was the only thing the site could ever
   return. A file copied out of `frontend/public/` (`favicon.svg`, `robots.txt`,
   `manifest.webmanifest`, `sw.js`) could not be served at all: `/favicon.svg`
   answered with the HTML entry point and a `200`.

## One Rule for the Cache Policy

> A file the bundler content-hashed is `immutable` for a year. Every other file
> is revalidated with an `ETag`.

| Response | `Cache-Control` | Validator |
|----------|-----------------|-----------|
| A file listed in the bundler manifest | `public, max-age=31536000, immutable` (200 only) | none needed |
| Everything else that is served (`index.html`, files from `public/`, a file the manifest omits) | `no-cache` | strong `ETag` (sha256 of the body) |

`no-cache` still allows storing the response but forces revalidation, and the
`ETag` lets that revalidation answer `304` with an empty body instead of the
whole file.

### The manifest is the source of truth

Vite writes `.vite/manifest.json` next to the bundle (`build.manifest: true` in
`frontend/vite.config.ts`). Each entry names a chunk's `file` plus the `css` and
`assets` it pulled in, and it covers every file the bundler emitted, because the
bundler had to hash them to name them. `loadImmutablePaths` collects that set at
startup.

Files copied verbatim out of `frontend/public/` are **not** in the manifest: they
are not part of the module graph, so nothing hashed them. That is exactly the
distinction the policy needs, and it is why the policy is not a directory rule:

- `assets/index-BBgaulyO.js` is in the manifest -> `immutable`.
- `favicon.svg` sits at the root and is not in the manifest -> `no-cache`.
- A hashed-looking file the manifest does not list -> `no-cache`. Its URL does not
  guarantee its body, so a rebuild that changes it is picked up immediately.
- A future bundler writing hashed files outside `assets/` stays `immutable`,
  because the manifest, not the directory, decides.

A manifest that is missing or unparsable is **not** fatal: the server logs a
warning and revalidates everything. That costs a round trip per file and is
always correct, whereas guessing would risk pinning a mutable file for a year.

## Path Resolution

`frontendHandler.resolve` turns a request path into a name inside the embedded
filesystem:

- `path.Clean("/"+requestPath)` then `fs.ValidPath` reject anything that climbs
  out of the site; `..` cannot reach a file the embed does not contain.
- Only a **regular file** resolves. A directory, an unknown name, and the
  manifest itself do not.

The manifest is build metadata, not site content, so it is never published: a
request for `/.vite/manifest.json` is a `404`, not the file.

### A miss is a 404 inside a build output directory, and the entry point elsewhere

Under `assets/` and `.vite/`, a request names one specific file, so a miss must
stay a `404` (`underBuildOutput`). Answering it with `index.html` would hand
HTML to a script, stylesheet, or font request, and the browser would fail with a
parse error instead of a clean 404. Everywhere else, a miss is a client-side
route.

The **bare** directories (`/assets`, `/assets/`, `/.vite`) are not inside that
rule: they name no file, so they fall through to the entry point. Handing them to
the file server produced a `301` plus an HTML listing of every built file name.

No extension heuristic is involved. `GET /trackers/report.json` is a client-side
route and returns the entry point, because the router cannot tell a client route
from a mistyped asset by file extension alone.

## Implementation

`newFrontendHandler(fsys fs.FS)` in `server/server.go` builds the handler;
`initFrontendFileServer` only resolves `fs.Sub(staticFS, "static/public")` and
delegates to it. Taking an `fs.FS` is the test seam: `server/static_test.go`
feeds a `testing/fstest.MapFS` with fixed file names and a hand-written manifest,
so the tests do not depend on the committed build output or on hash names that
change on every rebuild.

- **Entry point.** `index.html` is read once at startup, which also fails fast if
  a build is missing it, and its `ETag` is computed once. It is served from
  memory through `http.ServeContent` rather than through the file server, because
  the file server redirects a request for `/index.html` to `/` instead of
  returning it. That redirect is correct for a canonical URL and wrong for the
  fallback.
- **Other files.** The file server serves the body, so a large asset is streamed
  and never buffered. The request is copied with its URL rewritten to the
  resolved name (`requestFor`), so the handler decides what a path means instead
  of the router rewriting it.
- **ETags of other files** are computed on first request and cached per name
  (`sync.Map`). The body is streamed into the hash, so hashing a 1.2MB chunk
  does not allocate a copy of it. Bundle files never need one: they are immutable.
- **`immutableCache`** wraps `WriteHeader` and sets `Cache-Control` only for
  `200`. Setting it unconditionally would let a `404` or a redirect be pinned in
  the browser cache for a year.
- **`http.ServeContent` and the file server** already implement the RFC 7232
  preconditions against the `ETag` on the response, so `If-None-Match` (a
  matching tag, a tag inside a list, or `*`) is answered with `304` by the
  standard library rather than by hand-written code.

## Known Behavior

| Request | Response | Note |
|---------|----------|------|
| `GET /` , `GET /trackers/abc` | `200` + `no-cache` + `ETag`, or `304` | Client-side routes. |
| `GET /favicon.svg` | `200` + `no-cache` + `ETag`, or `304` | A file from `frontend/public/`. Not in the manifest, so never `immutable`. |
| `GET /assets/index-BBgaulyO.js` | `200` + `immutable` | In the manifest. |
| `GET /assets/missing.js` | `404`, no `Cache-Control` | A script must not receive HTML. |
| `GET /assets`, `GET /assets/` | `200` + entry point | A bare directory is not a file. |
| `GET /.vite/manifest.json` | `404` | Build metadata is not published. |
| `GET /index.html` | `301` to `/` | Canonical redirect from the file server. |
| `GET /vite.svg` | `200` + entry point | The Vite template's icon no longer exists in the repo, so it is an unknown path. Harmless. |
| `GET /nope.js` | `200` + entry point | A miss outside a build directory is a client-side route. |

The `favicon.svg` change is invisible until each browser's icon cache expires;
use a hard reload or clear the cache to see it.

## Tests

`server/static_test.go`:

| Test | Asserts |
|------|---------|
| `TestNewFrontendHandler_ImmutableFollowsTheManifest` | Manifest-listed files are `immutable` with no `ETag`; a hashed file outside `assets/` is too; a hashed-looking file the manifest omits is revalidated |
| `TestNewFrontendHandler_RevalidatedFiles` | A `public/` file is served with the right content type, `no-cache` and a quoted `ETag`; that tag yields `304`, a stale tag yields the file |
| `TestNewFrontendHandler_EntryPointRevalidation` | `200` + `no-cache` + quoted `ETag`; `304` for a match, `*`, and a list; `200` for a stale tag; SPA paths return the entry point; `HEAD` has no body |
| `TestNewFrontendHandler_ETagTracksContent` | Same content -> same ETag, changed content -> changed ETag |
| `TestNewFrontendHandler_EntryPointMissing` | A filesystem without `index.html` returns an error |
| `TestNewFrontendHandler_WithoutAManifestEverythingRevalidates` | A missing manifest degrades to revalidating every file |
| `TestNewFrontendHandler_BuildMetadataIsNotServed` | The manifest is not published; a missing bundle file is a `404` without `index.html` in the body; bare build directories serve the entry point |
| `TestNewFrontendHandler_PathsCannotEscapeTheSite` | A climb out lands on the entry point; `/index.html` canonicalizes to `/` |
| `TestEmbeddedFrontend_ManifestDescribesEveryBundleFile` | The committed build ships a manifest that lists every file under `assets/` |
| `TestHandler_StaticCaching` | Routing through `Handler()` keeps the policy for bundle files, root files, and the fallback, and sets no session cookie |

## Manual Verification

```sh
bin/mora web -c mora.conf &

asset=$(ls server/static/public/assets/index-*.js | head -1 | sed 's|server/static/public||')

# immutable: content-hashed by the bundler
curl -sSD - -o /dev/null "http://localhost:4000$asset" | grep -i cache-control

# a public/ file is revalidated
etag=$(curl -sSD - -o /dev/null http://localhost:4000/favicon.svg | awk '/[Ee]tag/{print $2}' | tr -d '\r')
curl -sSD - -o /dev/null http://localhost:4000/favicon.svg | grep -i 'cache-control\|content-type'
curl -sSD - -o /dev/null -H "If-None-Match: $etag" http://localhost:4000/favicon.svg | head -1   # 304

# a client-side route falls back, a missing asset does not
curl -sSD - -o /dev/null http://localhost:4000/trackers/abc | head -1                        # 200
curl -sSD - -o /dev/null http://localhost:4000/assets/missing.js | head -1                   # 404
curl -sSD - -o /dev/null http://localhost:4000/.vite/manifest.json | head -1                # 404
```

A browser reload should issue no bundle requests on the second load; the entry
point and the icon are revalidated with `If-None-Match` and answered with `304`.
