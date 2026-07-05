package tracker

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func initTestService(t *testing.T) *Service {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)
	db.MustExec("PRAGMA foreign_keys = ON")
	db.MustExec(`
		CREATE TABLE user (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			provider_user_id TEXT NOT NULL,
			username TEXT NOT NULL,
			avatar_url TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(provider, provider_user_id)
		)
	`)
	db.MustExec(`INSERT INTO user (id, provider, provider_user_id, username, avatar_url)
		VALUES (1, 'system', 'superuser', 'admin', '')`)

	svc, err := NewService(db, nil)
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
		tr, err := svc.CreateTracker("test_tracker", "private", 1, "tracker", nil)
		require.NoError(t, err)
		require.Equal(t, int64(1), tr.Id)
		require.Equal(t, "test_tracker", tr.Name)
		require.Equal(t, "private", tr.Visibility)
		require.Equal(t, "tracker", tr.Type)
	})

	t.Run("duplicate name", func(t *testing.T) {
		_, err := svc.CreateTracker("dup", "private", 1, "tracker", nil)
		require.NoError(t, err)
		_, err = svc.CreateTracker("dup", "private", 1, "tracker", nil)
		require.Error(t, err)
	})
}

func TestServiceCreateSeries(t *testing.T) {
	svc := initTestService(t)

	tr, err := svc.CreateTracker("test_tracker", "private", 1, "tracker", nil)
	require.NoError(t, err)

	t.Run("valid series", func(t *testing.T) {
		s, err := svc.CreateSeries(tr.Id, "test_series", "float")
		require.NoError(t, err)
		require.Equal(t, tr.Id, s.TrackerId)
		require.Equal(t, "test_series", s.Name)
		require.Equal(t, "float", s.DataType)
	})

	t.Run("series for non-existent tracker", func(t *testing.T) {
		_, err := svc.CreateSeries(999, "orphan", "float")
		require.Error(t, err)
	})
}

func TestServiceCreateValue(t *testing.T) {
	svc := initTestService(t)

	tr, err := svc.CreateTracker("test_tracker", "private", 1, "tracker", nil)
	require.NoError(t, err)

	s, err := svc.CreateSeries(tr.Id, "test_series", "float")
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
