package coverage

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
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

	err := s.Put(want)
	require.NoError(t, err)

	got, err := s.Find(want.ID)
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

func TestCoverageStore_ListAll_Empty(t *testing.T) {
	s := initCoverageStore(t)

	covs, err := s.ListAll()
	require.NoError(t, err)
	require.Empty(t, covs)
}

func TestCoverageStore_Put_Insert(t *testing.T) {
	want := &Coverage{
		RepoID:    1215,
		Revision:  "abcde",
		Timestamp: time.Now().Round(0),
		Entries:   []*CoverageEntry{},
	}

	s := initCoverageStore(t)

	err := s.Put(want)
	require.NoError(t, err)
	require.Equal(t, int64(1), want.ID)

	got, err := s.Find(want.ID)
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

	err := s.Put(want)
	require.NoError(t, err)
	require.Equal(t, int64(1), want.ID)

	got, err := s.Find(want.ID)
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
			errs <- s.Put(cov)
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

	err := s.Put(cov) // Insert
	require.NoError(t, err)
	require.Equal(t, int64(1), cov.ID)

	err = s.Put(want) // Update
	require.NoError(t, err)
	require.Equal(t, int64(1), want.ID)
}
