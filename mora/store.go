package mora

import (
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

var schema = `
CREATE TABLE IF NOT EXISTS coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL,
    revision TEXT NOT NULL,
    time DATETIME NOT NULL,
    raw TEXT NOT NULL
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

func (s *JSONStore) Store(cov Coverage, raw string) error {
	s.Lock()
	defer s.Unlock()

	rows := []int{}
	err := s.db.Select(&rows, "SELECT id FROM coverage WHERE url = $1 and revision = $2", cov.RepoURL(), cov.Revision())
	if err != nil {
		return err
	}

	if len(rows) > 1 {
		return fmt.Errorf(
			"multiple records in store found for url=%s and revision=%s",
			cov.RepoURL(), cov.Revision())
	}

	if len(rows) == 0 { // insert
		log.Print("Insert")
		_, err = s.db.Exec(
			"INSERT INTO coverage (url, revision, time, raw) VALUES ($1, $2, $3, $4)",
			cov.RepoURL(), cov.Revision(), cov.Time(), raw)
	} else { // update
		log.Print("Update")
		_, err = s.db.Exec(
			"UPDATE coverage SET raw = $1 WHERE url = $2 and revision = $3",
			raw, cov.RepoURL(), cov.Revision())
	}
	return err
}

func (s *JSONStore) Scan() ([]string, error) {
	rows := []string{}
	err := s.db.Select(&rows, "SELECT raw FROM coverage")
	return rows, err
}
