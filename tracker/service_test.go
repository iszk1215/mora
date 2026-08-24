package tracker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	_ "github.com/tursodatabase/go-libsql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestService(t *testing.T) *Service {
	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	 db.MustExec("PRAGMA foreign_keys = OFF")
	db.MustExec("PRAGMA foreign_keys = ON")
	db.MustExec(`
		CREATE TABLE user (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			provider_user_id TEXT NOT NULL,
			username TEXT NOT NULL,
			avatar_url TEXT NOT NULL DEFAULT '',
			user_type TEXT NOT NULL DEFAULT 'free',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(provider, provider_user_id)
		)
	`)
	db.MustExec(`INSERT INTO user (id, provider, provider_user_id, username, avatar_url, user_type)
		VALUES (1, 'system', 'superuser', 'admin', '', 'admin')`)

	svc, err := NewService(db)
	require.NoError(t, err)
	return svc
}

func TestServiceNew(t *testing.T) {
	svc := initTestService(t)
	require.NotNil(t, svc)
}

func TestServiceCreateTracker(t *testing.T) {
	svc := initTestService(t)

	t.Run("valid tracker", func(t *testing.T) {
		tr, err := svc.CreateTracker("test_tracker", "", "", "private", 1, "tracker", "")
		require.NoError(t, err)
		require.Equal(t, int64(1), tr.Id)
		require.Equal(t, "test_tracker", tr.Name)
		require.Equal(t, "", tr.Description)
		require.Equal(t, "private", tr.Visibility)
		require.Equal(t, "tracker", tr.Type)
	})

	t.Run("duplicate name", func(t *testing.T) {
		_, err := svc.CreateTracker("dup", "", "", "private", 1, "tracker", "")
		require.NoError(t, err)
		_, err = svc.CreateTracker("dup", "", "", "private", 1, "tracker", "")
		require.NoError(t, err)
	})
}

func TestServiceCreateSeries(t *testing.T) {
	svc := initTestService(t)

	tr, err := svc.CreateTracker("test_tracker", "", "", "private", 1, "tracker", "")
	require.NoError(t, err)

	t.Run("valid series", func(t *testing.T) {
		s, err := svc.CreateSeries(tr.Id, "test_series", "float", "")
		require.NoError(t, err)
		require.Equal(t, tr.Id, s.TrackerId)
		require.Equal(t, "test_series", s.Name)
		require.Equal(t, "float", s.DataType)
	})

	t.Run("series for non-existent tracker", func(t *testing.T) {
		_, err := svc.CreateSeries(999, "orphan", "float", "")
		require.Error(t, err)
	})
}

func TestServiceCreateValue(t *testing.T) {
	svc := initTestService(t)

	tr, err := svc.CreateTracker("test_tracker", "", "", "private", 1, "tracker", "")
	require.NoError(t, err)

	s, err := svc.CreateSeries(tr.Id, "test_series", "float", "")
	require.NoError(t, err)

	t.Run("valid value", func(t *testing.T) {
		now := time.Now().Round(0)
		v, err := svc.CreateValue(s.Id, now, 42.5)
		require.NoError(t, err)
		require.Equal(t, s.Id, v.SeriesId)
		require.Equal(t, now, v.Timestamp)
		require.Equal(t, 42.5, v.Value)
	})

	t.Run("value for non-existent series", func(t *testing.T) {
		_, err := svc.CreateValue(999, time.Now(), 1.0)
		require.Error(t, err)
	})
}

func TestServiceHandler(t *testing.T) {
	svc := initTestService(t)
	h := svc.Handler()
	require.NotNil(t, h)
}

func TestServiceFindTrackerById(t *testing.T) {
	svc := initTestService(t)

	tr, err := svc.CreateTracker("findme", "", "", "public", 1, "tracker", "{}")
	require.NoError(t, err)

	t.Run("found", func(t *testing.T) {
		got, err := svc.FindTrackerById(tr.Id)
		require.NoError(t, err)
		assert.Equal(t, "findme", got.Name)
		assert.Equal(t, "public", got.Visibility)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.FindTrackerById(999)
		require.Error(t, err)
	})
}

func TestServiceIsMember(t *testing.T) {
	svc := initTestService(t)

	tr, err := svc.CreateTracker("member_test", "", "", "private", 1, "tracker", "{}")
	require.NoError(t, err)

	// user 1 is owner (added automatically by CreateTracker)
	member, role, err := svc.IsMember(1, tr.Id)
	require.NoError(t, err)
	assert.True(t, member)
	assert.Equal(t, "owner", role)

	// user 999 is not a member
	member, _, err = svc.IsMember(999, tr.Id)
	require.NoError(t, err)
	assert.False(t, member)
}

func TestServiceRequireReadPermission(t *testing.T) {
	svc := initTestService(t)

	publicTr, err := svc.CreateTracker("public_tracker", "", "", "public", 1, "tracker", "{}")
	require.NoError(t, err)

	privateTr, err := svc.CreateTracker("private_tracker", "", "", "private", 1, "tracker", "{}")
	require.NoError(t, err)

	// Create a second user (non-member, non-superuser)
	db := svc.store.db
	_, err = db.Exec(`INSERT INTO user (id, provider, provider_user_id, username, avatar_url)
		VALUES (2, 'test', 'user2', 'user2', '')`)
	require.NoError(t, err)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	// Chain: InjectTracker -> RequireReadPermission -> next
	handler := InjectTracker(svc.store, RequireReadPermission(svc.store, next))

	makeRequest := func(trackerID int64, ctx context.Context) *httptest.ResponseRecorder {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("trackerId", fmt.Sprintf("%d", trackerID))
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		nextCalled = false
		handler.ServeHTTP(w, req)
		return w
	}

	t.Run("public tracker - anonymous allowed", func(t *testing.T) {
		w := makeRequest(publicTr.Id, context.Background())
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, nextCalled)
	})

	t.Run("public tracker - any user allowed", func(t *testing.T) {
		ctx := ContextWithAuth(context.Background(), &[]int64{2}[0])
		w := makeRequest(publicTr.Id, ctx)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, nextCalled)
	})

	t.Run("private tracker - anonymous returns 404", func(t *testing.T) {
		w := makeRequest(privateTr.Id, context.Background())
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.False(t, nextCalled)
	})

	t.Run("private tracker - superuser allowed", func(t *testing.T) {
		uid := int64(1)
		ctx := ContextWithAuth(context.Background(), &uid)
		w := makeRequest(privateTr.Id, ctx)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, nextCalled)
	})

	t.Run("private tracker - member allowed", func(t *testing.T) {
		uid := int64(1) // user 1 is owner
		ctx := ContextWithAuth(context.Background(), &uid)
		w := makeRequest(privateTr.Id, ctx)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, nextCalled)
	})

	t.Run("private tracker - non-member returns 404", func(t *testing.T) {
		uid := int64(2) // user 2 is not a member
		ctx := ContextWithAuth(context.Background(), &uid)
		w := makeRequest(privateTr.Id, ctx)
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.False(t, nextCalled)
	})

	t.Run("invalid tracker id returns 400", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("trackerId", "abc")
		ctx := context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		nextCalled = false
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.False(t, nextCalled)
	})

	t.Run("non-existent tracker returns 404", func(t *testing.T) {
		w := makeRequest(999, context.Background())
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.False(t, nextCalled)
	})
}

func TestServiceRequireEditPermission(t *testing.T) {
	svc := initTestService(t)

	publicTr, err := svc.CreateTracker("public_edit", "", "", "public", 1, "tracker", "{}")
	require.NoError(t, err)

	privateTr, err := svc.CreateTracker("private_edit", "", "", "private", 1, "tracker", "{}")
	require.NoError(t, err)

	db := svc.store.db
	_, err = db.Exec(`INSERT INTO user (id, provider, provider_user_id, username, avatar_url)
		VALUES (2, 'test', 'user2', 'user2', '')`)
	require.NoError(t, err)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	// Chain: InjectTracker -> RequireEditPermission -> next
	handler := InjectTracker(svc.store, RequireEditPermission(svc.store, next))

	makeRequest := func(trackerID int64, ctx context.Context) *httptest.ResponseRecorder {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("trackerId", fmt.Sprintf("%d", trackerID))
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		nextCalled = false
		handler.ServeHTTP(w, req)
		return w
	}

	t.Run("public tracker - owner allowed", func(t *testing.T) {
		uid := int64(1)
		ctx := ContextWithAuth(context.Background(), &uid)
		w := makeRequest(publicTr.Id, ctx)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, nextCalled)
	})

	t.Run("public tracker - non-member returns 404", func(t *testing.T) {
		uid := int64(2)
		ctx := ContextWithAuth(context.Background(), &uid)
		w := makeRequest(publicTr.Id, ctx)
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.False(t, nextCalled)
	})

	t.Run("private tracker - anonymous returns 404", func(t *testing.T) {
		w := makeRequest(privateTr.Id, context.Background())
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.False(t, nextCalled)
	})

	t.Run("private tracker - superuser allowed", func(t *testing.T) {
		uid := int64(1)
		ctx := ContextWithAuth(context.Background(), &uid)
		w := makeRequest(privateTr.Id, ctx)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, nextCalled)
	})

	t.Run("private tracker - non-member returns 404", func(t *testing.T) {
		uid := int64(2)
		ctx := ContextWithAuth(context.Background(), &uid)
		w := makeRequest(privateTr.Id, ctx)
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.False(t, nextCalled)
	})

	t.Run("invalid tracker id returns 400", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("trackerId", "abc")
		ctx := context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		nextCalled = false
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.False(t, nextCalled)
	})

	t.Run("non-existent tracker returns 404", func(t *testing.T) {
		w := makeRequest(999, context.Background())
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.False(t, nextCalled)
	})
}
