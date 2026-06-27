package track

import (
	"fmt"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Service struct {
	store  *trackStore
	apiKey string
}

func NewService(db *sqlx.DB, apiKey string) (*Service, error) {
	log.Print("track.NewService")
	store := newTrackStore(db)
	err := store.initialize()
	if err != nil {
		return nil, fmt.Errorf("track store initialize: %w", err)
	}
	return &Service{store: store, apiKey: apiKey}, nil
}

func (s *Service) Handler() http.Handler {
	return newHandler(s.store, s.apiKey)
}
