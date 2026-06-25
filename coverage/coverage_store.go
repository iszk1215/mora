package coverage

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/iszk1215/mora/coverage/profile"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var schema = `
CREATE TABLE IF NOT EXISTS coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id INTEGER NOT NULL,
    revision TEXT NOT NULL,
    time DATETIME NOT NULL,
    contents TEXT NOT NULL,
    UNIQUE(repo_id, revision)
);

CREATE TABLE IF NOT EXISTS coverage_entry (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    coverage_id INTEGER NOT NULL REFERENCES coverage(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    hits INTEGER NOT NULL DEFAULT 0,
    lines INTEGER NOT NULL DEFAULT 0,
    UNIQUE(coverage_id, name)
);

CREATE TABLE IF NOT EXISTS coverage_block (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER NOT NULL REFERENCES coverage_entry(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    hits INTEGER NOT NULL DEFAULT 0,
    lines INTEGER NOT NULL DEFAULT 0,
    blocks TEXT NOT NULL,
    UNIQUE(entry_id, filename)
);`

type (
	storableCoverage struct {
		ID       int64     `db:"id"`
		RepoID   int64     `db:"repo_id"`
		Revision string    `db:"revision"`
		Time     time.Time `db:"time"`
	}

	storableEntry struct {
		ID         int64  `db:"id"`
		CoverageID int64  `db:"coverage_id"`
		Name       string `db:"name"`
		Hits       int    `db:"hits"`
		Lines      int    `db:"lines"`
	}

	storableBlock struct {
		ID       int64  `db:"id"`
		EntryID  int64  `db:"entry_id"`
		Filename string `db:"filename"`
		Hits     int    `db:"hits"`
		Lines    int    `db:"lines"`
		Blocks   string `db:"blocks"`
	}

	coverageStoreImpl struct {
		db *sqlx.DB

		selectQuery string
	}
)

func NewCoverageStore(db *sqlx.DB) CoverageStore {
	query := "SELECT id, repo_id, revision, time FROM coverage"
	return &coverageStoreImpl{db: db, selectQuery: query}
}

func (s *coverageStoreImpl) Init() error {
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("coverage Init schema: %w", err)
	}

	_, err = s.db.Exec(
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_coverage_repo_revision ON coverage(repo_id, revision)")
	if err != nil {
		return fmt.Errorf("coverage Init index: %w", err)
	}

	return s.migrateIfNeeded()
}

func (s *coverageStoreImpl) migrateIfNeeded() error {
	var entryCount int
	if err := s.db.Get(&entryCount, "SELECT COUNT(*) FROM coverage_entry"); err != nil {
		return fmt.Errorf("migrateIfNeeded count entries: %w", err)
	}
	if entryCount > 0 {
		return nil
	}

	var coverageCount int
	if err := s.db.Get(&coverageCount, "SELECT COUNT(*) FROM coverage"); err != nil {
		return fmt.Errorf("migrateIfNeeded count coverages: %w", err)
	}
	if coverageCount == 0 {
		return nil
	}

	return s.migrateFromContents()
}

func (s *coverageStoreImpl) migrateFromContents() error {
	var rows []struct {
		ID       int64     `db:"id"`
		Contents string    `db:"contents"`
	}
	if err := s.db.Select(&rows, "SELECT id, contents FROM coverage"); err != nil {
		return fmt.Errorf("migrateFromContents select: %w", err)
	}

	for _, row := range rows {
		var entries []*CoverageEntry
		if err := json.Unmarshal([]byte(row.Contents), &entries); err != nil {
			return fmt.Errorf("migrate: failed to parse contents for coverage %d: %w", row.ID, err)
		}

		if err := s.replaceEntries(row.ID, entries); err != nil {
			return fmt.Errorf("migrate: failed to insert entries for coverage %d: %w", row.ID, err)
		}
	}

	return nil
}

func (s *coverageStoreImpl) loadCoverage(row storableCoverage) (*Coverage, error) {
	var entryRows []storableEntry
	if err := s.db.Select(&entryRows,
		"SELECT id, coverage_id, name, hits, lines FROM coverage_entry WHERE coverage_id = ? ORDER BY id",
		row.ID); err != nil {
		return nil, fmt.Errorf("loadCoverage select entries: %w", err)
	}

	var blockRows []storableBlock
	if err := s.db.Select(&blockRows,
		`SELECT b.id, b.entry_id, b.filename, b.hits, b.lines, b.blocks
		 FROM coverage_block b
		 JOIN coverage_entry e ON e.id = b.entry_id
		 WHERE e.coverage_id = ?`, row.ID); err != nil {
		return nil, fmt.Errorf("loadCoverage select blocks: %w", err)
	}

	blocksByEntry := make(map[int64][]storableBlock)
	for _, br := range blockRows {
		blocksByEntry[br.EntryID] = append(blocksByEntry[br.EntryID], br)
	}

	entries := make([]*CoverageEntry, len(entryRows))
	for i, er := range entryRows {
		var profiles map[string]*profile.Profile
		for _, br := range blocksByEntry[er.ID] {
			var blocks [][]int
			if err := json.Unmarshal([]byte(br.Blocks), &blocks); err != nil {
				return nil, fmt.Errorf("loadCoverage unmarshal blocks: %w", err)
			}
			if profiles == nil {
				profiles = make(map[string]*profile.Profile)
			}
			profiles[br.Filename] = &profile.Profile{
				FileName: br.Filename,
				Hits:     br.Hits,
				Lines:    br.Lines,
				Blocks:   blocks,
			}
		}

		entries[i] = &CoverageEntry{
			Name:     er.Name,
			Hits:     er.Hits,
			Lines:    er.Lines,
			Profiles: profiles,
		}
	}

	return &Coverage{
		ID:        row.ID,
		RepoID:    row.RepoID,
		Revision:  row.Revision,
		Timestamp: row.Time,
		Entries:   entries,
	}, nil
}

func (s *coverageStoreImpl) scanFull(query string, params ...interface{}) ([]*Coverage, error) {
	rows := []storableCoverage{}
	if err := s.db.Select(&rows, query, params...); err != nil {
		return nil, fmt.Errorf("scanFull select: %w", err)
	}

	coverages := make([]*Coverage, len(rows))
	for i, record := range rows {
		cov, err := s.loadCoverage(record)
		if err != nil {
			return nil, fmt.Errorf("scanFull loadCoverage: %w", err)
		}
		coverages[i] = cov
	}

	return coverages, nil
}

func (s *coverageStoreImpl) scanLite(repoID int64) ([]*Coverage, error) {
	type row struct {
		ID         int64     `db:"id"`
		RepoID     int64     `db:"repo_id"`
		Revision   string    `db:"revision"`
		Time       time.Time `db:"time"`
		EntryID    *int64    `db:"entry_id"`
		EntryName  *string   `db:"entry_name"`
		EntryHits  *int      `db:"entry_hits"`
		EntryLines *int      `db:"entry_lines"`
	}

	rows := []row{}
	if err := s.db.Select(&rows, `
		SELECT c.id, c.repo_id, c.revision, c.time,
		       e.id        AS entry_id,
		       e.name      AS entry_name,
		       e.hits      AS entry_hits,
		       e.lines     AS entry_lines
		FROM coverage c
		LEFT JOIN coverage_entry e ON e.coverage_id = c.id
		WHERE c.repo_id = ?
		ORDER BY c.id, e.id`, repoID); err != nil {
		return nil, fmt.Errorf("scanLite select: %w", err)
	}

	var coverages []*Coverage
	var current *Coverage
	for _, r := range rows {
		if current == nil || current.ID != r.ID {
			current = &Coverage{
				ID:        r.ID,
				RepoID:    r.RepoID,
				Revision:  r.Revision,
				Timestamp: r.Time,
			}
			coverages = append(coverages, current)
		}
		if r.EntryID != nil {
			current.Entries = append(current.Entries, &CoverageEntry{
				Name:  *r.EntryName,
				Hits:  *r.EntryHits,
				Lines: *r.EntryLines,
			})
		}
	}
	if coverages == nil {
		coverages = []*Coverage{}
	}
	return coverages, nil
}

func (s *coverageStoreImpl) findOne(query string, params ...interface{}) (*Coverage, error) {
	coverages, err := s.scanFull(query, params...)
	if err != nil {
		return nil, fmt.Errorf("findOne scanFull: %w", err)
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
	return s.scanLite(repo_id)
}

func (s *coverageStoreImpl) Put(cov *Coverage) error {
	contents, err := json.Marshal(cov.Entries)
	if err != nil {
		return fmt.Errorf("Put marshal entries: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO coverage (repo_id, revision, time, contents)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(repo_id, revision) DO UPDATE SET contents = ?, time = ?`,
		cov.RepoID, cov.Revision, cov.Timestamp, contents, contents, cov.Timestamp)
	if err != nil {
		return fmt.Errorf("Put insert coverage: %w", err)
	}

	err = s.db.Get(&cov.ID,
		"SELECT id FROM coverage WHERE repo_id = ? AND revision = ?",
		cov.RepoID, cov.Revision)
	if err != nil {
		return fmt.Errorf("Put select coverage id: %w", err)
	}

	return s.replaceEntries(cov.ID, cov.Entries)
}

func (s *coverageStoreImpl) replaceEntries(coverageID int64, entries []*CoverageEntry) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("replaceEntries begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		"DELETE FROM coverage_block WHERE entry_id IN (SELECT id FROM coverage_entry WHERE coverage_id = ?)",
		coverageID); err != nil {
		return fmt.Errorf("replaceEntries delete blocks: %w", err)
	}
	if _, err := tx.Exec(
		"DELETE FROM coverage_entry WHERE coverage_id = ?", coverageID); err != nil {
		return fmt.Errorf("replaceEntries delete entries: %w", err)
	}

	for _, entry := range entries {
		res, err := tx.Exec(
			"INSERT INTO coverage_entry (coverage_id, name, hits, lines) VALUES (?, ?, ?, ?)",
			coverageID, entry.Name, entry.Hits, entry.Lines)
		if err != nil {
			return fmt.Errorf("replaceEntries insert entry %q: %w", entry.Name, err)
		}
		entryID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("replaceEntries LastInsertId for %q: %w", entry.Name, err)
		}

		for _, prof := range entry.Profiles {
			blocksJSON, err := json.Marshal(prof.Blocks)
			if err != nil {
				return fmt.Errorf("replaceEntries marshal blocks for %q: %w", prof.FileName, err)
			}
			if _, err := tx.Exec(
				"INSERT INTO coverage_block (entry_id, filename, hits, lines, blocks) VALUES (?, ?, ?, ?, ?)",
				entryID, prof.FileName, prof.Hits, prof.Lines, string(blocksJSON)); err != nil {
				return fmt.Errorf("replaceEntries insert block for %q: %w", prof.FileName, err)
			}
		}
	}

	return tx.Commit()
}
