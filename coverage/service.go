package coverage

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/iszk1215/mora/core"
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

// FindRepoByTrackerID resolves the repository described by a coverage tracker.
func (s *CoverageService) FindRepoByTrackerID(trackerID int64) (*core.Repository, error) {
	return s.store.FindRepoByTrackerID(trackerID)
}

// Link links a coverage-type tracker to a repository description.
func (s *CoverageService) Link(trackerID int64, repo core.Repository) error {
	return s.impl.linkTracker(trackerID, repo)
}

// MigrateCoverageTrackers creates a coverage-type tracker for every repository
// that does not have one linked yet, and re-keys existing coverage history to
// the linked tracker id. Rows that migrateCoverage marked with a negative
// repository id are resolved to the tracker created for their repository, and
// rows whose repository no longer exists are removed. Trackers are created
// through the tracker service and linked through this service (coverage owns
// tracker_coverage).
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
		Scm       int64  `db:"scm"`
		Namespace string `db:"namespace"`
		Name      string `db:"name"`
		URL       string `db:"url"`
	}

	var repos []repoRow
	err = s.impl.db.Select(&repos, `
		SELECT r.id, r.scm, r.namespace, r.name, r.url
		FROM repository r
	`)
	if err != nil {
		return fmt.Errorf("MigrateCoverageTrackers select repos: %w", err)
	}

	for _, r := range repos {
		repo := core.Repository{
			Id:                r.ID,
			RepositoryManager: r.Scm,
			Namespace:         r.Namespace,
			Name:              r.Name,
			Url:               r.URL,
		}

		trackerID, err := s.impl.trackerIDByURL(r.URL)
		if err != nil {
			return fmt.Errorf("MigrateCoverageTrackers lookup tracker for repo %d: %w", r.ID, err)
		}
		if trackerID == 0 {
			trackerName := r.Namespace + "/" + r.Name + " coverage"
			tr, err := creator.CreateTracker(trackerName, "", "", "public", CoverageTrackerOwnerID, tracker.TypeCoverage, `{"area":false}`)
			if err != nil {
				return fmt.Errorf("MigrateCoverageTrackers create tracker for repo %d: %w", r.ID, err)
			}
			if err := s.Link(tr.Id, repo); err != nil {
				return fmt.Errorf("MigrateCoverageTrackers link tracker for repo %d: %w", r.ID, err)
			}
		}
	}

	// Resolve coverage history that migrateCoverage marked with a negative
	// repository id to the tracker created for its repository. Real tracker ids
	// are always positive, so only unlinked rows from the old schema match.
	if _, err := s.impl.db.Exec(`
		UPDATE coverage SET tracker_id = (
			SELECT tc.tracker_id FROM tracker_coverage tc
			JOIN repository r ON r.url = tc.url
			WHERE r.id = -coverage.tracker_id)
		WHERE coverage.tracker_id < 0
		  AND EXISTS (SELECT 1 FROM repository r WHERE r.id = -coverage.tracker_id)`); err != nil {
		return fmt.Errorf("MigrateCoverageTrackers re-key coverage: %w", err)
	}

	// Rows whose repository no longer exists have no tracker to resolve to.
	if _, err := s.impl.db.Exec("DELETE FROM coverage WHERE tracker_id < 0"); err != nil {
		return fmt.Errorf("MigrateCoverageTrackers delete orphaned coverage: %w", err)
	}

	return nil
}

// CreateCoverageTracker creates a coverage-type tracker for a repository and
// links it. It returns ErrCoverageTrackerAlreadyLinked when the repository
// already has a coverage tracker.
func (s *CoverageService) CreateCoverageTracker(creator TrackerCreator, name, description, visibility string, userID int64, repo core.Repository) (*tracker.TrackerModel, error) {
	linked, err := s.impl.trackerIDByURL(repo.Url)
	if err != nil {
		return nil, fmt.Errorf("CreateCoverageTracker check existing link: %w", err)
	}
	if linked != 0 {
		return nil, ErrCoverageTrackerAlreadyLinked
	}

	tr, err := creator.CreateTracker(name, description, "", visibility, userID, tracker.TypeCoverage, `{"area":false}`)
	if err != nil {
		return nil, fmt.Errorf("CreateCoverageTracker create tracker: %w", err)
	}
	if err := s.Link(tr.Id, repo); err != nil {
		return nil, fmt.Errorf("CreateCoverageTracker link tracker: %w", err)
	}
	return tr, nil
}
