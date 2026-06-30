package tracker

import (
	"fmt"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Service struct {
	store *trackerStore
}

func NewService(db *sqlx.DB) (*Service, error) {
	log.Print("tracker.NewService")
	store := newTrackerStore(db)
	err := store.initialize()
	if err != nil {
		return nil, fmt.Errorf("tracker store initialize: %w", err)
	}
	return &Service{store: store}, nil
}

func (s *Service) Handler() http.Handler {
	return newHandler(s.store)
}
