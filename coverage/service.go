package coverage

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/iszk1215/mora/tracker"
	"github.com/jmoiron/sqlx"
)

type (
	CoverageService struct {
		handler *CoverageHandler
		store   CoverageStore
		impl    *coverageStoreImpl
	}
)

// TrackerCreator creates trackers via the tracker package.
// Implemented by tracker.Service.
type TrackerCreator interface {
	CreateTracker(name, description, body, visibility string, userID int64, trackerType string, chartConfig string) (*tracker.TrackerModel, error)
}

// ErrCoverageTrackerAlreadyLinked is returned when a repository already has a
// coverage tracker, so a new one must not be created.
var ErrCoverageTrackerAlreadyLinked = errors.New("coverage tracker already exists for repository")

// CoverageTrackerOwnerID is the user that owns coverage trackers created during
// migration (the superuser/admin account).
const CoverageTrackerOwnerID int64 = 1

func NewCoverageService(db *sqlx.DB) (*CoverageService, error) {

	impl := newCoverageStoreImpl(db)
	if err := impl.Init(); err != nil {
		return nil, fmt.Errorf("coverage store Init: %w", err)
	}

	return &CoverageService{handler: newCoverageHandler(impl), store: impl, impl: impl}, nil
}

func (s *CoverageService) Handler() http.Handler {
	return s.handler.Handler()
}

func (s *CoverageService) HandleCoverageListPublic(w http.ResponseWriter, r *http.Request) {
	s.handler.HandleCoverageListPublic(w, r)
}

func (s *CoverageService) HandleCoveragePreview(w http.ResponseWriter, r *http.Request) {
	s.handler.HandleCoveragePreview(w, r)
}

func (s *CoverageService) Store() CoverageStore {
	return s.store
}

// FindRepoIDByTrackerID resolves the repo_id linked to a coverage tracker.
func (s *CoverageService) FindRepoIDByTrackerID(trackerID int64) (*int64, error) {
	return s.store.FindRepoIDByTrackerID(trackerID)
}

// Link links a coverage-type tracker to a repository.
func (s *CoverageService) Link(trackerID, repoID int64) error {
	return s.impl.linkTracker(trackerID, repoID)
}

// MigrateCoverageTrackers creates a coverage-type tracker for every repository
// that does not have one linked yet. Trackers are created through the tracker
// service and linked through this service (coverage owns tracker_coverage).
func (s *CoverageService) MigrateCoverageTrackers(creator TrackerCreator) error {
	var count int
	err := s.impl.db.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='repository'")
	if err != nil || count == 0 {
		return nil
	}

	err = s.impl.db.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tracker_coverage'")
	if err != nil || count == 0 {
		return nil
	}

	type repoRow struct {
		ID        int64  `db:"id"`
		Namespace string `db:"namespace"`
		Name      string `db:"name"`
	}

	var repos []repoRow
	err = s.impl.db.Select(&repos, `
		SELECT r.id, r.namespace, r.name
		FROM repository r
		WHERE NOT EXISTS (
			SELECT 1 FROM tracker_coverage tc WHERE tc.repo_id = r.id
		)
	`)
	if err != nil {
		return fmt.Errorf("MigrateCoverageTrackers select repos: %w", err)
	}

	for _, r := range repos {
		trackerName := r.Namespace + "/" + r.Name + " coverage"
		tr, err := creator.CreateTracker(trackerName, "", "", "public", CoverageTrackerOwnerID, tracker.TypeCoverage, `{"area":false}`)
		if err != nil {
			return fmt.Errorf("MigrateCoverageTrackers create tracker for repo %d: %w", r.ID, err)
		}
		if err := s.Link(tr.Id, r.ID); err != nil {
			return fmt.Errorf("MigrateCoverageTrackers link tracker for repo %d: %w", r.ID, err)
		}
	}

	return nil
}

// CreateCoverageTracker creates a coverage-type tracker for a repository and
// links it. It returns ErrCoverageTrackerAlreadyLinked when the repository
// already has a coverage tracker.
func (s *CoverageService) CreateCoverageTracker(creator TrackerCreator, name, description, visibility string, userID, repoID int64) (*tracker.TrackerModel, error) {
	var linked int
	err := s.impl.db.Get(&linked, "SELECT COUNT(*) FROM tracker_coverage WHERE repo_id = ?", repoID)
	if err != nil {
		return nil, fmt.Errorf("CreateCoverageTracker check existing link: %w", err)
	}
	if linked > 0 {
		return nil, ErrCoverageTrackerAlreadyLinked
	}

	tr, err := creator.CreateTracker(name, description, "", visibility, userID, tracker.TypeCoverage, `{"area":false}`)
	if err != nil {
		return nil, fmt.Errorf("CreateCoverageTracker create tracker: %w", err)
	}
	if err := s.Link(tr.Id, repoID); err != nil {
		return nil, fmt.Errorf("CreateCoverageTracker link tracker: %w", err)
	}
	return tr, nil
}
