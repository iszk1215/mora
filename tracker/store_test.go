package tracker

import (
	"fmt"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func initTestStore(t *testing.T) *trackerStore {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	db.MustExec("PRAGMA foreign_keys = ON")

	// Create user table and seed superuser for FK
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

	// seed additional test users for FK constraints
	for _, uid := range []int64{2, 3, 888, 999} {
		db.MustExec(`INSERT OR IGNORE INTO user (id, provider, provider_user_id, username, avatar_url)
			VALUES (?, 'test', ?, ?, '')`, uid, fmt.Sprintf("user%d", uid), fmt.Sprintf("user%d", uid))
	}

	s := newTrackerStore(db)

	err = s.initialize()
	require.NoError(t, err)

	return s
}

func TestTrackerStoreNew(t *testing.T) {
	initTestStore(t)
}

func TestStoreTracker(t *testing.T) {
	s := initTestStore(t)

	t.Run("store tracker", func(t *testing.T) {
		tracker := &TrackerModel{
			Name: "test_tracker",
		}

		err := s.addTracker(tracker, 1)
		require.NoError(t, err)
		require.Equal(t, int64(1), tracker.Id)

		// Creator should be member with role=owner
		member, role, err := s.isMember(1, tracker.Id)
		require.NoError(t, err)
		require.True(t, member)
		require.Equal(t, "owner", role)
	})

	t.Run("duplicate name", func(t *testing.T) {
		tracker := &TrackerModel{
			Name: "test_tracker",
		}

		err := s.addTracker(tracker, 1)
		require.Error(t, err)
	})
}

func TestStoreFindTracker(t *testing.T) {
	tracker := &TrackerModel{
		Name: "test_tracker",
	}

	s := initTestStore(t)
	err := s.addTracker(tracker, 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), tracker.Id)

	t.Run("find by existing id", func(t *testing.T) {
		got, err := s.findTrackerById(tracker.Id)
		require.NoError(t, err)
		require.Equal(t, tracker, got)
	})

	t.Run("find by non existing id", func(t *testing.T) {
		_, err := s.findTrackerById(1976)
		require.ErrorIs(t, err, errorTrackerNotFound)
	})

	t.Run("list for non-member user returns empty", func(t *testing.T) {
		trackers, err := s.listTrackers(999)
		require.NoError(t, err)
		require.Empty(t, trackers)
	})

	t.Run("list for owner user returns tracker", func(t *testing.T) {
		trackers, err := s.listTrackers(1)
		require.NoError(t, err)
		require.Equal(t, 1, len(trackers))
		require.Equal(t, tracker.Id, trackers[0].Id)
		require.Equal(t, "owner", trackers[0].Role)
		require.Equal(t, false, trackers[0].Liked)
	})
}

func TestStoreDeleteTracker(t *testing.T) {
	s := initTestStore(t)

	trackers := []*TrackerModel{
		{Name: "tracker0"},
		{Name: "tracker1"},
	}

	for _, tr := range trackers {
		err := s.addTracker(tr, 1)
		require.NoError(t, err)
	}

	t.Run("delete existing tracker", func(t *testing.T) {
		err := s.deleteTracker(trackers[0].Id)
		require.NoError(t, err)
	})

	t.Run("delete non existing tracker", func(t *testing.T) {
		err := s.deleteTracker(1976)
		require.NoError(t, err)
	})
}

func TestStoreUpdateVisibility(t *testing.T) {
	s := initTestStore(t)

	tracker := &TrackerModel{Name: "vis_test"}
	require.NoError(t, s.addTracker(tracker, 1))

	t.Run("update to public", func(t *testing.T) {
		err := s.updateVisibility(tracker.Id, "public")
		require.NoError(t, err)

		got, err := s.findTrackerById(tracker.Id)
		require.NoError(t, err)
		require.Equal(t, "public", got.Visibility)
	})

	t.Run("update to unlisted", func(t *testing.T) {
		err := s.updateVisibility(tracker.Id, "unlisted")
		require.NoError(t, err)

		got, err := s.findTrackerById(tracker.Id)
		require.NoError(t, err)
		require.Equal(t, "unlisted", got.Visibility)
	})

	t.Run("update to private", func(t *testing.T) {
		err := s.updateVisibility(tracker.Id, "private")
		require.NoError(t, err)

		got, err := s.findTrackerById(tracker.Id)
		require.NoError(t, err)
		require.Equal(t, "private", got.Visibility)
	})

	t.Run("non-existing tracker returns error", func(t *testing.T) {
		err := s.updateVisibility(99999, "public")
		require.ErrorIs(t, err, errorTrackerNotFound)
	})
}

func TestStoreSeries(t *testing.T) {
	tracker := &TrackerModel{Name: "test_tracker"}

	s := initTestStore(t)
	err := s.addTracker(tracker, 1)
	require.NoError(t, err)

	t.Run("add series with existing tracker", func(t *testing.T) {
		series := &SeriesModel{
			TrackerId: tracker.Id,
			Name:      "test_series",
			DataType:  "float",
		}

		err = s.addSeries(series)
		require.NoError(t, err)
		require.Equal(t, int64(1), series.Id)
	})

	t.Run("add series with non existing tracker", func(t *testing.T) {
		series := &SeriesModel{
			TrackerId: tracker.Id + 1,
			Name:      "test_series",
			DataType:  "float",
		}

		err = s.addSeries(series)
		require.Error(t, err)
	})

	t.Run("duplicate series name in same tracker", func(t *testing.T) {
		series := &SeriesModel{
			TrackerId: tracker.Id,
			Name:      "test_series",
			DataType:  "float",
		}

		err = s.addSeries(series)
		require.Error(t, err)
	})
}

func TestStoreFindSeries(t *testing.T) {
	tracker := &TrackerModel{Name: "test_tracker"}

	s := initTestStore(t)
	err := s.addTracker(tracker, 1)
	require.NoError(t, err)

	series := &SeriesModel{
		TrackerId: tracker.Id,
		Name:      "test_series",
		DataType:  "int",
	}

	err = s.addSeries(series)
	require.NoError(t, err)
	require.Equal(t, int64(1), series.Id)

	t.Run("find existing by id", func(t *testing.T) {
		got, err := s.findSeriesById(series.Id)
		require.NoError(t, err)
		require.Equal(t, series, got)
	})

	t.Run("find non existing by id", func(t *testing.T) {
		_, err := s.findSeriesById(1215)
		require.ErrorIs(t, err, errorSeriesNotFound)
	})

	t.Run("list by existing tracker id", func(t *testing.T) {
		items, err := s.listSeries(series.TrackerId)
		require.NoError(t, err)
		require.Equal(t, []SeriesModel{*series}, items)
	})

	t.Run("list by non existing tracker id", func(t *testing.T) {
		items, err := s.listSeries(1215)
		require.NoError(t, err)
		require.Empty(t, items)
	})
}

func TestStoreDeleteSeries(t *testing.T) {
	tracker := &TrackerModel{Name: "test_tracker"}

	s := initTestStore(t)
	err := s.addTracker(tracker, 1)
	require.NoError(t, err)

	series := &SeriesModel{
		TrackerId: tracker.Id,
		Name:      "test_series",
		DataType:  "float",
	}

	err = s.addSeries(series)
	require.NoError(t, err)

	t.Run("delete existing series", func(t *testing.T) {
		err := s.deleteSeries(series.Id)
		require.NoError(t, err)
	})

	t.Run("delete non existing series", func(t *testing.T) {
		err := s.deleteSeries(series.Id + 1)
		require.NoError(t, err)
	})
}

func TestStoreValue(t *testing.T) {
	tracker := &TrackerModel{Name: "test_tracker"}

	s := initTestStore(t)
	err := s.addTracker(tracker, 1)
	require.NoError(t, err)

	series := &SeriesModel{
		TrackerId: tracker.Id,
		Name:      "test_series",
		DataType:  "float",
	}

	err = s.addSeries(series)
	require.NoError(t, err)

	value := &ValueModel{
		SeriesId:  series.Id,
		Timestamp: time.Now().Round(0),
		Value:     1976.5,
	}

	err = s.addValue(value)
	require.NoError(t, err)
	require.Equal(t, int64(1), value.Id)

	t.Run("list values by non existing series id", func(t *testing.T) {
		values, err := s.listValues(130, 0)
		require.NoError(t, err)
		require.Empty(t, values)
	})

	t.Run("list values by existing series id", func(t *testing.T) {
		values, err := s.listValues(series.Id, 0)
		require.NoError(t, err)
		require.Equal(t, []ValueModel{*value}, values)
	})

	t.Run("list values with limit", func(t *testing.T) {
		values, err := s.listValues(series.Id, 1)
		require.NoError(t, err)
		require.Equal(t, 1, len(values))
	})

	t.Run("delete values", func(t *testing.T) {
		err = s.deleteValues(series.Id)
		require.NoError(t, err)

		values, err := s.listValues(series.Id, 0)
		require.NoError(t, err)
		require.Empty(t, values)
	})
}

func TestStoreDeleteValueCascade(t *testing.T) {
	s := initTestStore(t)

	tr := &TrackerModel{Name: "test_tracker"}
	err := s.addTracker(tr, 1)
	require.NoError(t, err)

	series := &SeriesModel{TrackerId: tr.Id, Name: "test_series", DataType: "float"}
	err = s.addSeries(series)
	require.NoError(t, err)

	value := &ValueModel{SeriesId: series.Id, Timestamp: time.Now().Round(0), Value: 42}
	err = s.addValue(value)
	require.NoError(t, err)

	// Delete series should cascade delete value
	err = s.deleteSeries(series.Id)
	require.NoError(t, err)

	values, err := s.listValues(series.Id, 0)
	require.NoError(t, err)
	require.Empty(t, values)

	// Delete tracker should cascade delete series
	tr2 := &TrackerModel{Name: "test_tracker2"}
	err = s.addTracker(tr2, 1)
	require.NoError(t, err)

	series2 := &SeriesModel{TrackerId: tr2.Id, Name: "test_series2", DataType: "float"}
	err = s.addSeries(series2)
	require.NoError(t, err)

	value2 := &ValueModel{SeriesId: series2.Id, Timestamp: time.Now().Round(0), Value: 99}
	err = s.addValue(value2)
	require.NoError(t, err)

	err = s.deleteTracker(tr2.Id)
	require.NoError(t, err)

	_, err = s.findTrackerById(tr2.Id)
	require.ErrorIs(t, err, errorTrackerNotFound)

	_, err = s.findSeriesById(series2.Id)
	require.ErrorIs(t, err, errorSeriesNotFound)

	values2, err := s.listValues(series2.Id, 0)
	require.NoError(t, err)
	require.Empty(t, values2)
}

func TestStoreMember(t *testing.T) {
	s := initTestStore(t)

	tr := &TrackerModel{Name: "test_tracker"}
	err := s.addTracker(tr, 1)
	require.NoError(t, err)

	t.Run("non-member", func(t *testing.T) {
		ok, role, err := s.isMember(999, tr.Id)
		require.NoError(t, err)
		require.False(t, ok)
		require.Empty(t, role)
	})

	t.Run("owner member", func(t *testing.T) {
		ok, role, err := s.isMember(1, tr.Id)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "owner", role)
	})
}

func TestStoreLike(t *testing.T) {
	s := initTestStore(t)

	tr := &TrackerModel{Name: "test_tracker"}
	err := s.addTracker(tr, 1)
	require.NoError(t, err)

	t.Run("add like", func(t *testing.T) {
		err := s.addLike(999, tr.Id)
		require.NoError(t, err)

		trackers, err := s.listTrackers(999)
		require.NoError(t, err)
		require.Equal(t, 1, len(trackers))
		require.True(t, trackers[0].Liked)
		require.Empty(t, trackers[0].Role)
	})

	t.Run("remove like", func(t *testing.T) {
		err := s.removeLike(999, tr.Id)
		require.NoError(t, err)

		trackers, err := s.listTrackers(999)
		require.NoError(t, err)
		require.Empty(t, trackers)
	})

	t.Run("remove non-existing like", func(t *testing.T) {
		err := s.removeLike(999, tr.Id)
		require.NoError(t, err)
	})

	t.Run("duplicate like is idempotent", func(t *testing.T) {
		err := s.addLike(888, tr.Id)
		require.NoError(t, err)
		err = s.addLike(888, tr.Id)
		require.NoError(t, err)

		trackers, err := s.listTrackers(888)
		require.NoError(t, err)
		require.Equal(t, 1, len(trackers))
	})
}

func TestStoreTrackerListUserScoped(t *testing.T) {
	s := initTestStore(t)

	// Create trackers owned by user 1
	tr1 := &TrackerModel{Name: "tracker1"}
	require.NoError(t, s.addTracker(tr1, 1))

	tr2 := &TrackerModel{Name: "tracker2"}
	require.NoError(t, s.addTracker(tr2, 1))

	// User 2 likes tracker1
	require.NoError(t, s.addLike(2, tr1.Id))

	t.Run("user 1 sees owned trackers with liked flag", func(t *testing.T) {
		trackers, err := s.listTrackers(1)
		require.NoError(t, err)
		require.Equal(t, 2, len(trackers))
		for _, tr := range trackers {
			require.Equal(t, "owner", tr.Role)
		}
	})

	t.Run("user 2 sees liked trackers only", func(t *testing.T) {
		trackers, err := s.listTrackers(2)
		require.NoError(t, err)
		require.Equal(t, 1, len(trackers))
		require.Equal(t, tr1.Id, trackers[0].Id)
		require.True(t, trackers[0].Liked)
		require.Empty(t, trackers[0].Role)
	})

	t.Run("user 3 sees nothing", func(t *testing.T) {
		trackers, err := s.listTrackers(3)
		require.NoError(t, err)
		require.Empty(t, trackers)
	})

	t.Run("tracker delete cascades to member and like", func(t *testing.T) {
		require.NoError(t, s.deleteTracker(tr1.Id))
		trackers, err := s.listTrackers(2)
		require.NoError(t, err)
		require.Empty(t, trackers)
	})
}
