package server

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

var schema = `
CREATE TABLE IF NOT EXISTS coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id INTEGER NOT NULL,
    revision TEXT NOT NULL,
    time DATETIME NOT NULL,
    contents TEXT NOT NULL,
    UNIQUE(repo_id, revision)
)`

type (
	storableCoverage struct {
		ID       int64     `db:"id"`
		RepoID   int64     `db:"repo_id"`
		Revision string    `db:"revision"`
		Time     time.Time `db:"time"`
		Contents string    `db:"contents"`
	}

	coverageStoreImpl struct {
		db *sqlx.DB
		sync.Mutex

		selectQuery string
	}
)

func NewCoverageStore(db *sqlx.DB) *coverageStoreImpl {
	query := "SELECT id, repo_id, revision, time, contents FROM coverage"
	return &coverageStoreImpl{db: db, selectQuery: query}
}

func (s *coverageStoreImpl) Init() error {
	_, err := s.db.Exec(schema)
	return err
}

/*
// contents is serialized []CoverageEntryUploadRequest
func parseStorableCoverageContents(contents string) ([]*CoverageEntry, error) {
	var req []*CoverageEntryUploadRequest

	err := json.Unmarshal([]byte(contents), &req)
	if err != nil {
		return nil, err
	}

	entries, err := parseCoverageEntryUploadRequests(req)
	if err != nil {
		return nil, err
	}

	return entries, nil
}
*/

func toCoverage(record storableCoverage) (*Coverage, error) {
	/*
		entries, err := parseStorableCoverageContents(record.Contents)
		if err != nil {
			return nil, err
		}
	*/
	var entries []*CoverageEntry
	err := json.Unmarshal([]byte(record.Contents), &entries)
	if err != nil {
		return nil, err
	}

	cov := &Coverage{}
	cov.ID = record.ID
	cov.RepoID = record.RepoID
	cov.Revision = record.Revision
	cov.Entries = entries
	cov.Timestamp = record.Time

	return cov, nil
}

func (s *coverageStoreImpl) scan(query string, params ...interface{}) ([]*Coverage, error) {
	log.Print("query=", query)

	rows := []storableCoverage{}
	err := s.db.Select(&rows, query, params...)
	if err != nil {
		return nil, err
	}

	coverages := []*Coverage{}
	for _, record := range rows {
		cov, err := toCoverage(record)
		if err != nil {
			return nil, err
		}

		coverages = append(coverages, cov)
	}

	return coverages, nil
}

func (s *coverageStoreImpl) findOne(query string, params ...interface{}) (*Coverage, error) {
	coverages, err := s.scan(query, params...)
	if err != nil {
		return nil, err
	}
	if len(coverages) == 0 {
		return nil, nil
	}
	return coverages[0], nil
}

func (s *coverageStoreImpl) Find(id int64) (*Coverage, error) {
	return s.findOne(s.selectQuery+" WHERE id = ?", id)
}

func (s *coverageStoreImpl) FindRevision(repoID int64, revision string) (*Coverage, error) {
	return s.findOne(
		s.selectQuery+" WHERE repo_id = ? and revision = ?", repoID, revision)
}

func (s *coverageStoreImpl) List(repo_id int64) ([]*Coverage, error) {
	return s.scan(s.selectQuery+" WHERE repo_id = ?", repo_id)
}

func (s *coverageStoreImpl) ListAll() ([]*Coverage, error) {
	return s.scan(s.selectQuery)
}

func (s *coverageStoreImpl) Put(cov *Coverage) error {
	/*
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
	*/

	contents, err := json.Marshal(cov.Entries)
	if err != nil {
		return err
	}
	log.Print(string(contents))

	s.Lock()
	defer s.Unlock()

	rows := []int{}
	err = s.db.Select(&rows,
		"SELECT id FROM coverage WHERE repo_id = $1 and revision = $2",
		cov.RepoID, cov.Revision)
	if err != nil {
		return err
	}

	if len(rows) == 0 { // insert
		log.Print("Insert")
		res, err := s.db.Exec(
			"INSERT INTO coverage (repo_id, revision, time, contents) VALUES ($1, $2, $3, $4)",
			cov.RepoID, cov.Revision, cov.Timestamp, contents)
		if err != nil {
			return err
		}

		cov.ID, err = res.LastInsertId()
		// log.Print("Assing id=", cov.ID)
		return err
	} else { // update
		log.Print("Update")
		_, err = s.db.Exec(
			"UPDATE coverage SET contents = $1 WHERE repo_id = $2 and revision = $3",
			contents, cov.RepoID, cov.Revision)
		return err
	}
}
