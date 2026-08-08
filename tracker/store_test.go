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
		require.NoError(t, err)
	})

	t.Run("store body", func(t *testing.T) {
		tracker := &TrackerModel{
			Name: "body_tracker",
			Body: "# Title\n\nMarkdown body",
		}

		err := s.addTracker(tracker, 1)
		require.NoError(t, err)

		got, err := s.findTrackerById(tracker.Id)
		require.NoError(t, err)
		require.Equal(t, tracker.Body, got.Body)

		resp, err := s.findTrackerResponseById(tracker.Id, 1)
		require.NoError(t, err)
		require.Equal(t, tracker.Body, resp.Body)
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
		require.Equal(t, tracker.Id, got.Id)
		require.Equal(t, tracker.Name, got.Name)
		require.Equal(t, tracker.Visibility, got.Visibility)
		require.Equal(t, tracker.OwnerId, got.OwnerId)
		require.True(t, got.CreatedAt.Equal(tracker.CreatedAt))
		require.True(t, got.LastUpdatedAt.Equal(tracker.LastUpdatedAt))
	})

	t.Run("find by non existing id", func(t *testing.T) {
		_, err := s.findTrackerById(1976)
		require.ErrorIs(t, err, errorTrackerNotFound)
	})

	t.Run("list for non-member user returns empty", func(t *testing.T) {
		trackers, total, err := s.listTrackers(999, "", 0, 0)
		require.NoError(t, err)
		require.Empty(t, trackers)
		require.Equal(t, 0, total)
	})

	t.Run("list for owner user returns tracker", func(t *testing.T) {
		trackers, total, err := s.listTrackers(1, "", 0, 0)
		require.NoError(t, err)
		require.Equal(t, 1, len(trackers))
		require.Equal(t, 1, total)
		require.Equal(t, tracker.Id, trackers[0].Id)
		require.Equal(t, "owner", trackers[0].Role)
		require.Equal(t, false, trackers[0].Liked)
	})

	t.Run("list with pagination", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			tr := &TrackerModel{Name: fmt.Sprintf("paginate_%d", i)}
			require.NoError(t, s.addTracker(tr, 1))
		}
		trackers, total, err := s.listTrackers(1, "", 1, 3)
		require.NoError(t, err)
		require.Equal(t, 3, len(trackers))
		require.Equal(t, 6, total)
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

func TestStoreUpdateTracker(t *testing.T) {
	s := initTestStore(t)

	tracker := &TrackerModel{Name: "vis_test"}
	require.NoError(t, s.addTracker(tracker, 1))

	t.Run("update to public", func(t *testing.T) {
		v := "public"
		err := s.updateTracker(tracker.Id, &v, nil, nil, nil)
		require.NoError(t, err)

		got, err := s.findTrackerById(tracker.Id)
		require.NoError(t, err)
		require.Equal(t, "public", got.Visibility)
	})

	t.Run("update description", func(t *testing.T) {
		desc := "test description"
		err := s.updateTracker(tracker.Id, nil, nil, &desc, nil)
		require.NoError(t, err)

		got, err := s.findTrackerById(tracker.Id)
		require.NoError(t, err)
		require.Equal(t, desc, got.Description)
	})

	t.Run("update to private", func(t *testing.T) {
		v := "private"
		err := s.updateTracker(tracker.Id, &v, nil, nil, nil)
		require.NoError(t, err)

		got, err := s.findTrackerById(tracker.Id)
		require.NoError(t, err)
		require.Equal(t, "private", got.Visibility)
	})

	t.Run("non-existing tracker returns error", func(t *testing.T) {
		v := "public"
		err := s.updateTracker(99999, &v, nil, nil, nil)
		require.ErrorIs(t, err, errorTrackerNotFound)
	})

	t.Run("update chart_config", func(t *testing.T) {
		cc := `{"x_axis_label":"Time"}`
		err := s.updateTracker(tracker.Id, nil, &cc, nil, nil)
		require.NoError(t, err)

		got, err := s.findTrackerById(tracker.Id)
		require.NoError(t, err)
		require.Equal(t, cc, got.ChartConfig)
	})

	t.Run("update body", func(t *testing.T) {
		body := "## Overview\n\nSome markdown content"
		err := s.updateTracker(tracker.Id, nil, nil, nil, &body)
		require.NoError(t, err)

		got, err := s.findTrackerById(tracker.Id)
		require.NoError(t, err)
		require.Equal(t, body, got.Body)
	})

	t.Run("nil fields is no-op", func(t *testing.T) {
		err := s.updateTracker(tracker.Id, nil, nil, nil, nil)
		require.NoError(t, err)
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

func TestStoreListLatestValues(t *testing.T) {
	s := initTestStore(t)

	tr := &TrackerModel{Name: "test_tracker"}
	require.NoError(t, s.addTracker(tr, 1))

	series := &SeriesModel{TrackerId: tr.Id, Name: "test_series", DataType: "float"}
	require.NoError(t, s.addSeries(series))

	now := time.Now().Round(0)
	for i := 0; i < 10; i++ {
		v := &ValueModel{SeriesId: series.Id, Timestamp: now.Add(time.Duration(i) * time.Hour), Value: float64(i)}
		require.NoError(t, s.addValue(v))
	}

	t.Run("latest values returns most recent first in ASC order", func(t *testing.T) {
		values, err := s.listLatestValues(series.Id, 3)
		require.NoError(t, err)
		require.Equal(t, 3, len(values))
		require.Equal(t, 7.0, values[0].Value)
		require.Equal(t, 8.0, values[1].Value)
		require.Equal(t, 9.0, values[2].Value)
	})

	t.Run("no limit returns all in ASC order", func(t *testing.T) {
		values, err := s.listLatestValues(series.Id, 0)
		require.NoError(t, err)
		require.Equal(t, 10, len(values))
		require.Equal(t, 0.0, values[0].Value)
		require.Equal(t, 9.0, values[9].Value)
	})

	t.Run("no values returns empty", func(t *testing.T) {
		values, err := s.listLatestValues(999, 5)
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

		trackers, _, err := s.listTrackers(999, "", 0, 0)
		require.NoError(t, err)
		require.Equal(t, 1, len(trackers))
		require.True(t, trackers[0].Liked)
		require.Empty(t, trackers[0].Role)
	})

	t.Run("remove like", func(t *testing.T) {
		err := s.removeLike(999, tr.Id)
		require.NoError(t, err)

		trackers, _, err := s.listTrackers(999, "", 0, 0)
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

		trackers, _, err := s.listTrackers(888, "", 0, 0)
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
		trackers, total, err := s.listTrackers(1, "", 0, 0)
		require.NoError(t, err)
		require.Equal(t, 2, len(trackers))
		require.Equal(t, 2, total)
		for _, tr := range trackers {
			require.Equal(t, "owner", tr.Role)
		}
	})

	t.Run("user 2 sees liked trackers only", func(t *testing.T) {
		trackers, total, err := s.listTrackers(2, "", 0, 0)
		require.NoError(t, err)
		require.Equal(t, 1, len(trackers))
		require.Equal(t, 1, total)
		require.Equal(t, tr1.Id, trackers[0].Id)
		require.True(t, trackers[0].Liked)
		require.Empty(t, trackers[0].Role)
	})

	t.Run("user 3 sees nothing", func(t *testing.T) {
		trackers, total, err := s.listTrackers(3, "", 0, 0)
		require.NoError(t, err)
		require.Empty(t, trackers)
		require.Equal(t, 0, total)
	})

	t.Run("tracker delete cascades to member and like", func(t *testing.T) {
		require.NoError(t, s.deleteTracker(tr1.Id))
		trackers, total, err := s.listTrackers(2, "", 0, 0)
		require.NoError(t, err)
		require.Empty(t, trackers)
		require.Equal(t, 0, total)
	})
}

func TestStoreTrackerSearch(t *testing.T) {
	s := initTestStore(t)

	// User 1 owns "tracker_alpha" (private) and "tracker_beta" (public)
	tr1 := &TrackerModel{Name: "tracker_alpha", Visibility: "private"}
	require.NoError(t, s.addTracker(tr1, 1))

	tr2 := &TrackerModel{Name: "tracker_beta", Visibility: "public"}
	require.NoError(t, s.addTracker(tr2, 1))

	// User 2 owns "public_gamma"
	tr3 := &TrackerModel{Name: "public_gamma", Visibility: "public"}
	require.NoError(t, s.addTracker(tr3, 2))

	t.Run("logged in, no query returns user's trackers", func(t *testing.T) {
		trackers, total, err := s.listTrackers(1, "", 0, 0)
		require.NoError(t, err)
		require.Equal(t, 2, total)
		require.Equal(t, 2, len(trackers))
	})

	t.Run("logged in, query matches user's trackers and public", func(t *testing.T) {
		trackers, total, err := s.listTrackers(1, "tracker", 0, 0)
		require.NoError(t, err)
		// user 1's trackers matching "tracker": tracker_alpha, tracker_beta
		// public trackers matching "tracker": tracker_beta (already counted)
		require.Equal(t, 2, total)
		require.Equal(t, 2, len(trackers))
	})

	t.Run("logged in, query matches only public from other user", func(t *testing.T) {
		trackers, total, err := s.listTrackers(1, "gamma", 0, 0)
		require.NoError(t, err)
		// user 1's trackers: none match "gamma"
		// public trackers: "public_gamma" matches "gamma"
		require.Equal(t, 1, total)
		require.Equal(t, 1, len(trackers))
		require.Equal(t, tr3.Id, trackers[0].Id)
	})

	t.Run("not logged in, no query returns empty", func(t *testing.T) {
		trackers, total, err := s.listTrackers(0, "", 0, 0)
		require.NoError(t, err)
		require.Empty(t, trackers)
		require.Equal(t, 0, total)
	})

	t.Run("not logged in, query matches public only", func(t *testing.T) {
		trackers, total, err := s.listTrackers(0, "gamma", 0, 0)
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Equal(t, 1, len(trackers))
		require.Equal(t, tr3.Id, trackers[0].Id)
	})

	t.Run("not logged in, query does not match private", func(t *testing.T) {
		trackers, total, err := s.listTrackers(0, "alpha", 0, 0)
		require.NoError(t, err)
		require.Empty(t, trackers)
		require.Equal(t, 0, total)
	})
}

func TestStoreTrackerOwner(t *testing.T) {
	s := initTestStore(t)

	t.Run("addTracker sets owner and timestamps", func(t *testing.T) {
		tr := &TrackerModel{Name: "owner_tracker"}
		require.NoError(t, s.addTracker(tr, 2))
		require.Equal(t, int64(2), tr.OwnerId)
		require.False(t, tr.CreatedAt.IsZero())
		require.True(t, tr.LastUpdatedAt.Equal(tr.CreatedAt))

		got, err := s.findTrackerById(tr.Id)
		require.NoError(t, err)
		require.Equal(t, int64(2), got.OwnerId)

		// owner is derived from owner_id, not stored as a tracker_member row
		member, role, err := s.isMember(2, tr.Id)
		require.NoError(t, err)
		require.True(t, member)
		require.Equal(t, "owner", role)

		var memberRows int
		err = s.db.Get(&memberRows, "SELECT COUNT(*) FROM tracker_member WHERE tracker_id = ?", tr.Id)
		require.NoError(t, err)
		require.Equal(t, 0, memberRows)
	})

	t.Run("non-owner member is editor", func(t *testing.T) {
		tr := &TrackerModel{Name: "editor_tracker"}
		require.NoError(t, s.addTracker(tr, 1))
		_, err := s.db.Exec("INSERT INTO tracker_member (user_id, tracker_id, role) VALUES (2, ?, 'editor')", tr.Id)
		require.NoError(t, err)

		member, role, err := s.isMember(2, tr.Id)
		require.NoError(t, err)
		require.True(t, member)
		require.Equal(t, "editor", role)
	})

	t.Run("addValue updates last_updated_at", func(t *testing.T) {
		tr := &TrackerModel{Name: "touch_tracker"}
		require.NoError(t, s.addTracker(tr, 1))
		ser := &SeriesModel{TrackerId: tr.Id, Name: "s", DataType: "float"}
		require.NoError(t, s.addSeries(ser))

		time.Sleep(5 * time.Millisecond)
		v := &ValueModel{SeriesId: ser.Id, Timestamp: time.Now(), Value: 1.0}
		require.NoError(t, s.addValue(v))

		got, err := s.findTrackerById(tr.Id)
		require.NoError(t, err)
		require.True(t, got.LastUpdatedAt.After(tr.LastUpdatedAt))
	})

	t.Run("list includes owner info", func(t *testing.T) {
		tr := &TrackerModel{Name: "owner_list"}
		require.NoError(t, s.addTracker(tr, 2))
		trackers, _, err := s.listTrackers(2, "owner_list", 0, 0)
		require.NoError(t, err)
		require.Equal(t, 1, len(trackers))
		require.Equal(t, int64(2), trackers[0].OwnerId)
		require.Equal(t, "user2", trackers[0].OwnerName)
		require.Equal(t, "owner", trackers[0].Role)
		require.True(t, trackers[0].CreatedAt.Equal(tr.CreatedAt))
		require.True(t, trackers[0].LastUpdatedAt.Equal(tr.LastUpdatedAt))
	})
}
