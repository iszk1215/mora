package server

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

const (
	testIndexHTML      = "<!DOCTYPE html><html><body>mora</body></html>"
	testAssetPath      = "/assets/index-ABCDEFGH.js"
	testImmutableValue = "public, max-age=31536000, immutable"
	testRevalidate     = "no-cache"
)

// testManifest mirrors what Vite writes to .vite/manifest.json: one entry per
// chunk, naming the hashed file it emitted plus the CSS and assets it pulled in.
// Files copied verbatim out of public/ are absent, which is exactly what the
// server needs to tell the two kinds apart.
var testManifest = `{
  "src/main.tsx": {
    "file": "assets/index-ABCDEFGH.js",
    "isEntry": true,
    "css": ["assets/index-Dz-ZE_zq.css"]
  },
  "node_modules/noto-sans-jp/font.css": {
    "file": "assets/noto-sans-jp-0-wght.woff2"
  },
  "public/vendor.js": {
    "file": "vendor-9xK2mQ.js"
  }
}`

func newTestFrontendHandler(t *testing.T) http.Handler {
	t.Helper()

	return newHandlerFor(t, testFrontendFS())
}

func testFrontendFS() fstest.MapFS {
	return fstest.MapFS{
		indexFileName:                      {Data: []byte(testIndexHTML)},
		manifestPath:                       {Data: []byte(testManifest)},
		"favicon.svg":                      {Data: []byte("<svg/>")},
		"vendor-9xK2mQ.js":                 {Data: []byte("vendor")},
		"assets/index-ABCDEFGH.js":         {Data: []byte("console.log(1)")},
		"assets/index-Dz-ZE_zq.css":        {Data: []byte("body{}")},
		"assets/noto-sans-jp-0-wght.woff2": {Data: []byte("font-bytes")},
		// A file the manifest does not list. Its name looks hashed, which must
		// not be what decides the policy.
		"assets/legacy-ABCDEFGH.js": {Data: []byte("legacy")},
	}
}

func newHandlerFor(t *testing.T, fsys fs.FS) http.Handler {
	t.Helper()

	handler, err := newFrontendHandler(fsys)
	require.NoError(t, err)

	return handler
}

func serve(t *testing.T, handler http.Handler, method, target string, headers map[string]string) *http.Response {
	t.Helper()

	r := httptest.NewRequest(method, target, nil)
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	return w.Result()
}

func drain(t *testing.T, res *http.Response) string {
	t.Helper()

	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	return string(body)
}

func TestNewFrontendHandler_ImmutableFollowsTheManifest(t *testing.T) {
	handler := newTestFrontendHandler(t)

	t.Run("a hashed file is cached immutably", func(t *testing.T) {
		res := serve(t, handler, http.MethodGet, testAssetPath, nil)
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, testImmutableValue, res.Header.Get("Cache-Control"))
		require.Equal(t, "console.log(1)", drain(t, res))
		require.Empty(t, res.Header.Get("ETag"),
			"immutable assets do not need a validator")
	})

	t.Run("the css and assets a chunk pulls in share the policy", func(t *testing.T) {
		for _, path := range []string{
			"/assets/index-Dz-ZE_zq.css",
			"/assets/noto-sans-jp-0-wght.woff2",
		} {
			res := serve(t, handler, http.MethodGet, path, nil)
			require.Equal(t, http.StatusOK, res.StatusCode, path)
			require.Equal(t, testImmutableValue, res.Header.Get("Cache-Control"), path)
			_ = drain(t, res)
		}
	})

	t.Run("a hashed file outside the bundle directory is immutable too", func(t *testing.T) {
		res := serve(t, handler, http.MethodGet, "/vendor-9xK2mQ.js", nil)
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, testImmutableValue, res.Header.Get("Cache-Control"))
		require.Equal(t, "vendor", drain(t, res))
	})

	t.Run("a hashed-looking file the manifest omits is revalidated", func(t *testing.T) {
		// The directory is not the signal. A file the bundler did not emit gets
		// revalidated, so replacing it in place is picked up immediately.
		res := serve(t, handler, http.MethodGet, "/assets/legacy-ABCDEFGH.js", nil)
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, testRevalidate, res.Header.Get("Cache-Control"))
		require.NotEmpty(t, res.Header.Get("ETag"))
		require.Equal(t, "legacy", drain(t, res))
	})
}

func TestNewFrontendHandler_RevalidatedFiles(t *testing.T) {
	handler := newTestFrontendHandler(t)

	t.Run("a file copied out of public/ is served", func(t *testing.T) {
		res := serve(t, handler, http.MethodGet, "/favicon.svg", nil)
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, "image/svg+xml", res.Header.Get("Content-Type"))
		require.Equal(t, testRevalidate, res.Header.Get("Cache-Control"))
		require.NotEmpty(t, res.Header.Get("ETag"))
		require.Equal(t, "<svg/>", drain(t, res))
	})

	t.Run("its ETag revalidates it", func(t *testing.T) {
		first := serve(t, handler, http.MethodGet, "/favicon.svg", nil)
		etag := first.Header.Get("ETag")
		require.NotEmpty(t, etag)
		require.Equal(t, `"`+etag[1:len(etag)-1]+`"`, etag, "ETag must be quoted")
		_ = drain(t, first)

		res := serve(t, handler, http.MethodGet, "/favicon.svg",
			map[string]string{"If-None-Match": etag})
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusNotModified, res.StatusCode)
		require.Empty(t, drain(t, res))
		require.Equal(t, etag, res.Header.Get("ETag"))
		require.Equal(t, testRevalidate, res.Header.Get("Cache-Control"))
	})

	t.Run("a stale ETag returns the whole file", func(t *testing.T) {
		res := serve(t, handler, http.MethodGet, "/favicon.svg",
			map[string]string{"If-None-Match": `"stale"`})
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, "<svg/>", drain(t, res))
	})
}

func TestNewFrontendHandler_EntryPointRevalidation(t *testing.T) {
	handler := newTestFrontendHandler(t)

	res := serve(t, handler, http.MethodGet, "/", nil)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, testRevalidate, res.Header.Get("Cache-Control"))
	require.Equal(t, testIndexHTML, drain(t, res))

	etag := res.Header.Get("ETag")
	require.NotEmpty(t, etag)
	require.Equal(t, `"`+etag[1:len(etag)-1]+`"`, etag, "ETag must be quoted")

	t.Run("matching ETag returns 304", func(t *testing.T) {
		res := serve(t, handler, http.MethodGet, "/", map[string]string{"If-None-Match": etag})
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusNotModified, res.StatusCode)
		require.Empty(t, drain(t, res))
		require.Equal(t, etag, res.Header.Get("ETag"))
		require.Equal(t, testRevalidate, res.Header.Get("Cache-Control"))
	})

	t.Run("wildcard ETag returns 304", func(t *testing.T) {
		res := serve(t, handler, http.MethodGet, "/", map[string]string{"If-None-Match": "*"})
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusNotModified, res.StatusCode)
		require.Empty(t, drain(t, res))
	})

	t.Run("ETag in a list returns 304", func(t *testing.T) {
		res := serve(t, handler, http.MethodGet, "/",
			map[string]string{"If-None-Match": `"stale", ` + etag})
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusNotModified, res.StatusCode)
	})

	t.Run("stale ETag returns the full document", func(t *testing.T) {
		res := serve(t, handler, http.MethodGet, "/", map[string]string{"If-None-Match": `"stale"`})
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, testIndexHTML, drain(t, res))
	})

	t.Run("SPA paths serve the entry point", func(t *testing.T) {
		for _, path := range []string{"/trackers/abc", "/users/kazuhisa", "/vite.svg"} {
			res := serve(t, handler, http.MethodGet, path, nil)
			require.Equal(t, http.StatusOK, res.StatusCode, path)
			require.Equal(t, etag, res.Header.Get("ETag"), path)
			require.Equal(t, testRevalidate, res.Header.Get("Cache-Control"), path)
			require.Equal(t, testIndexHTML, drain(t, res), path)
		}
	})

	t.Run("HEAD returns no body", func(t *testing.T) {
		res := serve(t, handler, http.MethodHead, "/", nil)
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, etag, res.Header.Get("ETag"))
		require.Empty(t, drain(t, res))
	})
}

func TestNewFrontendHandler_ETagTracksContent(t *testing.T) {
	etagOf := func(t *testing.T, index string) string {
		t.Helper()

		res := serve(t, newHandlerFor(t, fstest.MapFS{
			indexFileName: {Data: []byte(index)},
		}), http.MethodGet, "/", nil)
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)
		_ = drain(t, res)

		return res.Header.Get("ETag")
	}

	require.Equal(t, etagOf(t, testIndexHTML), etagOf(t, testIndexHTML),
		"the same content must produce the same ETag")
	require.NotEqual(t, etagOf(t, testIndexHTML), etagOf(t, testIndexHTML+" "),
		"a rebuilt entry point must change the ETag")
}

func TestNewFrontendHandler_EntryPointMissing(t *testing.T) {
	_, err := newFrontendHandler(fstest.MapFS{
		manifestPath:               {Data: []byte(testManifest)},
		"assets/index-ABCDEFGH.js": {Data: []byte("console.log(1)")},
	})
	require.Error(t, err)
}

func TestNewFrontendHandler_WithoutAManifestEverythingRevalidates(t *testing.T) {
	// A build without a manifest still has to work: revalidating a hashed file
	// costs a round trip, marking a revalidated file immutable would not.
	fsys := testFrontendFS()
	delete(fsys, manifestPath)
	handler := newHandlerFor(t, fsys)

	for _, path := range []string{testAssetPath, "/favicon.svg", "/"} {
		res := serve(t, handler, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, res.StatusCode, path)
		require.Equal(t, testRevalidate, res.Header.Get("Cache-Control"), path)
		require.NotEmpty(t, res.Header.Get("ETag"), path)
		_ = drain(t, res)
	}
}

func TestNewFrontendHandler_BuildMetadataIsNotServed(t *testing.T) {
	handler := newTestFrontendHandler(t)

	t.Run("the manifest is not published", func(t *testing.T) {
		res := serve(t, handler, http.MethodGet, "/.vite/manifest.json", nil)
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusNotFound, res.StatusCode)
		require.NotContains(t, drain(t, res), "ABCDEFGH")
	})

	t.Run("a missing bundle file stays a 404", func(t *testing.T) {
		res := serve(t, handler, http.MethodGet, "/assets/missing-ABCDEFGH.js", nil)
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusNotFound, res.StatusCode)
		require.Empty(t, res.Header.Get("Cache-Control"),
			"a 404 carrying immutable would be cached for a year")
		require.NotContains(t, drain(t, res), testIndexHTML,
			"a script request must not receive the entry point")
	})

	t.Run("a bare build directory serves the entry point", func(t *testing.T) {
		// Neither directory has a file of its own, so both fall back instead of
		// rendering a listing of every built file name.
		for _, path := range []string{"/assets", "/assets/", "/.vite", "/.vite/"} {
			res := serve(t, handler, http.MethodGet, path, nil)
			require.Equal(t, http.StatusOK, res.StatusCode, path)
			require.Equal(t, testIndexHTML, drain(t, res), path)
		}
	})
}

func TestNewFrontendHandler_PathsCannotEscapeTheSite(t *testing.T) {
	handler := newTestFrontendHandler(t)

	t.Run("a climb out finds no file and lands on the entry point", func(t *testing.T) {
		for _, path := range []string{
			"/../etc/passwd",
			"/assets/../../../../etc/passwd",
			"/assets/..%2f..%2fetc/passwd",
			"/%2e%2e/etc/passwd",
		} {
			res := serve(t, handler, http.MethodGet, path, nil)
			require.Equal(t, http.StatusOK, res.StatusCode, path)
			require.Equal(t, testIndexHTML, drain(t, res), path)
		}
	})

	t.Run("the entry point redirects to the site root", func(t *testing.T) {
		// The file server treats /index.html as the canonical name for /, which
		// is why the entry point itself is served from memory.
		for _, path := range []string{"/index.html", "/../index.html", "/assets/../../index.html"} {
			res := serve(t, handler, http.MethodGet, path, nil)
			require.Equal(t, http.StatusMovedPermanently, res.StatusCode, path)
			require.Equal(t, "./", res.Header.Get("Location"), path)
			_ = drain(t, res)
		}
	})
}

// TestEmbeddedFrontend_ManifestDescribesEveryBundleFile guards the assumption the
// cache policy rests on: a real build lists every file it emitted. If a future
// bundler change stops recording a file, it would silently start revalidating
// and this test would point at the gap.
func TestEmbeddedFrontend_ManifestDescribesEveryBundleFile(t *testing.T) {
	fsys, err := fs.Sub(staticFS, "static/public")
	require.NoError(t, err)

	emitted := loadImmutablePaths(fsys)
	require.NotEmpty(t, emitted, "the committed build must ship a manifest")

	published := map[string]bool{}
	err = fs.WalkDir(fsys, bundleOutputDir, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		published[name] = true

		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, published)

	for name := range published {
		require.True(t, emitted[name],
			"%s is served from the bundle directory but missing from "+manifestPath, name)
	}
}

func TestHandler_StaticCaching(t *testing.T) {
	server := NewMoraServerBuilder(t).
		WithFrontendFileServer(newTestFrontendHandler(t)).
		Finish()
	router := server.Handler()

	get := func(target string) *http.Response {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))

		return w.Result()
	}

	t.Run("bundle files are immutable and skip the session", func(t *testing.T) {
		res := get(testAssetPath)
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, testImmutableValue, res.Header.Get("Cache-Control"))
		require.Empty(t, res.Cookies())
	})

	t.Run("root files are served and skip the session", func(t *testing.T) {
		res := get("/favicon.svg")
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, testRevalidate, res.Header.Get("Cache-Control"))
		require.NotEmpty(t, res.Header.Get("ETag"))
		require.Empty(t, res.Cookies())
	})

	t.Run("entry point revalidates and skips the session", func(t *testing.T) {
		res := get("/")
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, testRevalidate, res.Header.Get("Cache-Control"))
		require.NotEmpty(t, res.Header.Get("ETag"))
		require.Empty(t, res.Cookies())
	})

	t.Run("SPA fallback serves the entry point", func(t *testing.T) {
		res := get("/trackers/abc")
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, testRevalidate, res.Header.Get("Cache-Control"))
		require.NotEmpty(t, res.Header.Get("ETag"))
		require.Equal(t, testIndexHTML, drain(t, res))
	})

	t.Run("missing bundle file stays a 404", func(t *testing.T) {
		res := get("/assets/missing-ABCDEFGH.js")
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusNotFound, res.StatusCode)
		require.Empty(t, res.Header.Get("Cache-Control"))
	})
}
