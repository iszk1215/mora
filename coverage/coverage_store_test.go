package coverage

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iszk1215/mora/core"
	"github.com/iszk1215/mora/coverage/profile"
)

func initCoverageStore(t *testing.T) CoverageStore {
	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	 db.MustExec("PRAGMA foreign_keys = OFF")
	db.SetMaxOpenConns(1)

	s := NewCoverageStore(db)

	err = s.Init()
	require.NoError(t, err)

	return s
}

func TestCoverageStore_New(t *testing.T) {
	initCoverageStore(t)
}

func TestCoverageStore_Find(t *testing.T) {
	s := initCoverageStore(t)
	want := &Coverage{
		TrackerID:    1215,
		Revision:  "123abc",
		Timestamp: time.Now().Round(0),
		Entries: []*CoverageEntry{
			{
				Name:  "go",
				Hits:  13,
				Lines: 17,
			},
		},
	}

	id, err := s.Put(want)
	require.NoError(t, err)
	want.ID = id

	got, err := s.Find(id)
	require.NoError(t, err)
	assertCoverageEqual(t, want, got)
}

func TestCoverageStore_Find_Nil(t *testing.T) {
	s := initCoverageStore(t)

	cov, err := s.Find(0)
	require.NoError(t, err)
	require.Nil(t, cov)
}

func TestCoverageStore_FindRevision_Nil(t *testing.T) {
	s := initCoverageStore(t)

	cov, err := s.FindRevision(0, "revision")
	require.NoError(t, err)
	require.Nil(t, cov)
}

func TestCoverageStore_List_Empty(t *testing.T) {
	s := initCoverageStore(t)

	covs, err := s.List(0)
	require.NoError(t, err)
	require.Empty(t, covs)
}

func TestCoverageStore_Init_CreatesUniqueConstraint(t *testing.T) {
	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	 db.MustExec("PRAGMA foreign_keys = OFF")
	db.SetMaxOpenConns(1)

	s := NewCoverageStore(db)
	err = s.Init()
	require.NoError(t, err)

	type idxInfo struct {
		Seq     int    `db:"seq"`
		Name    string `db:"name"`
		Unique  bool   `db:"unique"`
		Origin  string `db:"origin"`
		Partial int    `db:"partial"`
	}
	var indices []idxInfo
	err = db.Select(&indices, "PRAGMA index_list('coverage')")
	require.NoError(t, err)

	uniqueCount := 0
	for _, idx := range indices {
		if idx.Unique && (idx.Origin == "u" || idx.Origin == "c") {
			uniqueCount++
		}
	}
	require.GreaterOrEqual(t, uniqueCount, 1,
		"Init() must create a UNIQUE constraint or unique index on coverage")
}

func TestCoverageStore_Put_Insert(t *testing.T) {
	want := &Coverage{
		TrackerID:    1215,
		Revision:  "abcde",
		Timestamp: time.Now().Round(0),
		Entries:   []*CoverageEntry{},
	}

	s := initCoverageStore(t)

	id, err := s.Put(want)
	require.NoError(t, err)
	require.Equal(t, int64(1), id)
	want.ID = id

	got, err := s.Find(id)
	require.NoError(t, err)
	assertCoverageEqual(t, want, got)
}

func TestCoverageStore_Put_InsertWithEntry(t *testing.T) {
	want := &Coverage{
		TrackerID:    1215,
		Revision:  "abcde",
		Timestamp: time.Now().Round(0),
		Entries: []*CoverageEntry{
			{
				Name:  "go",
				Hits:  10,
				Lines: 17,
			},
		},
	}

	s := initCoverageStore(t)

	id, err := s.Put(want)
	require.NoError(t, err)
	require.Equal(t, int64(1), id)
	want.ID = id

	got, err := s.Find(id)
	require.NoError(t, err)
	assertCoverageEqual(t, want, got)
}

func TestCoverageStore_Put_Concurrent(t *testing.T) {
	s := initCoverageStore(t)
	cov := &Coverage{
		TrackerID:    1215,
		Revision:  "same-revision",
		Timestamp: time.Now().Round(0),
		Entries:   []*CoverageEntry{},
	}

	var barrier sync.WaitGroup
	barrier.Add(1)

	var wg sync.WaitGroup
	const goroutines = 50
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			barrier.Wait()
			runtime.Gosched()
			_, err := s.Put(cov)
			errs <- err
		}()
	}

	barrier.Done()
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err, "concurrent Put must not fail")
	}
}

func TestCoverageStore_Put_Update(t *testing.T) {
	cov := &Coverage{
		TrackerID:    1215,
		Revision:  "abcde",
		Timestamp: time.Now().Round(0),
		Entries:   []*CoverageEntry{},
	}

	want := &Coverage{
		TrackerID:    1215,
		Revision:  "abcde",
		Timestamp: time.Now().Round(0),
		Entries:   []*CoverageEntry{},
	}

	s := initCoverageStore(t)

	id1, err := s.Put(cov) // Insert
	require.NoError(t, err)
	require.Equal(t, int64(1), id1)

	id2, err := s.Put(want) // Update
	require.NoError(t, err)
	require.Equal(t, int64(1), id2)

	got, err := s.Find(id2)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want.Timestamp.Unix(), got.Timestamp.Unix(),
		"timestamp should be updated on UPSERT conflict")
}

func TestCoverageStore_Put_TouchesLinkedTracker(t *testing.T) {
	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	 db.MustExec("PRAGMA foreign_keys = OFF")
	db.SetMaxOpenConns(1)

	db.MustExec(`CREATE TABLE user (id INTEGER PRIMARY KEY, username TEXT NOT NULL)`)
	db.MustExec(`INSERT INTO user (id, username) VALUES (1, 'admin')`)
	db.MustExec(`CREATE TABLE tracker (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		owner_id INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		last_updated_at DATETIME NOT NULL
	)`)
	db.MustExec(`INSERT INTO tracker (name, owner_id, created_at, last_updated_at)
		VALUES ('t', 1, datetime('now'), datetime('now'))`)

	s := newCoverageStoreImpl(db)
	require.NoError(t, s.Init())
	require.NoError(t, s.linkTracker(1, core.Repository{Id: 1, RepositoryManager: 1, Namespace: "acme", Name: "alpha", Url: "http://example.com/acme/alpha"}))

	var before time.Time
	require.NoError(t, db.Get(&before, "SELECT last_updated_at FROM tracker WHERE id = 1"))

	time.Sleep(5 * time.Millisecond)
	_, err = s.Put(&Coverage{TrackerID: 1, Revision: "abc", Timestamp: time.Now(), Entries: []*CoverageEntry{}})
	require.NoError(t, err)

	var after time.Time
	require.NoError(t, db.Get(&after, "SELECT last_updated_at FROM tracker WHERE id = 1"))
	require.True(t, after.After(before), "linked tracker last_updated_at should be updated by Put")

	t.Run("Put without a link does not fail", func(t *testing.T) {
		_, err := s.Put(&Coverage{TrackerID: 2, Revision: "def", Timestamp: time.Now(), Entries: []*CoverageEntry{}})
		require.NoError(t, err)
	})
}

// --- Timeline tests ---

func TestCoverageStore_Timeline(t *testing.T) {
	now := time.Now().Round(0).Truncate(time.Second)
	entry := &CoverageEntry{
		Name:  "go",
		Hits:  50,
		Lines: 100,
	}

	s := initCoverageStore(t)

	// Insert coverage at two different timestamps
	id1, err := s.Put(&Coverage{
		TrackerID:    1215, Revision: "abc", Timestamp: now.Add(-2 * time.Hour),
		Entries: []*CoverageEntry{{Name: "go", Hits: 30, Lines: 100}},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), id1)

	id2, err := s.Put(&Coverage{
		TrackerID: 1215, Revision: "def", Timestamp: now,
		Entries: []*CoverageEntry{entry},
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), id2)

	t.Run("returns all points in ascending time order", func(t *testing.T) {
		result, err := s.Timeline(1215, 0)
		require.NoError(t, err)
		require.Len(t, result, 1)
		points := result["go"]
		require.Len(t, points, 2)

		require.Equal(t, now.Add(-2*time.Hour).Unix(), points[0].Time.Unix())
		require.InDelta(t, 30.0, points[0].Value, 0.01)

		require.Equal(t, now.Unix(), points[1].Time.Unix())
		require.InDelta(t, 50.0, points[1].Value, 0.01)
	})

	t.Run("limit restricts number of points per entry", func(t *testing.T) {
		// Insert a third point
		_, err := s.Put(&Coverage{
			TrackerID: 1215, Revision: "ghi", Timestamp: now.Add(-time.Hour),
			Entries: []*CoverageEntry{{Name: "go", Hits: 40, Lines: 100}},
		})
		require.NoError(t, err)

		result, err := s.Timeline(1215, 2)
		require.NoError(t, err)
		require.Len(t, result, 1)
		// With limit=2, should return the 2 most recent (in asc order)
		require.Len(t, result["go"], 2)
	})

	t.Run("multiple entries", func(t *testing.T) {
		_, err := s.Put(&Coverage{
			TrackerID: 1215, Revision: "jkl", Timestamp: now,
			Entries: []*CoverageEntry{
				{Name: "go", Hits: 50, Lines: 100},
				{Name: "python", Hits: 80, Lines: 100},
			},
		})
		require.NoError(t, err)

		result, err := s.Timeline(1215, 0)
		require.NoError(t, err)
		require.Contains(t, result, "go")
		require.Contains(t, result, "python")
	})

	t.Run("empty result for non-existing repo", func(t *testing.T) {
		result, err := s.Timeline(9999, 0)
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("zero lines produces zero percentage", func(t *testing.T) {
		s2 := initCoverageStore(t)
		_, err := s2.Put(&Coverage{
			TrackerID:    1,
			Revision: "abc",
			Timestamp: now,
			Entries:   []*CoverageEntry{{Name: "zero", Hits: 0, Lines: 0}},
		})
		require.NoError(t, err)

		result, err := s2.Timeline(1, 0)
		require.NoError(t, err)
		require.InDelta(t, 0.0, result["zero"][0].Value, 0.01)
	})
}

func TestCoverageStore_Timeline_DBError(t *testing.T) {
	s := initCoverageStore(t)
	impl := s.(*coverageStoreImpl)
	require.NoError(t, impl.db.Close())

	_, err := s.Timeline(1, 0)
	require.Error(t, err)
	require.ErrorContains(t, err, "Timeline select")
}

func TestCoverageStore_ReplaceEntries_BeginTxError(t *testing.T) {
	cov := &Coverage{
		TrackerID:    1215,
		Revision:  "abc",
		Timestamp: time.Now().Round(0),
		Entries:   []*CoverageEntry{{Name: "go", Hits: 10, Lines: 20}},
	}

	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	 db.MustExec("PRAGMA foreign_keys = OFF")

	s := NewCoverageStore(db)
	err = s.Init()
	require.NoError(t, err)

	id, err := s.Put(cov)
	require.NoError(t, err)

	impl := s.(*coverageStoreImpl)
	require.NoError(t, impl.db.Close())

	err = impl.replaceEntries(id, cov.Entries)
	require.Error(t, err)
	require.ErrorContains(t, err, "replaceEntries")
}

func TestCoverageStore_LoadCoverage_BlockUnmarshalError(t *testing.T) {
	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	 db.MustExec("PRAGMA foreign_keys = OFF")
	db.SetMaxOpenConns(1)

	s := NewCoverageStore(db)
	err = s.Init()
	require.NoError(t, err)

	// Put a coverage entry, then corrupt its block data directly
	_, err = s.Put(&Coverage{
		TrackerID:    1215,
		Revision:  "abc",
		Timestamp: time.Now().Round(0),
		Entries: []*CoverageEntry{
			{
				Name:  "go",
				Hits:  10,
				Lines: 20,
				Profiles: map[string]*profile.Profile{
					"main.go": {FileName: "main.go", Hits: 10, Lines: 20, Blocks: [][]int{{1, 5, 1}}},
				},
			},
		},
	})
	require.NoError(t, err)

	// Corrupt the blocks in the coverage_block table
	db.MustExec("UPDATE coverage_block SET blocks = 'not-json' WHERE entry_id = (SELECT id FROM coverage_entry LIMIT 1)")

	// Find should fail when trying to unmarshal corrupt blocks
	_, err = s.Find(1)
	require.Error(t, err)
	require.ErrorContains(t, err, "loadCoverage unmarshal blocks")
}

// oldSchema recreates the tracker_coverage schema used before Issue #157 so
// migration from an existing database can be exercised. Statements are kept
// separate because the libSQL driver executes only the first statement of a
// multi-statement string.
const oldSchemaCoverage = `
CREATE TABLE coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id INTEGER NOT NULL,
    revision TEXT NOT NULL,
    time DATETIME NOT NULL,
    contents TEXT NOT NULL,
    UNIQUE(repo_id, revision)
);`
const oldSchemaTrackerCoverage = `
CREATE TABLE tracker_coverage (
    tracker_id INTEGER PRIMARY KEY,
    repo_id    INTEGER NOT NULL,
    FOREIGN KEY (tracker_id) REFERENCES tracker(id) ON DELETE CASCADE,
    FOREIGN KEY (repo_id)    REFERENCES repository(id) ON DELETE CASCADE
);`

func TestCoverageStore_MigrateFromOldSchema(t *testing.T) {
	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	 db.MustExec("PRAGMA foreign_keys = OFF")
	db.SetMaxOpenConns(1)

	db.MustExec(`CREATE TABLE repository (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		scm INTEGER NOT NULL,
		namespace TEXT NOT NULL,
		name TEXT NOT NULL,
		url TEXT NOT NULL UNIQUE
	)`)
	db.MustExec(`INSERT INTO repository (id, scm, namespace, name, url)
		VALUES (1, 2, 'acme', 'alpha', 'http://example.com/acme/alpha')`)
	db.MustExec(`INSERT INTO repository (id, scm, namespace, name, url)
		VALUES (2, 2, 'acme', 'beta', 'http://example.com/acme/beta')`)
	db.MustExec(`CREATE TABLE tracker (id INTEGER PRIMARY KEY AUTOINCREMENT)`)
	db.MustExec(`INSERT INTO tracker (id) VALUES (10)`)

	db.MustExec(oldSchemaCoverage)
	db.MustExec(oldSchemaTrackerCoverage)
	db.MustExec(`INSERT INTO coverage (repo_id, revision, time, contents)
		VALUES (1, 'abc', '2024-01-01', '[]')`)
	db.MustExec(`INSERT INTO coverage (repo_id, revision, time, contents)
		VALUES (2, 'def', '2024-01-01', '[]')`)
	db.MustExec(`INSERT INTO tracker_coverage (tracker_id, repo_id) VALUES (10, 1)`)

	s := NewCoverageStore(db)
	require.NoError(t, s.Init())

	impl := s.(*coverageStoreImpl)
	require.False(t, impl.hasColumn("coverage", "repo_id"))
	require.True(t, impl.hasColumn("coverage", "tracker_id"))
	require.False(t, impl.hasColumn("tracker_coverage", "repo_id"))
	require.True(t, impl.hasColumn("tracker_coverage", "url"))

	repo, err := s.FindRepoByTrackerID(10)
	require.NoError(t, err)
	require.NotNil(t, repo)
	require.Equal(t, &core.Repository{
		Id:                10,
		RepositoryManager: 2,
		Namespace:         "acme",
		Name:              "alpha",
		Url:               "http://example.com/acme/alpha",
	}, repo)

	cov, err := s.Find(1)
	require.NoError(t, err)
	require.NotNil(t, cov)
	require.Equal(t, int64(10), cov.TrackerID)

	// A row whose repository has no link is marked with a negative
	// repository id and resolved by MigrateCoverageTrackers at startup.
	unlinked, err := s.Find(2)
	require.NoError(t, err)
	require.NotNil(t, unlinked)
	require.Equal(t, int64(-2), unlinked.TrackerID)

	// Re-running Init is idempotent.
	require.NoError(t, s.Init())
}

func TestCoverageStore_DeleteTrackerCascadesCoverage(t *testing.T) {
	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	 db.MustExec("PRAGMA foreign_keys = OFF")
	db.SetMaxOpenConns(1)
	db.MustExec("PRAGMA foreign_keys = ON")

	db.MustExec(`CREATE TABLE user (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL)`)
	db.MustExec(`CREATE TABLE tracker (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		owner_id INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		last_updated_at DATETIME NOT NULL
	)`)
	db.MustExec(`INSERT INTO tracker (name, owner_id, created_at, last_updated_at)
		VALUES ('t', 1, datetime('now'), datetime('now'))`)

	s := newCoverageStoreImpl(db)
	require.NoError(t, s.Init())
	require.NoError(t, s.linkTracker(1, core.Repository{
		Id: 1, RepositoryManager: 1, Namespace: "acme", Name: "alpha", Url: "http://example.com/acme/alpha"}))

	_, err = s.Put(&Coverage{TrackerID: 1, Revision: "abc", Timestamp: time.Now(), Entries: []*CoverageEntry{}})
	require.NoError(t, err)
	_, err = s.Put(&Coverage{TrackerID: 1, Revision: "def", Timestamp: time.Now(), Entries: []*CoverageEntry{}})
	require.NoError(t, err)

	_, err = db.Exec("DELETE FROM tracker WHERE id = 1")
	require.NoError(t, err)

	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM coverage"))
	require.Zero(t, count, "deleting a tracker must cascade to its coverage rows")

	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM tracker_coverage"))
	require.Zero(t, count, "deleting a tracker must cascade to its tracker_coverage row")
}

func TestCoverageStore_Put_FailsForMissingTrackerWithForeignKeys(t *testing.T) {
	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	 db.MustExec("PRAGMA foreign_keys = OFF")
	db.SetMaxOpenConns(1)
	db.MustExec("PRAGMA foreign_keys = ON")

	s := newCoverageStoreImpl(db)
	require.NoError(t, s.Init())

	_, err = s.Put(&Coverage{TrackerID: 999, Revision: "abc", Timestamp: time.Now(), Entries: []*CoverageEntry{}})
	require.Error(t, err)
}
