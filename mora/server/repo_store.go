package server

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

var schema_repo = `
CREATE TABLE IF NOT EXISTS repository (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    namespace TEXT NOT NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL
)`

type (
	storableRepository struct {
		ID        int64  `db:"id"`
		Namespace string `db:"namespace"`
		Name      string `db:"name"`
		URL       string `db:"url"`
	}

	RepositoryStore interface {
		Init() error
		Scan() ([]Repository, error)
	}

	RepositoryStoreImpl struct {
		db *sqlx.DB
	}
)

func NewRepositoryStore(db *sqlx.DB) RepositoryStore {
	return &RepositoryStoreImpl{db}
}

func (s *RepositoryStoreImpl) Init() error {
	_, err := s.db.Exec(schema_repo)
	if err != nil {
		log.Err(err).Msg("")
		return err
	}
	return nil
}

func (s *RepositoryStoreImpl) Scan() ([]Repository, error) {
	rows := []storableRepository{}
	err := s.db.Select(&rows, "SELECT id, name, namespace, url FROM repository")

	if err != nil {
		return nil, err
	}

	repos := []Repository{}
	for _, record := range rows {
		repo := Repository{
			ID:        record.ID,
			Namespace: record.Namespace,
			Name:      record.Name,
			Link:      record.URL,
		}
		repos = append(repos, repo)
	}

	return repos, nil
}
