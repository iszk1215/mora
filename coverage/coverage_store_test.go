package coverage

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iszk1215/mora/coverage/profile"
)

func initCoverageStore(t *testing.T) CoverageStore {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	// db, err := sqlx.Connect("sqlite3", ":memory:")
	require.NoError(t, err)
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
		RepoID:    1215,
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
	require.Equal(t, want, got)
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
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)
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
		RepoID:    1215,
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
	require.Equal(t, want, got)
}

func TestCoverageStore_Put_InsertWithEntry(t *testing.T) {
	want := &Coverage{
		RepoID:    1215,
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
	require.Equal(t, want, got)
}

func TestCoverageStore_Put_Concurrent(t *testing.T) {
	s := initCoverageStore(t)
	cov := &Coverage{
		RepoID:    1215,
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
		RepoID:    1215,
		Revision:  "abcde",
		Timestamp: time.Now().Round(0),
		Entries:   []*CoverageEntry{},
	}

	want := &Coverage{
		RepoID:    1215,
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
		RepoID:    1215, Revision: "abc", Timestamp: now.Add(-2 * time.Hour),
		Entries: []*CoverageEntry{{Name: "go", Hits: 30, Lines: 100}},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), id1)

	id2, err := s.Put(&Coverage{
		RepoID: 1215, Revision: "def", Timestamp: now,
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
			RepoID: 1215, Revision: "ghi", Timestamp: now.Add(-time.Hour),
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
			RepoID: 1215, Revision: "jkl", Timestamp: now,
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
			RepoID:    1,
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
		RepoID:    1215,
		Revision:  "abc",
		Timestamp: time.Now().Round(0),
		Entries:   []*CoverageEntry{{Name: "go", Hits: 10, Lines: 20}},
	}

	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

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
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)

	s := NewCoverageStore(db)
	err = s.Init()
	require.NoError(t, err)

	// Put a coverage entry, then corrupt its block data directly
	_, err = s.Put(&Coverage{
		RepoID:    1215,
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
