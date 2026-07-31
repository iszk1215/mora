package coverage

import (
	"fmt"
	"net/http"

	"github.com/jmoiron/sqlx"
)

type (
	CoverageService struct {
		handler *CoverageHandler
		store   CoverageStore
		impl    *coverageStoreImpl
	}
)

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

// FindRepoIDByTrackerID implements tracker.CoverageLinkManager.
func (s *CoverageService) FindRepoIDByTrackerID(trackerID int64) (*int64, error) {
	return s.impl.findRepoIDByTrackerID(trackerID)
}

// Link implements tracker.CoverageLinkManager.
func (s *CoverageService) Link(trackerID, repoID int64) error {
	return s.impl.linkTracker(trackerID, repoID)
}

// Unlink implements tracker.CoverageLinkManager.
func (s *CoverageService) Unlink(trackerID int64) error {
	return s.impl.unlinkTracker(trackerID)
}
