package track

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.Disabled)
	}

	os.Exit(m.Run())
}

func newTestHandler(t *testing.T) http.Handler {
	return newHandler(initTestStore(t), "")
}

func newTestHandlerWithAPIKey(t *testing.T, apiKey string) http.Handler {
	return newHandler(initTestStore(t), apiKey)
}

func superuserCtx() context.Context {
	var uid int64 = 1
	return ContextWithAuth(context.Background(), &uid)
}

func unmarshalResponse(t *testing.T, r *http.Response, data any) {
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)

	err = json.Unmarshal(body, data)
	require.NoError(t, err)
}

func newRequestWithJSON(t *testing.T, method, path string, data any) *http.Request {
	body, err := json.Marshal(data)
	require.NoError(t, err)

	return httptest.NewRequest(method, path, bytes.NewBuffer(body))
}

func getResponse(t *testing.T, expectedStatus int, h http.Handler, r *http.Request) *http.Response {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	res := w.Result()

	if expectedStatus != res.StatusCode {
		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		t.Log(string(body))
	}

	require.Equal(t, expectedStatus, res.StatusCode)

	return res
}

// ----------------------------------------------------------------------
// Auth

func TestRequireAuth(t *testing.T) {
	t.Run("no api key configured allows all", func(t *testing.T) {
		h := newTestHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		getResponse(t, http.StatusOK, h, r)
	})

	t.Run("valid api key", func(t *testing.T) {
		h := newTestHandlerWithAPIKey(t, "secret123")
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", "Bearer secret123")
		getResponse(t, http.StatusOK, h, r)
	})

	t.Run("invalid api key", func(t *testing.T) {
		h := newTestHandlerWithAPIKey(t, "secret123")
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", "Bearer wrongkey")
		getResponse(t, http.StatusUnauthorized, h, r)
	})

	t.Run("missing authorization header", func(t *testing.T) {
		h := newTestHandlerWithAPIKey(t, "secret123")
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		getResponse(t, http.StatusUnauthorized, h, r)
	})

	t.Run("session auth bypasses API key check", func(t *testing.T) {
		h := newTestHandlerWithAPIKey(t, "secret123")
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusOK, h, r)
	})

	t.Run("session auth works without API key", func(t *testing.T) {
		h := newTestHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusOK, h, r)
	})
}

// ----------------------------------------------------------------------
// Track

func TestHandlerCreateTrack(t *testing.T) {
	t.Run("valid name with auth", func(t *testing.T) {
		h := newTestHandler(t)
		r := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackRequest{Name: "test_track"})
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusCreated, h, r)

		var got TrackModel
		unmarshalResponse(t, res, &got)
		require.Equal(t, int64(1), got.Id)
		require.Equal(t, "test_track", got.Name)
	})

	t.Run("duplicate name", func(t *testing.T) {
		h := newTestHandler(t)
		r1 := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackRequest{Name: "dup"})
		r1 = r1.WithContext(superuserCtx())
		getResponse(t, http.StatusCreated, h, r1)

		r2 := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackRequest{Name: "dup"})
		r2 = r2.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r2)
	})

	t.Run("forbidden without auth", func(t *testing.T) {
		h := newTestHandler(t)
		r := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackRequest{Name: "no_auth"})
		// no auth context → anonymous user
		getResponse(t, http.StatusForbidden, h, r)
	})
}

func TestHandlerListTracks(t *testing.T) {
	t.Run("empty for authenticated user", func(t *testing.T) {
		h := newTestHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListTracksResponse
		unmarshalResponse(t, res, &got)
		require.Empty(t, got.Tracks)
	})

	t.Run("with tracks", func(t *testing.T) {
		store := initTestStore(t)
		tr1 := &TrackModel{Name: "track1"}
		tr2 := &TrackModel{Name: "track2"}
		require.NoError(t, store.addTrack(tr1, 1))
		require.NoError(t, store.addTrack(tr2, 1))

		h := newHandler(store, "")
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListTracksResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, 2, len(got.Tracks))
	})

	t.Run("returns empty for anonymous", func(t *testing.T) {
		h := newTestHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		res := getResponse(t, http.StatusOK, h, r)

		var got ListTracksResponse
		unmarshalResponse(t, res, &got)
		require.Empty(t, got.Tracks)
	})
}

func TestHandlerDeleteTrack(t *testing.T) {
	t.Run("delete existing track as member", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d", tr.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNoContent, h, r)
	})

	t.Run("delete forbidden without edit permission", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d", tr.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		// anonymous user, no context set → requireAuth sets ContextWithAuth(ctx, nil)
		r = r.WithContext(ContextWithAuth(context.Background(), nil))
		getResponse(t, http.StatusForbidden, h, r)
	})

	t.Run("delete with invalid URL", func(t *testing.T) {
		store := initTestStore(t)
		h := newHandler(store, "")
		r := httptest.NewRequest(http.MethodDelete, "/foo", nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("delete non existing track", func(t *testing.T) {
		store := initTestStore(t)
		h := newHandler(store, "")
		r := httptest.NewRequest(http.MethodDelete, "/99999", nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNotFound, h, r)
	})
}

func TestHandlerPatchTrack(t *testing.T) {
	t.Run("owner changes visibility to public", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d", tr.Id)
		body := PatchTrackRequest{Visibility: "public"}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got TrackResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, "public", got.Visibility)
		require.Equal(t, "owner", got.Role)
	})

	t.Run("non-member cannot change visibility", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d", tr.Id)
		body := PatchTrackRequest{Visibility: "public"}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		var uid int64 = 2
		r = r.WithContext(ContextWithAuth(context.Background(), &uid))
		getResponse(t, http.StatusForbidden, h, r)
	})

	t.Run("invalid visibility value", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d", tr.Id)
		type badBody struct {
			Visibility string `json:"visibility"`
		}
		r := newRequestWithJSON(t, http.MethodPatch, path, badBody{Visibility: "invalid"})
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})
}

func TestHandlerRequireReadPermission(t *testing.T) {
	t.Run("public track accessible by anonymous", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test", Visibility: "public"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		getResponse(t, http.StatusOK, h, r)
	})

	t.Run("unlisted track accessible by anonymous", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test", Visibility: "unlisted"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		getResponse(t, http.StatusOK, h, r)
	})

	t.Run("private track blocked for anonymous", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test", Visibility: "private"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		getResponse(t, http.StatusForbidden, h, r)
	})

	t.Run("private track accessible by member", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test", Visibility: "private"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusOK, h, r)
	})
}

func TestHandlerInjectTrackDBError(t *testing.T) {
	store := initTestStore(t)
	tr := &TrackModel{Name: "test_track"}
	require.NoError(t, store.addTrack(tr, 1))

	require.NoError(t, store.db.Close())

	h := newHandler(store, "")
	path := fmt.Sprintf("/%d", tr.Id)
	r := httptest.NewRequest(http.MethodDelete, path, nil)
	r = r.WithContext(superuserCtx())
	w := httptest.NewRecorder()

	require.NotPanics(t, func() {
		h.ServeHTTP(w, r)
	})

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusInternalServerError, res.StatusCode)
}

// ----------------------------------------------------------------------
// Series

func TestHandlerCreateSeries(t *testing.T) {
	t.Run("success with data_type float", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := newRequestWithJSON(t, http.MethodPost, path, CreateSeriesRequest{Name: "s1", DataType: "float"})
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusCreated, h, r)

		var got SeriesModel
		unmarshalResponse(t, res, &got)
		require.Equal(t, int64(1), got.Id)
		require.Equal(t, "float", got.DataType)
	})

	t.Run("success with data_type int", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := newRequestWithJSON(t, http.MethodPost, path, CreateSeriesRequest{Name: "s2", DataType: "int"})
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusCreated, h, r)

		var got SeriesModel
		unmarshalResponse(t, res, &got)
		require.Equal(t, "int", got.DataType)
	})

	t.Run("default data_type is float", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := newRequestWithJSON(t, http.MethodPost, path, CreateSeriesRequest{Name: "s3"})
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusCreated, h, r)

		var got SeriesModel
		unmarshalResponse(t, res, &got)
		require.Equal(t, "float", got.DataType)
	})

	t.Run("invalid data_type", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := newRequestWithJSON(t, http.MethodPost, path, CreateSeriesRequest{Name: "s4", DataType: "binary"})
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("non existing track", func(t *testing.T) {
		store := initTestStore(t)
		h := newHandler(store, "")
		r := newRequestWithJSON(t, http.MethodPost, "/99999/series", CreateSeriesRequest{Name: "s1", DataType: "float"})
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("forbidden without auth", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := newRequestWithJSON(t, http.MethodPost, path, CreateSeriesRequest{Name: "s1", DataType: "float"})
		// anonymous
		getResponse(t, http.StatusForbidden, h, r)
	})
}

func TestHandlerListSeries(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListSeriesResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, tr.Id, got.Track.Id)
		require.Empty(t, got.Series)
	})

	t.Run("with series", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		s1 := &SeriesModel{TrackId: tr.Id, Name: "series1", DataType: "float"}
		require.NoError(t, store.addSeries(s1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListSeriesResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, 1, len(got.Series))
		require.Equal(t, s1.Id, got.Series[0].Id)
	})
}

func TestHandlerDeleteSeries(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		s := &SeriesModel{TrackId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series/%d", tr.Id, s.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNoContent, h, r)
	})

	t.Run("non existing series", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series/99999", tr.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("series from different track", func(t *testing.T) {
		store := initTestStore(t)
		tr1 := &TrackModel{Name: "track1"}
		tr2 := &TrackModel{Name: "track2"}
		require.NoError(t, store.addTrack(tr1, 1))
		require.NoError(t, store.addTrack(tr2, 1))

		s := &SeriesModel{TrackId: tr2.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series/%d", tr1.Id, s.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNotFound, h, r)
	})
}

// ----------------------------------------------------------------------
// Values

func TestHandlerCreateValue(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))
		s := &SeriesModel{TrackId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series/%d/values", tr.Id, s.Id)
		now := time.Now().Round(0)
		r := newRequestWithJSON(t, http.MethodPost, path, CreateValueRequest{Timestamp: now, Value: 42.5})
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusCreated, h, r)

		var got ValueModel
		unmarshalResponse(t, res, &got)
		require.Equal(t, int64(1), got.Id)
		require.Equal(t, 42.5, got.Value)
	})

	t.Run("duplicate time", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))
		s := &SeriesModel{TrackId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series/%d/values", tr.Id, s.Id)
		now := time.Now().Round(0)
		r1 := newRequestWithJSON(t, http.MethodPost, path, CreateValueRequest{Timestamp: now, Value: 42.5})
		r1 = r1.WithContext(superuserCtx())
		getResponse(t, http.StatusCreated, h, r1)

		r2 := newRequestWithJSON(t, http.MethodPost, path, CreateValueRequest{Timestamp: now, Value: 99.9})
		r2 = r2.WithContext(superuserCtx())
		getResponse(t, http.StatusInternalServerError, h, r2)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))
		s := &SeriesModel{TrackId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series/%d/values", tr.Id, s.Id)
		r := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString("{invalid}"))
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("forbidden without auth", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))
		s := &SeriesModel{TrackId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series/%d/values", tr.Id, s.Id)
		now := time.Now().Round(0)
		r := newRequestWithJSON(t, http.MethodPost, path, CreateValueRequest{Timestamp: now, Value: 42.5})
		// anonymous
		getResponse(t, http.StatusForbidden, h, r)
	})
}

func TestHandlerListValues(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))
		s := &SeriesModel{TrackId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series/%d/values", tr.Id, s.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListValuesResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, s.Id, got.Series.Id)
		require.Empty(t, got.Values)
	})

	t.Run("with values", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))
		s := &SeriesModel{TrackId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		now := time.Now().Round(0)
		v1 := &ValueModel{SeriesId: s.Id, Timestamp: now, Value: 10.0}
		v2 := &ValueModel{SeriesId: s.Id, Timestamp: now.Add(time.Hour), Value: 20.0}
		require.NoError(t, store.addValue(v1))
		require.NoError(t, store.addValue(v2))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series/%d/values", tr.Id, s.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListValuesResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, 2, len(got.Values))
	})

	t.Run("with limit", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))
		s := &SeriesModel{TrackId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		now := time.Now().Round(0)
		for i := 0; i < 5; i++ {
			v := &ValueModel{SeriesId: s.Id, Timestamp: now.Add(time.Duration(i) * time.Hour), Value: float64(i)}
			require.NoError(t, store.addValue(v))
		}

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series/%d/values?limit=3", tr.Id, s.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListValuesResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, 3, len(got.Values))
	})

	t.Run("invalid limit", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))
		s := &SeriesModel{TrackId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/series/%d/values?limit=abc", tr.Id, s.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})
}

func TestHandlerDeleteValues(t *testing.T) {
	store := initTestStore(t)
	tr := &TrackModel{Name: "test"}
	require.NoError(t, store.addTrack(tr, 1))
	s := &SeriesModel{TrackId: tr.Id, Name: "s1", DataType: "float"}
	require.NoError(t, store.addSeries(s))

	now := time.Now().Round(0)
	v1 := &ValueModel{SeriesId: s.Id, Timestamp: now, Value: 10.0}
	v2 := &ValueModel{SeriesId: s.Id, Timestamp: now.Add(time.Hour), Value: 20.0}
	require.NoError(t, store.addValue(v1))
	require.NoError(t, store.addValue(v2))

	h := newHandler(store, "")
	path := fmt.Sprintf("/%d/series/%d/values", tr.Id, s.Id)
	r := httptest.NewRequest(http.MethodDelete, path, nil)
	r = r.WithContext(superuserCtx())
	getResponse(t, http.StatusNoContent, h, r)
}

// ----------------------------------------------------------------------
// Like

func TestHandlerLike(t *testing.T) {
	t.Run("like and unlike track", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/like", tr.Id)

		// Like
		r := httptest.NewRequest(http.MethodPost, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusCreated, h, r)

		// Verify liked in list
		listR := httptest.NewRequest(http.MethodGet, "/", nil)
		listR = listR.WithContext(superuserCtx())
		listRes := getResponse(t, http.StatusOK, h, listR)
		var list ListTracksResponse
		unmarshalResponse(t, listRes, &list)
		require.Len(t, list.Tracks, 1)
		require.True(t, list.Tracks[0].Liked)

		// Unlike
		r2 := httptest.NewRequest(http.MethodDelete, path, nil)
		r2 = r2.WithContext(superuserCtx())
		getResponse(t, http.StatusNoContent, h, r2)

		// Verify no longer liked
		listR2 := httptest.NewRequest(http.MethodGet, "/", nil)
		listR2 = listR2.WithContext(superuserCtx())
		listRes2 := getResponse(t, http.StatusOK, h, listR2)
		var list2 ListTracksResponse
		unmarshalResponse(t, listRes2, &list2)
		require.False(t, list2.Tracks[0].Liked)
	})

	t.Run("like requires auth", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/like", tr.Id)
		r := httptest.NewRequest(http.MethodPost, path, nil)
		getResponse(t, http.StatusForbidden, h, r)
	})

	t.Run("unlike requires auth", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/like", tr.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		getResponse(t, http.StatusForbidden, h, r)
	})

	t.Run("like non-existing track", func(t *testing.T) {
		h := newTestHandler(t)
		r := httptest.NewRequest(http.MethodPost, "/99999/like", nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("duplicate like is idempotent", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackModel{Name: "test"}
		require.NoError(t, store.addTrack(tr, 1))

		h := newHandler(store, "")
		path := fmt.Sprintf("/%d/like", tr.Id)

		r1 := httptest.NewRequest(http.MethodPost, path, nil)
		r1 = r1.WithContext(superuserCtx())
		getResponse(t, http.StatusCreated, h, r1)

		r2 := httptest.NewRequest(http.MethodPost, path, nil)
		r2 = r2.WithContext(superuserCtx())
		getResponse(t, http.StatusCreated, h, r2)
	})
}
