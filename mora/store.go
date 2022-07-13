package mora

import (
	"sync"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

var schema = `
CREATE TABLE IF NOT EXISTS json (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL,
    revision TEXT NOT NULL,
    json TEXT NOT NULL
)`

type JSONStore struct {
	db   *sqlx.DB
	name string
	sync.Mutex
}

func Connect(filename string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite3", filename)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(schema)
	if err != nil {
		log.Err(err).Msg("")
		return nil, err
	}

	return db, nil
}

func NewJSONStore(db *sqlx.DB, name string) *JSONStore {
	return &JSONStore{db: db, name: name}
}

func (s *JSONStore) Store(cov Coverage, json string) error {
	s.Lock()
	defer s.Unlock()

	_, err := s.db.Exec(
		"INSERT INTO json (url, revision, json) VALUES ($1, $2, $3)", cov.RepoURL(), cov.Revision(), json)
	return err
}

func (s *JSONStore) Scan() ([]string, error) {
	rows := []string{}
	err := s.db.Select(&rows, "SELECT json FROM json")
	return rows, err
}
