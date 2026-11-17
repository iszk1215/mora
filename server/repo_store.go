package server

import (
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

var schema_repo = `
CREATE TABLE IF NOT EXISTS repository (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scm INTEGER NOT NULL,
    namespace TEXT NOT NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,
    external_id TEXT,
    UNIQUE(scm, namespace, name)
)`

type (
	storableRepository struct {
		ID                int64  `db:"id"`
		ExternalID        string `db:"external_id"`
		RepositoryManager int64  `db:"scm"`
		Namespace         string `db:"namespace"`
		Name              string `db:"name"`
		URL               string `db:"url"`
	}

	repositoryStoreImpl struct {
		db *sqlx.DB
	}
)

func NewRepositoryStore(db *sqlx.DB) RepositoryStore {
	return &repositoryStoreImpl{db}
}

func (s *repositoryStoreImpl) Init() error {
	_, err := s.db.Exec(schema_repo)
	if err != nil {
		log.Err(err).Msg("repo store init failed")
		return fmt.Errorf("repo Init: %w", err)
	}
	if err := s.migrate(); err != nil {
		log.Err(err).Msg("repo store migration failed")
		return fmt.Errorf("repo Init: %w", err)
	}
	return nil
}

func (s *repositoryStoreImpl) migrate() error {
	if !hasColumn(s.db, "repository", "external_id") {
		if _, err := s.db.Exec("ALTER TABLE repository ADD COLUMN external_id TEXT"); err != nil {
			return fmt.Errorf("migrate repository external_id column: %w", err)
		}
	}

	var ids []int64
	if err := s.db.Select(&ids,
		"SELECT id FROM repository WHERE external_id IS NULL OR external_id = '' ORDER BY id"); err != nil {
		return fmt.Errorf("migrate repository external_id select: %w", err)
	}

	for _, id := range ids {
		eid, err := newRepositoryExternalID()
		if err != nil {
			return fmt.Errorf("migrate repository external_id generate: %w", err)
		}
		if _, err := s.db.Exec("UPDATE repository SET external_id = ? WHERE id = ?",
			bareRepositoryExternalID(eid), id); err != nil {
			return fmt.Errorf("migrate repository external_id update: %w", err)
		}
	}

	if _, err := s.db.Exec(
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_repository_external_id ON repository(external_id)"); err != nil {
		return fmt.Errorf("migrate idx_repository_external_id: %w", err)
	}
	return nil
}

func toRepo(from storableRepository) Repository {
	return Repository{
		Id:                from.ID,
		ExternalID:        repositoryExternalIDFromBare(from.ExternalID),
		RepositoryManager: from.RepositoryManager,
		Namespace:         from.Namespace,
		Name:              from.Name,
		Url:               from.URL,
	}
}

func (s *repositoryStoreImpl) findOne(query string, params ...interface{}) (Repository, error) {
	rows := []storableRepository{}
	err := s.db.Select(&rows, query, params...)
	if err != nil {
		return Repository{}, fmt.Errorf("repo Find: %w", err)
	}

	if len(rows) == 0 {
		return Repository{}, errors.New("no repo")
	}

	return toRepo(rows[0]), nil
}

func (s *repositoryStoreImpl) Find(id int64) (Repository, error) {
	query := "SELECT id, external_id, scm, namespace, name, url FROM repository WHERE id = ?"
	return s.findOne(query, id)
}

func (s *repositoryStoreImpl) FindByExternalID(externalID string) (Repository, error) {
	parsed, err := parseRepositoryExternalID(externalID)
	if err != nil {
		return Repository{}, err
	}
	query := "SELECT id, external_id, scm, namespace, name, url FROM repository WHERE external_id = ?"
	return s.findOne(query, bareRepositoryExternalID(parsed))
}

func (s *repositoryStoreImpl) FindURL(url string) (Repository, error) {
	query := "SELECT id, external_id, scm, namespace, name, url FROM repository WHERE url = ?"
	return s.findOne(query, url)
}

func (s *repositoryStoreImpl) Put(repo *Repository) error {
	externalID, err := newRepositoryExternalID()
	if err != nil {
		return fmt.Errorf("repo Put external id: %w", err)
	}

	res, err := s.db.Exec(
		"INSERT INTO repository (scm, namespace, name, url, external_id) VALUES (?, ?, ?, ?, ?)",
		repo.RepositoryManager, repo.Namespace, repo.Name, repo.Url,
		bareRepositoryExternalID(externalID))
	if err != nil {
		return fmt.Errorf("repo Insert: %w", err)
	}

	repo.Id, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repo Put LastInsertId: %w", err)
	}
	repo.ExternalID = externalID
	return nil
}

func (s *repositoryStoreImpl) ListAll() ([]Repository, error) {
	rows := []storableRepository{}
	err := s.db.Select(&rows, "SELECT id, external_id, scm, name, namespace, url FROM repository")

	if err != nil {
		return nil, fmt.Errorf("repo ListAll: %w", err)
	}

	repos := []Repository{}
	for _, record := range rows {
		repos = append(repos, toRepo(record))
	}

	return repos, nil
}
