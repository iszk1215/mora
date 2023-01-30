package server

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/elliotchance/pie/v2"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

var schema = `
CREATE TABLE IF NOT EXISTS coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id INTEGER,
    url TEXT NOT NULL,
    revision TEXT NOT NULL,
    time DATETIME NOT NULL,
    contents TEXT NOT NULL
)`

type (
	ScanedCoverage struct {
		ID       int64     `db:"id"`
		RepoID   int64     `db:"repo_id"`
		RepoURL  string    `db:"url"`
		Revision string    `db:"revision"`
		Time     time.Time `db:"time"`
		Contents string    `db:"contents"`
	}

	coverageStoreImpl struct {
		db *sqlx.DB
		sync.Mutex
	}
)

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

func NewCoverageStore(db *sqlx.DB) *coverageStoreImpl {
	return &coverageStoreImpl{db: db}
}

func (s *coverageStoreImpl) Put(cov *Coverage) error {
	var requests []*CoverageEntryUploadRequest
	for _, e := range cov.Entries {
		requests = append(requests,
			&CoverageEntryUploadRequest{
				Name:     e.Name,
				Hits:     e.Hits,
				Lines:    e.Lines,
				Profiles: pie.Values(e.Profiles),
			})
	}

	contents, err := json.Marshal(requests)
	if err != nil {
		return err
	}

	/*
		scaned := ScanedCoverage{
			RepoURL:  cov.RepoURL,
			Revision: cov.Revision,
			Time:     cov.Timestamp,
			Contents: string(contents),
		}
	*/

	s.Lock()
	defer s.Unlock()

	rows := []int{}
	err = s.db.Select(&rows,
		"SELECT id FROM coverage WHERE repo_id = $1 and revision = $2",
		cov.RepoID, cov.Revision)
	if err != nil {
		return err
	}

	if len(rows) > 1 {
		return fmt.Errorf(
			"multiple records in store found for url=%s and revision=%s",
			cov.RepoURL, cov.Revision)
	}

	if len(rows) == 0 { // insert
		log.Print("Insert")
		res, err := s.db.Exec(
			"INSERT INTO coverage (repo_id, url, revision, time, contents) VALUES ($1, $2, $3, $4, $5)",
			cov.RepoID, cov.RepoURL, cov.Revision, cov.Timestamp, contents)
		if err != nil {
			return err
		}

		cov.ID, err = res.LastInsertId()
		return err
	} else { // update
		log.Print("Update")
		_, err = s.db.Exec(
			"UPDATE coverage SET contents = $1 WHERE repo_id = $2 and revision = $3",
			contents, cov.RepoID, cov.Revision)
		return err
	}
}

func parseScanedCoverage(record ScanedCoverage) (*Coverage, error) {
	entries, err := parseScanedCoverageContents(record.Contents)
	if err != nil {
		return nil, err
	}

	cov := &Coverage{}
	cov.ID = record.ID
	cov.RepoID = record.RepoID
	cov.RepoURL = record.RepoURL
	cov.Revision = record.Revision
	cov.Entries = entries
	cov.Timestamp = record.Time

	return cov, nil
}

func (s *coverageStoreImpl) Scan() ([]*Coverage, error) {
	rows := []ScanedCoverage{}
	err := s.db.Select(&rows, "SELECT id, repo_id, url, revision, time, contents FROM coverage")

	if err != nil {
		return nil, err
	}

	coverages := []*Coverage{}
	for _, record := range rows {
		cov, err := parseScanedCoverage(record)
		if err != nil {
			return nil, err
		}

		coverages = append(coverages, cov)
	}

	return coverages, nil
}
