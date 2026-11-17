package server

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

var schema_repository_manager = `
CREATE TABLE IF NOT EXISTS scm (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id TEXT,
    driver TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE
)`

type (
	storableRepositoryManager struct {
		ID         int64  `db:"id"`
		ExternalID string `db:"external_id"`
		Driver     string `db:"driver"`
		URL        string `db:"url"`
	}

	repositoryManagerStoreImpl struct {
		db *sqlx.DB
	}
)

func NewRepositoryManagerStore(db *sqlx.DB) RepositoryManagerStore {
	return &repositoryManagerStoreImpl{db}
}

func (s *repositoryManagerStoreImpl) Init() error {
	_, err := s.db.Exec(schema_repository_manager)
	if err != nil {
		log.Err(err).Msg("scm store init failed")
		return fmt.Errorf("scm Init: %w", err)
	}
	if err := s.migrate(); err != nil {
		log.Err(err).Msg("scm store migration failed")
		return fmt.Errorf("scm Init: %w", err)
	}
	return nil
}

func (s *repositoryManagerStoreImpl) migrate() error {
	if !hasColumn(s.db, "scm", "external_id") {
		if _, err := s.db.Exec("ALTER TABLE scm ADD COLUMN external_id TEXT"); err != nil {
			return fmt.Errorf("migrate scm external_id column: %w", err)
		}
	}

	var ids []int64
	if err := s.db.Select(&ids,
		"SELECT id FROM scm WHERE external_id IS NULL OR external_id = '' ORDER BY id"); err != nil {
		return fmt.Errorf("migrate scm external_id select: %w", err)
	}

	for _, id := range ids {
		eid, err := newSCMExternalID()
		if err != nil {
			return fmt.Errorf("migrate scm external_id generate: %w", err)
		}
		if _, err := s.db.Exec("UPDATE scm SET external_id = ? WHERE id = ?",
			bareExternalID(scmExternalIDPrefix, eid), id); err != nil {
			return fmt.Errorf("migrate scm external_id update: %w", err)
		}
	}

	if _, err := s.db.Exec(
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_scm_external_id ON scm(external_id)"); err != nil {
		return fmt.Errorf("migrate idx_scm_external_id: %w", err)
	}
	return nil
}

func (s *repositoryManagerStoreImpl) FindURL(url string) (int64, string, string, error) {
	rows := []storableRepositoryManager{}
	err := s.db.Select(&rows,
		"SELECT id, external_id, driver FROM scm WHERE url = ?", url)
	if err != nil {
		return 0, "", "", fmt.Errorf("scm FindURL: %w", err)
	}

	if len(rows) == 0 {
		return -1, "", "", nil
	}

	return rows[0].ID, scmExternalIDFromBare(rows[0].ExternalID), rows[0].Driver, nil
}

func (s *repositoryManagerStoreImpl) Insert(driver string, url string) (int64, string, error) {
	externalID, err := newSCMExternalID()
	if err != nil {
		return -1, "", fmt.Errorf("scm Insert external id: %w", err)
	}

	query := "INSERT INTO scm (driver, url, external_id) values(?, ?, ?)"
	res, err := s.db.Exec(query, driver, url, bareExternalID(scmExternalIDPrefix, externalID))
	if err != nil {
		return -1, "", fmt.Errorf("scm Insert: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return -1, "", fmt.Errorf("scm Insert LastInsertId: %w", err)
	}

	return id, externalID, nil
}
