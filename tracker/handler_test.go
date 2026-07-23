package tracker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	return newHandler(initTestStore(t), nil)
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

func strPtr(s string) *string { return &s }

// ----------------------------------------------------------------------
// Auth

func TestRequireAuth(t *testing.T) {
	t.Run("allows all (auth is delegated to server middleware)", func(t *testing.T) {
		h := newTestHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		getResponse(t, http.StatusOK, h, r)
	})

	t.Run("respects context auth", func(t *testing.T) {
		h := newTestHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusOK, h, r)
	})
}

// ----------------------------------------------------------------------
// Tracker

func TestHandlerCreateTracker(t *testing.T) {
	t.Run("valid name with auth", func(t *testing.T) {
		h := newTestHandler(t)
		r := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackerRequest{Name: "test_tracker", Visibility: "private"})
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusCreated, h, r)

		var got TrackerModel
		unmarshalResponse(t, res, &got)
		require.Equal(t, int64(1), got.Id)
		require.Equal(t, "test_tracker", got.Name)
	})

	t.Run("duplicate name", func(t *testing.T) {
		h := newTestHandler(t)
		r1 := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackerRequest{Name: "dup", Visibility: "private"})
		r1 = r1.WithContext(superuserCtx())
		getResponse(t, http.StatusCreated, h, r1)

		r2 := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackerRequest{Name: "dup", Visibility: "private"})
		r2 = r2.WithContext(superuserCtx())
		getResponse(t, http.StatusCreated, h, r2)
	})

	t.Run("forbidden without auth", func(t *testing.T) {
		h := newTestHandler(t)
		r := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackerRequest{Name: "no_auth", Visibility: "private"})
		// no auth context -> anonymous user
		getResponse(t, http.StatusForbidden, h, r)
	})

	t.Run("missing visibility", func(t *testing.T) {
		h := newTestHandler(t)
		r := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackerRequest{Name: "test", Visibility: ""})
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("invalid visibility", func(t *testing.T) {
		h := newTestHandler(t)
		r := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackerRequest{Name: "test", Visibility: "invalid"})
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("with chart_config", func(t *testing.T) {
		h := newTestHandler(t)
		cc := `{"x_axis_label":"Time"}`
		req := CreateTrackerRequest{Name: "cfg_tracker", Visibility: "private", ChartConfig: &cc}
		r := newRequestWithJSON(t, http.MethodPost, "/", req)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusCreated, h, r)

		var got TrackerModel
		unmarshalResponse(t, res, &got)
		require.Equal(t, cc, got.ChartConfig)
	})

	t.Run("invalid chart_config JSON", func(t *testing.T) {
		h := newTestHandler(t)
		bad := "not-json"
		req := CreateTrackerRequest{Name: "bad_cfg", Visibility: "private", ChartConfig: &bad}
		r := newRequestWithJSON(t, http.MethodPost, "/", req)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("chart_config with 3+ y_axes rejected", func(t *testing.T) {
		h := newTestHandler(t)
		cc := `{"y_axes":[{"id":0,"position":"left"},{"id":1,"position":"right"},{"id":2,"position":"left"}]}`
		req := CreateTrackerRequest{Name: "too_many_axes", Visibility: "private", ChartConfig: &cc}
		r := newRequestWithJSON(t, http.MethodPost, "/", req)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("chart_config with empty y_axes rejected", func(t *testing.T) {
		h := newTestHandler(t)
		cc := `{"y_axes":[]}`
		req := CreateTrackerRequest{Name: "empty_axes", Visibility: "private", ChartConfig: &cc}
		r := newRequestWithJSON(t, http.MethodPost, "/", req)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})
}

func TestHandlerListTrackers(t *testing.T) {
	t.Run("empty for authenticated user", func(t *testing.T) {
		h := newTestHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListTrackersResponse
		unmarshalResponse(t, res, &got)
		require.Empty(t, got.Trackers)
		require.Equal(t, 0, got.Total)
		require.Equal(t, 1, got.Page)
		require.Equal(t, 0, got.PerPage)
	})

	t.Run("with trackers", func(t *testing.T) {
		store := initTestStore(t)
		tr1 := &TrackerModel{Name: "tracker1"}
		tr2 := &TrackerModel{Name: "tracker2"}
		require.NoError(t, store.addTracker(tr1, 1, nil))
		require.NoError(t, store.addTracker(tr2, 1, nil))

		h := newHandler(store, nil)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListTrackersResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, 2, len(got.Trackers))
		require.Equal(t, 2, got.Total)
	})

	t.Run("with pagination", func(t *testing.T) {
		store := initTestStore(t)
		for i := 0; i < 5; i++ {
			tr := &TrackerModel{Name: fmt.Sprintf("tracker_%d", i)}
			require.NoError(t, store.addTracker(tr, 1, nil))
		}

		h := newHandler(store, nil)
		r := httptest.NewRequest(http.MethodGet, "/?page=1&per_page=2", nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListTrackersResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, 2, len(got.Trackers))
		require.Equal(t, 5, got.Total)
		require.Equal(t, 1, got.Page)
		require.Equal(t, 2, got.PerPage)
	})

	t.Run("returns empty for anonymous", func(t *testing.T) {
		h := newTestHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		res := getResponse(t, http.StatusOK, h, r)

		var got ListTrackersResponse
		unmarshalResponse(t, res, &got)
		require.Empty(t, got.Trackers)
		require.Equal(t, 0, got.Total)
	})
}

func TestHandlerListTrackersSearch(t *testing.T) {
	t.Run("anonymous with query searches public only", func(t *testing.T) {
		store := initTestStore(t)
		tr1 := &TrackerModel{Name: "alpha", Visibility: "private"}
		tr2 := &TrackerModel{Name: "alpha_public", Visibility: "public"}
		tr3 := &TrackerModel{Name: "beta_public", Visibility: "public"}
		require.NoError(t, store.addTracker(tr1, 1, nil))
		require.NoError(t, store.addTracker(tr2, 2, nil))
		require.NoError(t, store.addTracker(tr3, 2, nil))

		h := newHandler(store, nil)
		r := httptest.NewRequest(http.MethodGet, "/?q=alpha", nil)
		res := getResponse(t, http.StatusOK, h, r)

		var got ListTrackersResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, 1, len(got.Trackers))
		require.Equal(t, tr2.Id, got.Trackers[0].Id)
	})

	t.Run("anonymous without query returns empty", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test", Visibility: "public"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		res := getResponse(t, http.StatusOK, h, r)

		var got ListTrackersResponse
		unmarshalResponse(t, res, &got)
		require.Empty(t, got.Trackers)
	})

	t.Run("logged in with query searches user and public", func(t *testing.T) {
		store := initTestStore(t)
		tr1 := &TrackerModel{Name: "my_tracker", Visibility: "private"}
		tr2 := &TrackerModel{Name: "public_tracker", Visibility: "public"}
		tr3 := &TrackerModel{Name: "other_public", Visibility: "public"}
		require.NoError(t, store.addTracker(tr1, 1, nil))
		require.NoError(t, store.addTracker(tr2, 2, nil))
		require.NoError(t, store.addTracker(tr3, 2, nil))

		h := newHandler(store, nil)
		r := httptest.NewRequest(http.MethodGet, "/?q=tracker", nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListTrackersResponse
		unmarshalResponse(t, res, &got)
		// "my_tracker" matches (user 1's), "public_tracker" matches (public)
		// "other_public" does not match "tracker"
		require.Equal(t, 2, len(got.Trackers))
	})

	t.Run("logged in without query returns user's trackers", func(t *testing.T) {
		store := initTestStore(t)
		tr1 := &TrackerModel{Name: "my_tracker"}
		tr2 := &TrackerModel{Name: "other_tracker"}
		require.NoError(t, store.addTracker(tr1, 1, nil))
		require.NoError(t, store.addTracker(tr2, 2, nil))

		h := newHandler(store, nil)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListTrackersResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, 1, len(got.Trackers))
		require.Equal(t, tr1.Id, got.Trackers[0].Id)
	})
}

func TestHandlerDeleteTracker(t *testing.T) {
	t.Run("delete existing tracker as member", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d", tr.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNoContent, h, r)
	})

	t.Run("delete forbidden without edit permission", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d", tr.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		// anonymous user, no context set -> requireAuth sets ContextWithAuth(ctx, nil)
		r = r.WithContext(ContextWithAuth(context.Background(), nil))
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("delete with invalid URL", func(t *testing.T) {
		store := initTestStore(t)
		h := newHandler(store, nil)
		r := httptest.NewRequest(http.MethodDelete, "/foo", nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("delete non existing tracker", func(t *testing.T) {
		store := initTestStore(t)
		h := newHandler(store, nil)
		r := httptest.NewRequest(http.MethodDelete, "/99999", nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNotFound, h, r)
	})
}

func TestHandlerPatchTracker(t *testing.T) {
	t.Run("owner changes visibility to public", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d", tr.Id)
		body := PatchTrackerRequest{Visibility: strPtr("public")}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got TrackerResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, "public", got.Visibility)
		require.Equal(t, "owner", got.Role)
	})

	t.Run("non-member cannot change visibility", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d", tr.Id)
		body := PatchTrackerRequest{Visibility: strPtr("public")}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		var uid int64 = 2
		r = r.WithContext(ContextWithAuth(context.Background(), &uid))
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("invalid visibility value", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d", tr.Id)
		r := newRequestWithJSON(t, http.MethodPatch, path, PatchTrackerRequest{Visibility: strPtr("invalid")})
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("patch chart_config", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d", tr.Id)
		cc := `{"x_axis_label":"Time","y_axis_label":"Value"}`
		body := PatchTrackerRequest{ChartConfig: strPtr(cc)}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got TrackerResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, cc, got.ChartConfig)
	})

	t.Run("patch chart_config invalid JSON", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d", tr.Id)
		body := PatchTrackerRequest{ChartConfig: strPtr("not-json")}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("patch chart_config with 3+ y_axes rejected", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d", tr.Id)
		cc := `{"y_axes":[{"id":0,"position":"left"},{"id":1,"position":"right"},{"id":2,"position":"left"}]}`
		body := PatchTrackerRequest{ChartConfig: strPtr(cc)}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("patch chart_config with empty y_axes rejected", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d", tr.Id)
		cc := `{"y_axes":[]}`
		body := PatchTrackerRequest{ChartConfig: strPtr(cc)}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})
}

func TestHandlerRequireReadPermission(t *testing.T) {
	t.Run("public tracker accessible by anonymous", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test", Visibility: "public"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		getResponse(t, http.StatusOK, h, r)
	})



	t.Run("private tracker blocked for anonymous", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test", Visibility: "private"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("private tracker accessible by member", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test", Visibility: "private"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusOK, h, r)
	})
}

func TestHandlerInjectTrackerDBError(t *testing.T) {
	store := initTestStore(t)
	tr := &TrackerModel{Name: "test_tracker"}
	require.NoError(t, store.addTracker(tr, 1, nil))

	require.NoError(t, store.db.Close())

	h := newHandler(store, nil)
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
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
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
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
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
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
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
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := newRequestWithJSON(t, http.MethodPost, path, CreateSeriesRequest{Name: "s4", DataType: "binary"})
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("non existing tracker", func(t *testing.T) {
		store := initTestStore(t)
		h := newHandler(store, nil)
		r := newRequestWithJSON(t, http.MethodPost, "/99999/series", CreateSeriesRequest{Name: "s1", DataType: "float"})
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("forbidden without auth", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := newRequestWithJSON(t, http.MethodPost, path, CreateSeriesRequest{Name: "s1", DataType: "float"})
		// anonymous
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("y_axis_index out of range rejected", func(t *testing.T) {
		store := initTestStore(t)
		cc := `{"y_axes":[{"id":0,"position":"left"},{"id":1,"position":"right"}]}`
		tr := &TrackerModel{Name: "test", ChartConfig: cc}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series", tr.Id)
		cfg := `{"y_axis_index":2}`
		r := newRequestWithJSON(t, http.MethodPost, path, CreateSeriesRequest{Name: "s1", Config: &cfg})
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})
}

func TestHandlerListSeries(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got ListSeriesResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, tr.Id, got.Tracker.Id)
		require.Empty(t, got.Series)
	})

	t.Run("with series", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		s1 := &SeriesModel{TrackerId: tr.Id, Name: "series1", DataType: "float"}
		require.NoError(t, store.addSeries(s1))

		h := newHandler(store, nil)
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
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series/%d", tr.Id, s.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNoContent, h, r)
	})

	t.Run("non existing series", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series/99999", tr.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("series from different tracker", func(t *testing.T) {
		store := initTestStore(t)
		tr1 := &TrackerModel{Name: "tracker1"}
		tr2 := &TrackerModel{Name: "tracker2"}
		require.NoError(t, store.addTracker(tr1, 1, nil))
		require.NoError(t, store.addTracker(tr2, 1, nil))

		s := &SeriesModel{TrackerId: tr2.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series/%d", tr1.Id, s.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNotFound, h, r)
	})
}

func TestHandlerPatchSeries(t *testing.T) {
	t.Run("patch series config", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))
		require.Equal(t, "{}", s.Config)

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series/%d", tr.Id, s.Id)
		cfg := `{"value_format":"%.2f"}`
		body := PatchSeriesRequest{Config: strPtr(cfg)}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got SeriesModel
		unmarshalResponse(t, res, &got)
		require.Equal(t, cfg, got.Config)
	})

	t.Run("patch series data_type", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series/%d", tr.Id, s.Id)
		body := PatchSeriesRequest{DataType: strPtr("int")}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got SeriesModel
		unmarshalResponse(t, res, &got)
		require.Equal(t, "int", got.DataType)
	})

	t.Run("patch series name", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series/%d", tr.Id, s.Id)
		body := PatchSeriesRequest{Name: strPtr("renamed")}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got SeriesModel
		unmarshalResponse(t, res, &got)
		require.Equal(t, "renamed", got.Name)
	})

	t.Run("invalid JSON config rejected", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series/%d", tr.Id, s.Id)
		body := PatchSeriesRequest{Config: strPtr("not-json")}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("forbidden without edit permission", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series/%d", tr.Id, s.Id)
		body := PatchSeriesRequest{Config: strPtr(`{"value_format":"%.1f"}`)}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		r = r.WithContext(ContextWithAuth(context.Background(), nil))
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("patch series y_axis_index out of range rejected", func(t *testing.T) {
		store := initTestStore(t)
		cc := `{"y_axes":[{"id":0,"position":"left"}]}`
		tr := &TrackerModel{Name: "test", ChartConfig: cc}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series/%d", tr.Id, s.Id)
		cfg := `{"y_axis_index":1}`
		body := PatchSeriesRequest{Config: strPtr(cfg)}
		r := newRequestWithJSON(t, http.MethodPatch, path, body)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})
}

// ----------------------------------------------------------------------
// Values

func TestHandlerCreateValue(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
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
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
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
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series/%d/values", tr.Id, s.Id)
		r := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString("{invalid}"))
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("forbidden without auth", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series/%d/values", tr.Id, s.Id)
		now := time.Now().Round(0)
		r := newRequestWithJSON(t, http.MethodPost, path, CreateValueRequest{Timestamp: now, Value: 42.5})
		// anonymous
		getResponse(t, http.StatusNotFound, h, r)
	})
}

func TestHandlerListValues(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
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
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		now := time.Now().Round(0)
		v1 := &ValueModel{SeriesId: s.Id, Timestamp: now, Value: 10.0}
		v2 := &ValueModel{SeriesId: s.Id, Timestamp: now.Add(time.Hour), Value: 20.0}
		require.NoError(t, store.addValue(v1))
		require.NoError(t, store.addValue(v2))

		h := newHandler(store, nil)
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
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		now := time.Now().Round(0)
		for i := 0; i < 5; i++ {
			v := &ValueModel{SeriesId: s.Id, Timestamp: now.Add(time.Duration(i) * time.Hour), Value: float64(i)}
			require.NoError(t, store.addValue(v))
		}

		h := newHandler(store, nil)
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
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))
		s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/series/%d/values?limit=abc", tr.Id, s.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})
}

func TestHandlerDeleteValues(t *testing.T) {
	store := initTestStore(t)
	tr := &TrackerModel{Name: "test"}
	require.NoError(t, store.addTracker(tr, 1, nil))
	s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
	require.NoError(t, store.addSeries(s))

	now := time.Now().Round(0)
	v1 := &ValueModel{SeriesId: s.Id, Timestamp: now, Value: 10.0}
	v2 := &ValueModel{SeriesId: s.Id, Timestamp: now.Add(time.Hour), Value: 20.0}
	require.NoError(t, store.addValue(v1))
	require.NoError(t, store.addValue(v2))

	h := newHandler(store, nil)
	path := fmt.Sprintf("/%d/series/%d/values", tr.Id, s.Id)
	r := httptest.NewRequest(http.MethodDelete, path, nil)
	r = r.WithContext(superuserCtx())
	getResponse(t, http.StatusNoContent, h, r)
}

// ----------------------------------------------------------------------
// Like

func TestHandlerLike(t *testing.T) {
	t.Run("like and unlike tracker", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/like", tr.Id)

		// Like
		r := httptest.NewRequest(http.MethodPost, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusCreated, h, r)

		// Verify liked in list
		listR := httptest.NewRequest(http.MethodGet, "/", nil)
		listR = listR.WithContext(superuserCtx())
		listRes := getResponse(t, http.StatusOK, h, listR)
		var list ListTrackersResponse
		unmarshalResponse(t, listRes, &list)
		require.Len(t, list.Trackers, 1)
		require.True(t, list.Trackers[0].Liked)

		// Unlike
		r2 := httptest.NewRequest(http.MethodDelete, path, nil)
		r2 = r2.WithContext(superuserCtx())
		getResponse(t, http.StatusNoContent, h, r2)

		// Verify no longer liked
		listR2 := httptest.NewRequest(http.MethodGet, "/", nil)
		listR2 = listR2.WithContext(superuserCtx())
		listRes2 := getResponse(t, http.StatusOK, h, listR2)
		var list2 ListTrackersResponse
		unmarshalResponse(t, listRes2, &list2)
		require.False(t, list2.Trackers[0].Liked)
	})

	t.Run("like requires auth", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/like", tr.Id)
		r := httptest.NewRequest(http.MethodPost, path, nil)
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("unlike requires auth", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/like", tr.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("like non-existing tracker", func(t *testing.T) {
		h := newTestHandler(t)
		r := httptest.NewRequest(http.MethodPost, "/99999/like", nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNotFound, h, r)
	})

	t.Run("duplicate like is idempotent", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/like", tr.Id)

		r1 := httptest.NewRequest(http.MethodPost, path, nil)
		r1 = r1.WithContext(superuserCtx())
		getResponse(t, http.StatusCreated, h, r1)

		r2 := httptest.NewRequest(http.MethodPost, path, nil)
		r2 = r2.WithContext(superuserCtx())
		getResponse(t, http.StatusCreated, h, r2)
	})
}

// ----------------------------------------------------------------------
// Preview

func TestHandlerPreviewTracker(t *testing.T) {
	t.Run("returns preview with latest values", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "test"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		s1 := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
		require.NoError(t, store.addSeries(s1))

		now := time.Now().Round(0)
		for i := 0; i < 5; i++ {
			v := &ValueModel{SeriesId: s1.Id, Timestamp: now.Add(time.Duration(i) * time.Hour), Value: float64(i)}
			require.NoError(t, store.addValue(v))
		}

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/preview", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got PreviewResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, tr.Id, got.Tracker.Id)
		require.Equal(t, "owner", got.Tracker.Role)
		require.Len(t, got.Series, 1)
		require.Equal(t, s1.Id, got.Series[0].Series.Id)
		require.Len(t, got.Series[0].Values, 5)
		// Values should be in chronological order (latest first reversed to ASC)
		require.Equal(t, 0.0, got.Series[0].Values[0].Value)
		require.Equal(t, 4.0, got.Series[0].Values[4].Value)
	})

	t.Run("returns empty series for tracker without series", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "empty"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/preview", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got PreviewResponse
		unmarshalResponse(t, res, &got)
		require.Len(t, got.Series, 0)
	})

	t.Run("returns 404 for non-existing tracker", func(t *testing.T) {
		h := newTestHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/99999/preview", nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusNotFound, h, r)
	})
}

// ----------------------------------------------------------------------
// Coverage type rejection

func TestHandlerCreateTrackerCoverageType(t *testing.T) {
	t.Run("coverage type with repo_id", func(t *testing.T) {
		store := initTestStore(t)
		store.db.MustExec("INSERT INTO repository (id) VALUES (100)")

		h := newHandler(store, nil)
		repoID := int64(100)
		r := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackerRequest{
			Name:       "cov_tracker",
			Visibility: "public",
			Type:       "coverage",
			RepoID:     &repoID,
		})
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusCreated, h, r)

		var got TrackerResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, "coverage", got.Type)
		require.NotNil(t, got.RepoID)
		require.Equal(t, int64(100), *got.RepoID)
	})

	t.Run("coverage type without repo_id", func(t *testing.T) {
		h := newTestHandler(t)
		r := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackerRequest{
			Name:       "bad_cov",
			Visibility: "public",
			Type:       "coverage",
		})
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("tracker type with repo_id", func(t *testing.T) {
		h := newTestHandler(t)
		repoID := int64(1)
		r := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackerRequest{
			Name:       "bad_tracker",
			Visibility: "public",
			Type:       "tracker",
			RepoID:     &repoID,
		})
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})

	t.Run("invalid type", func(t *testing.T) {
		h := newTestHandler(t)
		r := newRequestWithJSON(t, http.MethodPost, "/", CreateTrackerRequest{
			Name:       "bad",
			Visibility: "public",
			Type:       "invalid",
		})
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusBadRequest, h, r)
	})
}

func TestHandlerCreateSeriesCoverageTracker(t *testing.T) {
	store := initTestStore(t)
	tr := &TrackerModel{Name: "cov", Type: "coverage"}
	require.NoError(t, store.addTracker(tr, 1, nil))

	h := newHandler(store, nil)
	path := fmt.Sprintf("/%d/series", tr.Id)
	r := newRequestWithJSON(t, http.MethodPost, path, CreateSeriesRequest{Name: "s1", DataType: "float"})
	r = r.WithContext(superuserCtx())
	getResponse(t, http.StatusBadRequest, h, r)
}

func TestHandlerDeleteSeriesCoverageTracker(t *testing.T) {
	store := initTestStore(t)
	tr := &TrackerModel{Name: "cov", Type: "coverage"}
	require.NoError(t, store.addTracker(tr, 1, nil))
	s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
	require.NoError(t, store.addSeries(s))

	h := newHandler(store, nil)
	path := fmt.Sprintf("/%d/series/%d", tr.Id, s.Id)
	r := httptest.NewRequest(http.MethodDelete, path, nil)
	r = r.WithContext(superuserCtx())
	getResponse(t, http.StatusBadRequest, h, r)
}

func TestHandlerCreateValueCoverageTracker(t *testing.T) {
	store := initTestStore(t)
	tr := &TrackerModel{Name: "cov", Type: "coverage"}
	require.NoError(t, store.addTracker(tr, 1, nil))
	s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
	require.NoError(t, store.addSeries(s))

	h := newHandler(store, nil)
	path := fmt.Sprintf("/%d/series/%d/values", tr.Id, s.Id)
	now := time.Now().Round(0)
	r := newRequestWithJSON(t, http.MethodPost, path, CreateValueRequest{Timestamp: now, Value: 1.0})
	r = r.WithContext(superuserCtx())
	getResponse(t, http.StatusBadRequest, h, r)
}

func TestHandlerDeleteValuesCoverageTracker(t *testing.T) {
	store := initTestStore(t)
	tr := &TrackerModel{Name: "cov", Type: "coverage"}
	require.NoError(t, store.addTracker(tr, 1, nil))
	s := &SeriesModel{TrackerId: tr.Id, Name: "s1", DataType: "float"}
	require.NoError(t, store.addSeries(s))

	h := newHandler(store, nil)
	path := fmt.Sprintf("/%d/series/%d/values", tr.Id, s.Id)
	r := httptest.NewRequest(http.MethodDelete, path, nil)
	r = r.WithContext(superuserCtx())
	getResponse(t, http.StatusBadRequest, h, r)
}

// ----------------------------------------------------------------------
// Preview coverage tracker

type mockCoverageProvider struct {
	timelineFn func(repoID int64, limit int) (map[string][]CoverageTimelinePoint, error)
}

func (m *mockCoverageProvider) Timeline(repoID int64, limit int) (map[string][]CoverageTimelinePoint, error) {
	return m.timelineFn(repoID, limit)
}

func TestHandlerPreviewCoverageTracker(t *testing.T) {
	t.Run("coverage tracker with timeline data", func(t *testing.T) {
		store := initTestStore(t)
		store.db.MustExec("INSERT INTO repository (id) VALUES (100)")
		repoID := int64(100)
		tr := &TrackerModel{Name: "cov", Type: "coverage"}
		require.NoError(t, store.addTracker(tr, 1, &repoID))

		now := time.Now().Round(0)
		provider := &mockCoverageProvider{
			timelineFn: func(repoID int64, limit int) (map[string][]CoverageTimelinePoint, error) {
				return map[string][]CoverageTimelinePoint{
					"overall": {
						{Time: now.Add(-2 * time.Hour), Value: 70.0},
						{Time: now.Add(-time.Hour), Value: 80.0},
						{Time: now, Value: 90.0},
					},
				}, nil
			},
		}

		h := newHandler(store, provider)
		path := fmt.Sprintf("/%d/preview", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got PreviewResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, tr.Id, got.Tracker.Id)
		require.Len(t, got.Series, 1)
		require.Equal(t, "overall", got.Series[0].Series.Name)
		require.Equal(t, `{"value_format":"%.1f%%"}`, got.Series[0].Series.Config)
	})

	t.Run("coverage tracker without repoID", func(t *testing.T) {
		store := initTestStore(t)
		tr := &TrackerModel{Name: "cov", Type: "coverage"}
		require.NoError(t, store.addTracker(tr, 1, nil))

		h := newHandler(store, nil)
		path := fmt.Sprintf("/%d/preview", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		res := getResponse(t, http.StatusOK, h, r)

		var got PreviewResponse
		unmarshalResponse(t, res, &got)
		require.Equal(t, tr.Id, got.Tracker.Id)
		require.Empty(t, got.Series)
	})

	t.Run("coverage tracker with provider error", func(t *testing.T) {
		store := initTestStore(t)
		store.db.MustExec("INSERT INTO repository (id) VALUES (200)")
		repoID := int64(200)
		tr := &TrackerModel{Name: "cov", Type: "coverage"}
		require.NoError(t, store.addTracker(tr, 1, &repoID))

		provider := &mockCoverageProvider{
			timelineFn: func(repoID int64, limit int) (map[string][]CoverageTimelinePoint, error) {
				return nil, errors.New("timeline error")
			},
		}

		h := newHandler(store, provider)
		path := fmt.Sprintf("/%d/preview", tr.Id)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(superuserCtx())
		getResponse(t, http.StatusInternalServerError, h, r)
	})
}
