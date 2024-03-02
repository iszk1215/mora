package server

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

var schema_repository_manager = `
CREATE TABLE IF NOT EXISTS scm (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    driver TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE
)`

type (
	storableRepositoryManager struct {
		ID     int64  `db:"id"`
		Driver string `db:"driver"`
		URL    string `db:"url"`
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
		log.Err(err).Msg("")
		return err
	}
	return nil
}

func (s *repositoryManagerStoreImpl) FindURL(url string) (int64, string, error) {
	rows := []storableRepositoryManager{}
	err := s.db.Select(&rows, "SELECT id, driver FROM scm WHERE url = ?", url)
	if err != nil {
		return 0, "", err
	}

	if len(rows) == 0 {
		return -1, "", nil
	}

	return rows[0].ID, rows[0].Driver, nil
}

func (s *repositoryManagerStoreImpl) Insert(driver string, url string) (int64, error) {
	query := "INSERT INTO scm (driver, url) values($1, $2)"
	res, err := s.db.Exec(query, driver, url)
	if err != nil {
		return -1, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return -1, err
	}

	return id, nil
}
