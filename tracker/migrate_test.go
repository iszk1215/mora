package tracker

import (
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
