package coverage

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func initTestCoverageService(t *testing.T) *CoverageService {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	svc, err := NewCoverageService(db)
	require.NoError(t, err)
	return svc
}

func TestCoverageServiceNew(t *testing.T) {
	svc := initTestCoverageService(t)
	require.NotNil(t, svc)
}

func TestCoverageServiceStore(t *testing.T) {
	svc := initTestCoverageService(t)
	s := svc.Store()
	require.NotNil(t, s)

	cov := &Coverage{
		RepoID:    1,
		Revision:  "abc123",
		Entries:   []*CoverageEntry{},
	}
	id, err := s.Put(cov)
	require.NoError(t, err)
	require.True(t, id > 0)
}

func TestCoverageServiceHandler(t *testing.T) {
	svc := initTestCoverageService(t)
	h := svc.Handler()
	require.NotNil(t, h)
}
