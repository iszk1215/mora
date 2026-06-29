package track

import (
	"fmt"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func initTestStore(t *testing.T) *trackStore {
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

	s := newTrackStore(db)

	err = s.initialize()
	require.NoError(t, err)

	return s
}

func TestTrackStoreNew(t *testing.T) {
	initTestStore(t)
}

func TestStoreTrack(t *testing.T) {
	s := initTestStore(t)

	t.Run("store track", func(t *testing.T) {
		track := &TrackModel{
			Name: "test_track",
		}

		err := s.addTrack(track, 1)
		require.NoError(t, err)
		require.Equal(t, int64(1), track.Id)

		// Creator should be member with role=owner
		member, role, err := s.isMember(1, track.Id)
		require.NoError(t, err)
		require.True(t, member)
		require.Equal(t, "owner", role)
	})

	t.Run("duplicate name", func(t *testing.T) {
		track := &TrackModel{
			Name: "test_track",
		}

		err := s.addTrack(track, 1)
		require.Error(t, err)
	})
}

func TestStoreFindTrack(t *testing.T) {
	track := &TrackModel{
		Name: "test_track",
	}

	s := initTestStore(t)
	err := s.addTrack(track, 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), track.Id)

	t.Run("find by existing id", func(t *testing.T) {
		got, err := s.findTrackById(track.Id)
		require.NoError(t, err)
		require.Equal(t, track, got)
	})

	t.Run("find by non existing id", func(t *testing.T) {
		_, err := s.findTrackById(1976)
		require.ErrorIs(t, err, errorTrackNotFound)
	})

	t.Run("list for non-member user returns empty", func(t *testing.T) {
		tracks, err := s.listTracks(999)
		require.NoError(t, err)
		require.Empty(t, tracks)
	})

	t.Run("list for owner user returns track", func(t *testing.T) {
		tracks, err := s.listTracks(1)
		require.NoError(t, err)
		require.Equal(t, 1, len(tracks))
		require.Equal(t, track.Id, tracks[0].Id)
		require.Equal(t, "owner", tracks[0].Role)
		require.Equal(t, false, tracks[0].Liked)
	})
}

func TestStoreDeleteTrack(t *testing.T) {
	s := initTestStore(t)

	tracks := []*TrackModel{
		{Name: "track0"},
		{Name: "track1"},
	}

	for _, tr := range tracks {
		err := s.addTrack(tr, 1)
		require.NoError(t, err)
	}

	t.Run("delete existing track", func(t *testing.T) {
		err := s.deleteTrack(tracks[0].Id)
		require.NoError(t, err)
	})

	t.Run("delete non existing track", func(t *testing.T) {
		err := s.deleteTrack(1976)
		require.NoError(t, err)
	})
}

func TestStoreUpdateVisibility(t *testing.T) {
	s := initTestStore(t)

	track := &TrackModel{Name: "vis_test"}
	require.NoError(t, s.addTrack(track, 1))

	t.Run("update to public", func(t *testing.T) {
		err := s.updateVisibility(track.Id, "public")
		require.NoError(t, err)

		got, err := s.findTrackById(track.Id)
		require.NoError(t, err)
		require.Equal(t, "public", got.Visibility)
	})

	t.Run("update to unlisted", func(t *testing.T) {
		err := s.updateVisibility(track.Id, "unlisted")
		require.NoError(t, err)

		got, err := s.findTrackById(track.Id)
		require.NoError(t, err)
		require.Equal(t, "unlisted", got.Visibility)
	})

	t.Run("update to private", func(t *testing.T) {
		err := s.updateVisibility(track.Id, "private")
		require.NoError(t, err)

		got, err := s.findTrackById(track.Id)
		require.NoError(t, err)
		require.Equal(t, "private", got.Visibility)
	})

	t.Run("non-existing track returns error", func(t *testing.T) {
		err := s.updateVisibility(99999, "public")
		require.ErrorIs(t, err, errorTrackNotFound)
	})
}

func TestStoreSeries(t *testing.T) {
	track := &TrackModel{Name: "test_track"}

	s := initTestStore(t)
	err := s.addTrack(track, 1)
	require.NoError(t, err)

	t.Run("add series with existing track", func(t *testing.T) {
		series := &SeriesModel{
			TrackId:  track.Id,
			Name:     "test_series",
			DataType: "float",
		}

		err = s.addSeries(series)
		require.NoError(t, err)
		require.Equal(t, int64(1), series.Id)
	})

	t.Run("add series with non existing track", func(t *testing.T) {
		series := &SeriesModel{
			TrackId:  track.Id + 1,
			Name:     "test_series",
			DataType: "float",
		}

		err = s.addSeries(series)
		require.Error(t, err)
	})

	t.Run("duplicate series name in same track", func(t *testing.T) {
		series := &SeriesModel{
			TrackId:  track.Id,
			Name:     "test_series",
			DataType: "float",
		}

		err = s.addSeries(series)
		require.Error(t, err)
	})
}

func TestStoreFindSeries(t *testing.T) {
	track := &TrackModel{Name: "test_track"}

	s := initTestStore(t)
	err := s.addTrack(track, 1)
	require.NoError(t, err)

	series := &SeriesModel{
		TrackId:  track.Id,
		Name:     "test_series",
		DataType: "int",
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

	t.Run("list by existing track id", func(t *testing.T) {
		items, err := s.listSeries(series.TrackId)
		require.NoError(t, err)
		require.Equal(t, []SeriesModel{*series}, items)
	})

	t.Run("list by non existing track id", func(t *testing.T) {
		items, err := s.listSeries(1215)
		require.NoError(t, err)
		require.Empty(t, items)
	})
}

func TestStoreDeleteSeries(t *testing.T) {
	track := &TrackModel{Name: "test_track"}

	s := initTestStore(t)
	err := s.addTrack(track, 1)
	require.NoError(t, err)

	series := &SeriesModel{
		TrackId:  track.Id,
		Name:     "test_series",
		DataType: "float",
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
	track := &TrackModel{Name: "test_track"}

	s := initTestStore(t)
	err := s.addTrack(track, 1)
	require.NoError(t, err)

	series := &SeriesModel{
		TrackId:  track.Id,
		Name:     "test_series",
		DataType: "float",
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

	tr := &TrackModel{Name: "test_track"}
	err := s.addTrack(tr, 1)
	require.NoError(t, err)

	series := &SeriesModel{TrackId: tr.Id, Name: "test_series", DataType: "float"}
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

	// Delete track should cascade delete series
	tr2 := &TrackModel{Name: "test_track2"}
	err = s.addTrack(tr2, 1)
	require.NoError(t, err)

	series2 := &SeriesModel{TrackId: tr2.Id, Name: "test_series2", DataType: "float"}
	err = s.addSeries(series2)
	require.NoError(t, err)

	value2 := &ValueModel{SeriesId: series2.Id, Timestamp: time.Now().Round(0), Value: 99}
	err = s.addValue(value2)
	require.NoError(t, err)

	err = s.deleteTrack(tr2.Id)
	require.NoError(t, err)

	_, err = s.findTrackById(tr2.Id)
	require.ErrorIs(t, err, errorTrackNotFound)

	_, err = s.findSeriesById(series2.Id)
	require.ErrorIs(t, err, errorSeriesNotFound)

	values2, err := s.listValues(series2.Id, 0)
	require.NoError(t, err)
	require.Empty(t, values2)
}

func TestStoreMember(t *testing.T) {
	s := initTestStore(t)

	tr := &TrackModel{Name: "test_track"}
	err := s.addTrack(tr, 1)
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

	tr := &TrackModel{Name: "test_track"}
	err := s.addTrack(tr, 1)
	require.NoError(t, err)

	t.Run("add like", func(t *testing.T) {
		err := s.addLike(999, tr.Id)
		require.NoError(t, err)

		tracks, err := s.listTracks(999)
		require.NoError(t, err)
		require.Equal(t, 1, len(tracks))
		require.True(t, tracks[0].Liked)
		require.Empty(t, tracks[0].Role)
	})

	t.Run("remove like", func(t *testing.T) {
		err := s.removeLike(999, tr.Id)
		require.NoError(t, err)

		tracks, err := s.listTracks(999)
		require.NoError(t, err)
		require.Empty(t, tracks)
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

		tracks, err := s.listTracks(888)
		require.NoError(t, err)
		require.Equal(t, 1, len(tracks))
	})
}

func TestStoreTrackListUserScoped(t *testing.T) {
	s := initTestStore(t)

	// Create tracks owned by user 1
	tr1 := &TrackModel{Name: "track1"}
	require.NoError(t, s.addTrack(tr1, 1))

	tr2 := &TrackModel{Name: "track2"}
	require.NoError(t, s.addTrack(tr2, 1))

	// User 2 likes track1
	require.NoError(t, s.addLike(2, tr1.Id))

	t.Run("user 1 sees owned tracks with liked flag", func(t *testing.T) {
		tracks, err := s.listTracks(1)
		require.NoError(t, err)
		require.Equal(t, 2, len(tracks))
		for _, tr := range tracks {
			require.Equal(t, "owner", tr.Role)
		}
	})

	t.Run("user 2 sees liked tracks only", func(t *testing.T) {
		tracks, err := s.listTracks(2)
		require.NoError(t, err)
		require.Equal(t, 1, len(tracks))
		require.Equal(t, tr1.Id, tracks[0].Id)
		require.True(t, tracks[0].Liked)
		require.Empty(t, tracks[0].Role)
	})

	t.Run("user 3 sees nothing", func(t *testing.T) {
		tracks, err := s.listTracks(3)
		require.NoError(t, err)
		require.Empty(t, tracks)
	})

	t.Run("track delete cascades to member and like", func(t *testing.T) {
		require.NoError(t, s.deleteTrack(tr1.Id))
		tracks, err := s.listTracks(2)
		require.NoError(t, err)
		require.Empty(t, tracks)
	})
}
