package tracker

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	_ "github.com/tursodatabase/go-libsql"
)

func openOldTrackerDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	db.MustExec("PRAGMA foreign_keys = OFF")

	db.MustExec(`CREATE TABLE tracker (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		visibility TEXT NOT NULL DEFAULT 'private',
		type TEXT NOT NULL DEFAULT 'tracker',
		chart_config TEXT NOT NULL DEFAULT '{}',
		owner_id INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		last_updated_at DATETIME NOT NULL
	)`)
	db.MustExec(`INSERT INTO tracker (name, owner_id, created_at, last_updated_at)
		VALUES ('first', 1, datetime('now'), datetime('now')),
		       ('second', 1, datetime('now'), datetime('now'))`)
	return db
}

func TestMigrateBackfillsExternalIDs(t *testing.T) {
	db := openOldTrackerDB(t)
	s := newTrackerStore(db)
	require.NoError(t, s.initialize())

	var ids []string
	require.NoError(t, db.Select(&ids, "SELECT external_id FROM tracker ORDER BY id"))
	require.Len(t, ids, 2)

	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		require.NotEmpty(t, id)
		parsed, err := uuid.Parse(id)
		require.NoError(t, err)
		require.False(t, seen[parsed.String()], "duplicate external id %q", id)
		seen[parsed.String()] = true
	}

	var indexCount int
	require.NoError(t, db.Get(&indexCount,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_tracker_external_id'"))
	require.Equal(t, 1, indexCount)

	// Running initialize again must not change existing external IDs.
	require.NoError(t, s.initialize())
	var idsAgain []string
	require.NoError(t, db.Select(&idsAgain, "SELECT external_id FROM tracker ORDER BY id"))
	require.Equal(t, ids, idsAgain)
}

func openOldSeriesValueDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db := openOldTrackerDB(t)
	db.MustExec(`CREATE TABLE tracker_series (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tracker_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		data_type TEXT NOT NULL,
		config TEXT NOT NULL DEFAULT '{}'
	)`)
	db.MustExec(`INSERT INTO tracker_series (tracker_id, name, data_type, config)
		VALUES (1, 'first', 'float', '{}'),
		       (1, 'second', 'float', '{}')`)
	db.MustExec(`CREATE TABLE tracker_value (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		series_id INTEGER NOT NULL,
		time DATETIME NOT NULL,
		value REAL NOT NULL
	)`)
	db.MustExec(`INSERT INTO tracker_value (series_id, time, value)
		VALUES (1, datetime('now'), 1.0),
		       (1, datetime('now', '+1 hour'), 2.0),
		       (2, datetime('now'), 3.0)`)
	return db
}

func TestMigrateBackfillsSeriesValueExternalIDs(t *testing.T) {
	db := openOldSeriesValueDB(t)
	s := newTrackerStore(db)
	require.NoError(t, s.initialize())

	for _, table := range []string{"tracker_series", "tracker_value"} {
		var ids []string
		require.NoError(t, db.Select(&ids, "SELECT external_id FROM "+table+" ORDER BY id"))
		require.NotEmpty(t, ids)

		seen := make(map[string]bool, len(ids))
		for _, id := range ids {
			require.NotEmpty(t, id)
			parsed, err := uuid.Parse(id)
			require.NoError(t, err)
			require.False(t, seen[parsed.String()], "duplicate external id %q in %s", id, table)
			seen[parsed.String()] = true
		}
	}

	for _, name := range []string{"idx_tracker_series_external_id", "idx_tracker_value_external_id"} {
		var count int
		require.NoError(t, db.Get(&count,
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", name))
		require.Equal(t, 1, count, name)
	}

	var beforeSeries []string
	require.NoError(t, db.Select(&beforeSeries, "SELECT external_id FROM tracker_series ORDER BY id"))
	var beforeValues []string
	require.NoError(t, db.Select(&beforeValues, "SELECT external_id FROM tracker_value ORDER BY id"))

	require.NoError(t, s.initialize())

	var afterSeries []string
	require.NoError(t, db.Select(&afterSeries, "SELECT external_id FROM tracker_series ORDER BY id"))
	require.Equal(t, beforeSeries, afterSeries)
	var afterValues []string
	require.NoError(t, db.Select(&afterValues, "SELECT external_id FROM tracker_value ORDER BY id"))
	require.Equal(t, beforeValues, afterValues)

	series, err := s.findSeriesById(1)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(series.Id), seriesExternalIDPrefix))

	values, err := s.listValues(1, 0)
	require.NoError(t, err)
	require.NotEmpty(t, values)
	for _, v := range values {
		require.True(t, strings.HasPrefix(string(v.Id), valueExternalIDPrefix))
	}
}

func TestMigrateBackfillsSeriesPositions(t *testing.T) {
	db := openOldSeriesValueDB(t)
	s := newTrackerStore(db)
	require.NoError(t, s.initialize())

	series, err := s.listSeries(1)
	require.NoError(t, err)
	require.Len(t, series, 2)
	require.Equal(t, "first", series[0].Name)
	require.Equal(t, 1, series[0].Position)
	require.Equal(t, "second", series[1].Name)
	require.Equal(t, 2, series[1].Position)

	require.NoError(t, s.reorderSeries(1, []SeriesExternalID{series[1].Id, series[0].Id}))
	require.NoError(t, s.initialize())

	series, err = s.listSeries(1)
	require.NoError(t, err)
	require.Len(t, series, 2)
	require.Equal(t, "second", series[0].Name)
	require.Equal(t, 0, series[0].Position)
	require.Equal(t, "first", series[1].Name)
	require.Equal(t, 1, series[1].Position)
}

func TestStoreExternalIDRoundTrip(t *testing.T) {
	s := initTestStore(t)

	tr := &TrackerModel{Name: "round_trip"}
	require.NoError(t, s.addTracker(tr, 1))
	require.NotEmpty(t, tr.ExternalId)

	parsed, err := ParseExternalID(string(tr.ExternalId))
	require.NoError(t, err)
	require.Equal(t, tr.ExternalId, parsed)

	got, err := s.findTrackerByExternalID(parsed)
	require.NoError(t, err)
	require.Equal(t, tr.Id, got.Id)
	require.Equal(t, tr.ExternalId, got.ExternalId)

	_, err = s.findTrackerByExternalID(ExternalID("trk_" + uuid.NewString()))
	require.ErrorIs(t, err, errorTrackerNotFound)
}
