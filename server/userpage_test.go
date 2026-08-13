package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iszk1215/mora/tracker"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func setupUserPageServer(t *testing.T) (*MoraServer, http.Handler) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	userStore := NewUserStore(db)
	require.NoError(t, userStore.Init())

	trackerService, err := tracker.NewService(db)
	require.NoError(t, err)

	server := NewMoraServerBuilder(t).
		WithSessionManager().
		WithTracker(trackerService).
		WithUserStore(userStore).
		Finish()

	return server, server.Handler()
}

func TestHandleUserGet(t *testing.T) {
	server, handler := setupUserPageServer(t)

	owner, err := server.userStore.CreateUser("alice", "https://example.com/avatar.png")
	require.NoError(t, err)

	t.Run("returns user profile", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/users/alice", nil)
		handler.ServeHTTP(w, r)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)

		var got User
		require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
		require.Equal(t, owner.ID, got.ID)
		require.Equal(t, "alice", got.Username)
		require.Equal(t, "https://example.com/avatar.png", got.AvatarURL)
	})

	t.Run("username lookup is case-insensitive", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/users/Alice", nil)
		handler.ServeHTTP(w, r)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("unknown user returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/users/nobody", nil)
		handler.ServeHTTP(w, r)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})
}

func TestHandleUserTrackers(t *testing.T) {
	server, handler := setupUserPageServer(t)

	owner, err := server.userStore.CreateUser("alice", "")
	require.NoError(t, err)
	viewer, err := server.userStore.CreateUser("bob", "")
	require.NoError(t, err)

	pub, err := server.tracker.CreateTracker("public_tracker", "", "", "public", owner.ID, tracker.TypeTracker, "{}")
	require.NoError(t, err)
	_, err = server.tracker.CreateTracker("private_tracker", "", "", "private", owner.ID, tracker.TypeTracker, "{}")
	require.NoError(t, err)

	newAuthedRequest := func(path string, userID int64) *http.Request {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		sid := fmt.Sprintf("session-%d", userID)
		sess := NewMoraSession()
		sess.SetUserID(userID)
		server.sessionManager.store[sid] = sess
		r.AddCookie(&http.Cookie{Name: "morasessionid", Value: sid})
		return r
	}

	request := func(r *http.Request) *http.Response {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Result()
	}

	t.Run("anonymous sees public only", func(t *testing.T) {
		res := request(httptest.NewRequest(http.MethodGet, "/api/users/alice/trackers", nil))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)

		var got tracker.ListTrackersResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
		require.Equal(t, 1, len(got.Trackers))
		require.Equal(t, pub.Id, got.Trackers[0].Id)
		require.Equal(t, "public", got.Trackers[0].Visibility)
	})

	t.Run("owner sees public and private", func(t *testing.T) {
		res := request(newAuthedRequest("/api/users/alice/trackers", owner.ID))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)

		var got tracker.ListTrackersResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
		require.Equal(t, 2, len(got.Trackers))
	})

	t.Run("other user sees public only", func(t *testing.T) {
		res := request(newAuthedRequest("/api/users/alice/trackers", viewer.ID))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)

		var got tracker.ListTrackersResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
		require.Equal(t, 1, len(got.Trackers))
		require.Equal(t, pub.Id, got.Trackers[0].Id)
	})

	t.Run("search query filters owner's trackers", func(t *testing.T) {
		res := request(httptest.NewRequest(http.MethodGet, "/api/users/alice/trackers?q=private", nil))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)

		var got tracker.ListTrackersResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
		require.Empty(t, got.Trackers)
	})

	t.Run("pagination", func(t *testing.T) {
		res := request(newAuthedRequest("/api/users/alice/trackers?page=1&per_page=1", owner.ID))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)

		var got tracker.ListTrackersResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
		require.Equal(t, 2, got.Total)
		require.Equal(t, 1, len(got.Trackers))
	})

	t.Run("unknown user returns 404", func(t *testing.T) {
		res := request(httptest.NewRequest(http.MethodGet, "/api/users/nobody/trackers", nil))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})
}
