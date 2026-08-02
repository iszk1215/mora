package coverage

import (
	"testing"

	"github.com/iszk1215/mora/tracker"
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

func TestCoverageServiceLinkAndFindRepoIDByTrackerID(t *testing.T) {
	svc := initTestCoverageService(t)

	require.NoError(t, svc.Link(7, 42))

	repoID, err := svc.FindRepoIDByTrackerID(7)
	require.NoError(t, err)
	require.NotNil(t, repoID)
	require.Equal(t, int64(42), *repoID)

	missing, err := svc.FindRepoIDByTrackerID(999)
	require.NoError(t, err)
	require.Nil(t, missing)
}

type fakeTrackerCreator struct {
	calls []fakeCreateCall
	next  int64
}

type fakeCreateCall struct {
	name, description, visibility, trackerType, chartConfig string
	userID                                                    int64
}

func (f *fakeTrackerCreator) CreateTracker(name, description, visibility string, userID int64, trackerType string, chartConfig string) (*tracker.TrackerModel, error) {
	f.calls = append(f.calls, fakeCreateCall{
		name: name, description: description, visibility: visibility,
		trackerType: trackerType, chartConfig: chartConfig, userID: userID,
	})
	f.next++
	return &tracker.TrackerModel{Id: f.next}, nil
}

func TestCoverageServiceMigrateCoverageTrackers(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)
	db.MustExec(`
		CREATE TABLE IF NOT EXISTS repository (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			namespace TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT ''
		)
	`)
	db.MustExec(`INSERT INTO repository (id, namespace, name) VALUES (1, 'acme', 'alpha')`)
	db.MustExec(`INSERT INTO repository (id, namespace, name) VALUES (2, 'acme', 'beta')`)

	svc, err := NewCoverageService(db)
	require.NoError(t, err)
	require.NoError(t, svc.Link(10, 1)) // repo 1 already has a tracker

	creator := &fakeTrackerCreator{}
	require.NoError(t, svc.MigrateCoverageTrackers(creator))
	require.Len(t, creator.calls, 1)

	call := creator.calls[0]
	require.Equal(t, "acme/beta coverage", call.name)
	require.Equal(t, "", call.description)
	require.Equal(t, "public", call.visibility)
	require.Equal(t, int64(1), call.userID)
	require.Equal(t, "coverage", call.trackerType)
	require.Equal(t, `{"area":false}`, call.chartConfig)

	repoID, err := svc.FindRepoIDByTrackerID(creator.next)
	require.NoError(t, err)
	require.NotNil(t, repoID)
	require.Equal(t, int64(2), *repoID)

	require.NoError(t, svc.MigrateCoverageTrackers(creator))
	require.Len(t, creator.calls, 1)
}

func TestCoverageServiceMigrateCoverageTrackersSkipsWithoutRepositoryTable(t *testing.T) {
	svc := initTestCoverageService(t)

	creator := &fakeTrackerCreator{}
	require.NoError(t, svc.MigrateCoverageTrackers(creator))
	require.Empty(t, creator.calls)
}

func TestCoverageServiceCreateCoverageTracker(t *testing.T) {
	svc := initTestCoverageService(t)

	creator := &fakeTrackerCreator{}
	tr, err := svc.CreateCoverageTracker(creator, "my coverage", "", "public", 1, 42)
	require.NoError(t, err)
	require.Equal(t, int64(1), tr.Id)

	require.Len(t, creator.calls, 1)
	call := creator.calls[0]
	require.Equal(t, "my coverage", call.name)
	require.Equal(t, "", call.description)
	require.Equal(t, "public", call.visibility)
	require.Equal(t, int64(1), call.userID)
	require.Equal(t, "coverage", call.trackerType)
	require.Equal(t, `{"area":false}`, call.chartConfig)

	repoID, err := svc.FindRepoIDByTrackerID(1)
	require.NoError(t, err)
	require.NotNil(t, repoID)
	require.Equal(t, int64(42), *repoID)

	_, err = svc.CreateCoverageTracker(creator, "again", "", "public", 1, 42)
	require.ErrorIs(t, err, ErrCoverageTrackerAlreadyLinked)
	require.Len(t, creator.calls, 1)
}
