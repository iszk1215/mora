package coverage

import (
	"net/http"

	"github.com/jmoiron/sqlx"
)

type (
	CoverageService struct {
		handler *CoverageHandler
	}
)

func NewCoverageService(db *sqlx.DB) (*CoverageService, error) {

	store := NewCoverageStore(db)
	if err := store.Init(); err != nil {
		return nil, err
	}

	return &CoverageService{handler: newCoverageHandler(store)}, nil
}

func (s *CoverageService) Handler() http.Handler {
	return s.handler.Handler()
}

