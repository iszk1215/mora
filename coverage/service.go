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
	}
)

func NewCoverageService(db *sqlx.DB) (*CoverageService, error) {

	store := NewCoverageStore(db)
	if err := store.Init(); err != nil {
		return nil, fmt.Errorf("coverage store Init: %w", err)
	}

	return &CoverageService{handler: newCoverageHandler(store), store: store}, nil
}

func (s *CoverageService) Handler() http.Handler {
	return s.handler.Handler()
}

func (s *CoverageService) Store() CoverageStore {
	return s.store
}

