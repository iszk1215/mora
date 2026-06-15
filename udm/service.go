package udm

import (
	"fmt"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type (
	Service struct {
		store *udmStore
	}
)

func NewService(db *sqlx.DB) (*Service, error) {
	log.Print("udm.NewService")
	store := newUdmStore(db)
	err := store.initialize()
	if err != nil {
		return nil, fmt.Errorf("udm store initialize: %w", err)
	}
	return &Service{store: store}, nil
}

func (s *Service) Handler() http.Handler {
	return newHandler(s.store)
}
