package server

import (
	"io"
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
)

func newTestFrontendHandler(t *testing.T) http.Handler {
	t.Helper()

	handler, err := newFrontendHandler(fstest.MapFS{
		"index.html":                       {Data: []byte(testIndexHTML)},
		"assets/index-ABCDEFGH.js":         {Data: []byte("console.log(1)")},
		"assets/index-Dz-ZE_zq.css":        {Data: []byte("body{}")},
		"assets/noto-sans-jp-0-wght.woff2": {Data: []byte("font-bytes")},
	})
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

func TestNewFrontendHandler_AssetsAreImmutable(t *testing.T) {
	handler := newTestFrontendHandler(t)

	t.Run("hashed asset is cached immutably", func(t *testing.T) {
		res := serve(t, handler, http.MethodGet, testAssetPath, nil)
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, testImmutableValue, res.Header.Get("Cache-Control"))
		require.Equal(t, "console.log(1)", drain(t, res))
		require.Empty(t, res.Header.Get("ETag"),
			"immutable assets do not need a validator")
	})

	t.Run("other hashed assets share the policy", func(t *testing.T) {
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

	t.Run("missing asset is not cached", func(t *testing.T) {
		res := serve(t, handler, http.MethodGet, "/assets/missing-ABCDEFGH.js", nil)
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusNotFound, res.StatusCode)
		require.Empty(t, res.Header.Get("Cache-Control"),
			"a 404 carrying immutable would be cached for a year")
	})

	t.Run("assets directory serves the entry point", func(t *testing.T) {
		// The router rewrites unmatched paths to "/", so /assets and
		// /assets/ used to redirect and render a directory listing.
		for _, path := range []string{"/assets", "/assets/"} {
			res := serve(t, handler, http.MethodGet, path, nil)
			require.Equal(t, http.StatusOK, res.StatusCode, path)
			require.Equal(t, testIndexHTML, drain(t, res), path)
		}
	})
}

func TestNewFrontendHandler_EntryPointRevalidation(t *testing.T) {
	handler := newTestFrontendHandler(t)

	res := serve(t, handler, http.MethodGet, "/", nil)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "no-cache", res.Header.Get("Cache-Control"))
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
		require.Equal(t, "no-cache", res.Header.Get("Cache-Control"))
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
		res := serve(t, handler, http.MethodGet, "/trackers/abc", nil)
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, etag, res.Header.Get("ETag"))
		require.Equal(t, "no-cache", res.Header.Get("Cache-Control"))
		require.Equal(t, testIndexHTML, drain(t, res))
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

		handler, err := newFrontendHandler(fstest.MapFS{
			"index.html": {Data: []byte(index)},
		})
		require.NoError(t, err)

		res := serve(t, handler, http.MethodGet, "/", nil)
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

func TestNewFrontendHandler_IndexMissing(t *testing.T) {
	_, err := newFrontendHandler(fstest.MapFS{
		"assets/index-ABCDEFGH.js": {Data: []byte("console.log(1)")},
	})
	require.Error(t, err)
}

func TestHandler_StaticCaching(t *testing.T) {
	handler := newTestFrontendHandler(t)
	server := NewMoraServerBuilder(t).WithFrontendFileServer(handler).Finish()
	router := server.Handler()

	get := func(target string) *http.Response {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))

		return w.Result()
	}

	t.Run("assets are immutable and skip the session", func(t *testing.T) {
		res := get(testAssetPath)
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, testImmutableValue, res.Header.Get("Cache-Control"))
		require.Empty(t, res.Cookies())
	})

	t.Run("entry point revalidates and skips the session", func(t *testing.T) {
		res := get("/")
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, "no-cache", res.Header.Get("Cache-Control"))
		require.NotEmpty(t, res.Header.Get("ETag"))
		require.Empty(t, res.Cookies())
	})

	t.Run("SPA fallback serves the entry point", func(t *testing.T) {
		res := get("/trackers/abc")
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, "no-cache", res.Header.Get("Cache-Control"))
		require.NotEmpty(t, res.Header.Get("ETag"))
		require.Equal(t, testIndexHTML, drain(t, res))
	})

	t.Run("missing asset stays a 404", func(t *testing.T) {
		res := get("/assets/missing-ABCDEFGH.js")
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusNotFound, res.StatusCode)
		require.Empty(t, res.Header.Get("Cache-Control"))
	})
}
