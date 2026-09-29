package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/iszk1215/mora/coverage"
	"github.com/iszk1215/mora/tracker"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func setupTrackersListServer(t *testing.T) (*MoraServer, http.Handler, *sqlx.DB) {
	t.Helper()
	db := openTestDB(t)
	userStore := NewUserStore(db)
	require.NoError(t, userStore.Init())

	trackerService, err := tracker.NewService(db)
	require.NoError(t, err)

	coverageService, err := coverage.NewCoverageService(db)
	require.NoError(t, err)

	server := NewMoraServerBuilder(t).
		WithSessionManager().
		WithTracker(trackerService).
		WithCoverage(coverageService).
		WithUserStore(userStore).
		Finish()

	return server, server.Handler(), db
}

func authedListRequest(t *testing.T, server *MoraServer, path string, userID int64) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	sid := fmt.Sprintf("session-%d", userID)
	sess := NewMoraSession()
	sess.SetUserID(userID)
	require.NoError(t, server.sessionManager.store.Put(sid, sess))
	r.AddCookie(&http.Cookie{Name: "morasessionid", Value: sid})
	return r
}

func serveList(t *testing.T, handler http.Handler, req *http.Request) tracker.ListTrackersResponse {
	t.Helper()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)
	var got tracker.ListTrackersResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
	return got
}

func TestServerHandleTrackersList(t *testing.T) {
	server, handler, _ := setupTrackersListServer(t)

	alice, err := server.userStore.CreateUser("alice", "")
	require.NoError(t, err)

	_, err = server.tracker.CreateTracker("tracker1", "", "", "private", alice.ID, tracker.TypeTracker, "{}")
	require.NoError(t, err)
	_, err = server.tracker.CreateTracker("tracker2", "", "", "private", alice.ID, tracker.TypeTracker, "{}")
	require.NoError(t, err)

	carol, err := server.userStore.CreateUser("carol", "")
	require.NoError(t, err)

	t.Run("empty for authenticated user", func(t *testing.T) {
		got := serveList(t, handler, authedListRequest(t, server, "/api/trackers", carol.ID))
		require.Empty(t, got.Trackers)
		require.Equal(t, 0, got.Total)
		require.Equal(t, 1, got.Page)
		require.Equal(t, 0, got.PerPage)
	})

	t.Run("with trackers", func(t *testing.T) {
		got := serveList(t, handler, authedListRequest(t, server, "/api/trackers", alice.ID))
		require.Equal(t, 2, len(got.Trackers))
		require.Equal(t, 2, got.Total)
	})

	t.Run("with pagination", func(t *testing.T) {
		for i := 3; i < 6; i++ {
			_, err := server.tracker.CreateTracker(fmt.Sprintf("tracker%d", i), "", "", "private", alice.ID, tracker.TypeTracker, "{}")
			require.NoError(t, err)
		}
		got := serveList(t, handler, authedListRequest(t, server, "/api/trackers?page=1&per_page=2", alice.ID))
		require.Equal(t, 2, len(got.Trackers))
		require.Equal(t, 5, got.Total)
		require.Equal(t, 1, got.Page)
		require.Equal(t, 2, got.PerPage)
	})

	t.Run("returns empty for anonymous", func(t *testing.T) {
		got := serveList(t, handler, httptest.NewRequest(http.MethodGet, "/api/trackers", nil))
		require.Empty(t, got.Trackers)
		require.Equal(t, 0, got.Total)
	})
}

func TestServerHandleTrackersListSearch(t *testing.T) {
	server, handler, _ := setupTrackersListServer(t)

	alice, err := server.userStore.CreateUser("alice", "")
	require.NoError(t, err)
	bob, err := server.userStore.CreateUser("bob", "")
	require.NoError(t, err)

	_, err = server.tracker.CreateTracker("alpha", "", "", "private", alice.ID, tracker.TypeTracker, "{}")
	require.NoError(t, err)
	_, err = server.tracker.CreateTracker("alpha_public", "", "", "public", bob.ID, tracker.TypeTracker, "{}")
	require.NoError(t, err)
	_, err = server.tracker.CreateTracker("beta_public", "", "", "public", bob.ID, tracker.TypeTracker, "{}")
	require.NoError(t, err)

	t.Run("anonymous with query searches public only", func(t *testing.T) {
		got := serveList(t, handler, httptest.NewRequest(http.MethodGet, "/api/trackers?q=alpha", nil))
		require.Equal(t, 1, len(got.Trackers))
	})

	t.Run("anonymous without query returns empty", func(t *testing.T) {
		got := serveList(t, handler, httptest.NewRequest(http.MethodGet, "/api/trackers", nil))
		require.Empty(t, got.Trackers)
	})

	t.Run("logged in with query searches user and public", func(t *testing.T) {
		got := serveList(t, handler, authedListRequest(t, server, "/api/trackers?q=tracker", alice.ID))
		require.Equal(t, 0, len(got.Trackers))
		got = serveList(t, handler, authedListRequest(t, server, "/api/trackers?q=alpha", alice.ID))
		require.Equal(t, 2, len(got.Trackers))
	})

	t.Run("logged in without query returns user's trackers only", func(t *testing.T) {
		got := serveList(t, handler, authedListRequest(t, server, "/api/trackers", alice.ID))
		require.Equal(t, 1, len(got.Trackers))
	})
}

func TestServerHandleTrackersListPreview(t *testing.T) {
	server, handler, db := setupTrackersListServer(t)

	alice, err := server.userStore.CreateUser("alice", "")
	require.NoError(t, err)

	trackerTr, err := server.tracker.CreateTracker("tracker", "", "", "private", alice.ID, tracker.TypeTracker, "{}")
	require.NoError(t, err)

	covTr, err := server.tracker.CreateTracker("coverage", "", "", "private", alice.ID, tracker.TypeCoverage, "{}")
	require.NoError(t, err)

	db.MustExec("INSERT INTO tracker_series (tracker_id, name, data_type, config) VALUES (?, 'temp', 'float', '{}')", trackerTr.Id)
	var seriesID int64
	require.NoError(t, db.Get(&seriesID, "SELECT id FROM tracker_series WHERE tracker_id = ?", trackerTr.Id))
	now := time.Now().Round(0)
	db.MustExec("INSERT INTO tracker_value (series_id, time, value) VALUES (?, ?, ?)", seriesID, now, 12.5)

	store := server.coverage.Store()
	_, err = store.Put(&coverage.Coverage{
		TrackerID: covTr.Id,
		Revision:  "abcdef1234",
		Timestamp: now.Add(-time.Hour),
		Entries: []*coverage.CoverageEntry{
			{Name: "total", Hits: 8, Lines: 10},
		},
	})
	require.NoError(t, err)

	t.Run("without include omits series", func(t *testing.T) {
		got := serveList(t, handler, authedListRequest(t, server, "/api/trackers", alice.ID))
		require.Len(t, got.Trackers, 2)
		for _, tr := range got.Trackers {
			require.Nil(t, tr.Series)
		}
	})

	t.Run("include=preview embeds tracker and coverage series", func(t *testing.T) {
		got := serveList(t, handler, authedListRequest(t, server, "/api/trackers?include=preview", alice.ID))
		require.Len(t, got.Trackers, 2)

		var trackerSeries, coverageSeries []tracker.PreviewSeriesValues
		for _, tr := range got.Trackers {
			switch tr.Type {
			case tracker.TypeTracker:
				trackerSeries = tr.Series
			case tracker.TypeCoverage:
				coverageSeries = tr.Series
			}
		}

		require.Len(t, trackerSeries, 1)
		require.Equal(t, "temp", trackerSeries[0].Series.Name)
		require.Len(t, trackerSeries[0].Values, 1)
		require.Equal(t, 12.5, trackerSeries[0].Values[0].Value)

		require.Len(t, coverageSeries, 1)
		require.Equal(t, "total", coverageSeries[0].Series.Name)
		require.Len(t, coverageSeries[0].Values, 1)
		require.InDelta(t, 80.0, coverageSeries[0].Values[0].Value, 0.0001)
	})
}