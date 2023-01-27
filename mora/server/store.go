package server

import (
	"fmt"
	"sync"
	"time"

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
    contents TEXT NOT NULL
)`

type ScanedCoverage struct {
	ID       int       `db:"id"`
	RepoURL  string    `db:"url"`
	Revision string    `db:"revision"`
	Time     time.Time `db:"time"`
	Contents string    `db:"contents"`
}

type CoverageStore interface {
	Put(ScanedCoverage) (int, error)
	Scan() ([]ScanedCoverage, error)
}

type CoverageStoreSQLX struct {
	db *sqlx.DB
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

func NewCoverageStore(db *sqlx.DB) *CoverageStoreSQLX {
	return &CoverageStoreSQLX{db: db}
}

func (s *CoverageStoreSQLX) getCoverageID(cov ScanedCoverage) (int, error) {
	rows := []int{}
	err := s.db.Select(&rows,
		"SELECT id FROM coverage WHERE url = $1 and revision = $2",
		cov.RepoURL, cov.Revision)
	if err != nil {
		return 0, err
	}

	if len(rows) > 1 {
		return 0, fmt.Errorf(
			"multiple records in store found for url=%s and revision=%s",
			cov.RepoURL, cov.Revision)
	}

	if len(rows) == 0 {
		return -1, nil
	}

	return rows[0], nil
}

func (s *CoverageStoreSQLX) Put(cov ScanedCoverage) (int, error) {
	s.Lock()
	defer s.Unlock()

	/*
		rows := []int{}
		err := s.db.Select(&rows,
			"SELECT id FROM coverage WHERE url = $1 and revision = $2",
			cov.RepoURL, cov.Revision)
		if err != nil {
			return 0, err
		}

		if len(rows) > 1 {
			return 0, fmt.Errorf(
				"multiple records in store found for url=%s and revision=%s",
				cov.RepoURL, cov.Revision)
		}
	*/
	id, err := s.getCoverageID(cov)
	if err != nil {
		return -1, err
	}

	if id == -1 { // insert
		log.Print("Insert")
		_, err = s.db.Exec(
			"INSERT INTO coverage (url, revision, time, contents) VALUES ($1, $2, $3, $4)",
			cov.RepoURL, cov.Revision, cov.Time, cov.Contents)
		if err != nil {
			return -1, err
		}

		id, err := s.getCoverageID(cov)
		if err != nil {
			return -1, err
		}
		return id, nil
	} else { // update
		log.Print("Update")
		_, err = s.db.Exec(
			"UPDATE coverage SET contents = $1 WHERE url = $2 and revision = $3",
			cov.Contents, cov.RepoURL, cov.Revision)
		return id, err
	}
}

func (s *CoverageStoreSQLX) Scan() ([]ScanedCoverage, error) {
	rows := []ScanedCoverage{}
	err := s.db.Select(&rows, "SELECT id, url, revision, time, contents FROM coverage")
	return rows, err
}
