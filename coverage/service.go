package coverage

import (
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
	CreateTracker(name, description, visibility string, userID int64, trackerType string, repoID *int64, chartConfig string) (*tracker.TrackerModel, error)
}

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
// service (which links them via this service), so no direct DB writes happen
// here for tracker rows.
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
		if _, err := creator.CreateTracker(trackerName, "", "public", 1, "coverage", &r.ID, `{"area":false}`); err != nil {
			return fmt.Errorf("MigrateCoverageTrackers create tracker for repo %d: %w", r.ID, err)
		}
	}

	return nil
}
