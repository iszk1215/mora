package coverage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/iszk1215/mora/core"
	"github.com/iszk1215/mora/coverage/profile"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var schema = `
CREATE TABLE IF NOT EXISTS coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tracker_id INTEGER NOT NULL REFERENCES tracker(id) ON DELETE CASCADE,
    revision TEXT NOT NULL,
    time DATETIME NOT NULL,
    contents TEXT NOT NULL,
    UNIQUE(tracker_id, revision)
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
);

CREATE TABLE IF NOT EXISTS tracker_coverage (
    tracker_id INTEGER PRIMARY KEY,
    scm INTEGER NOT NULL,
    namespace TEXT NOT NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,
    FOREIGN KEY (tracker_id) REFERENCES tracker(id) ON DELETE CASCADE
);`

// schemaTrackerCoverage is the standalone tracker_coverage DDL used to recreate
// the table when migrating from the previous schema that linked repositories.
const schemaTrackerCoverage = `
CREATE TABLE IF NOT EXISTS tracker_coverage (
    tracker_id INTEGER PRIMARY KEY,
    scm INTEGER NOT NULL,
    namespace TEXT NOT NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,
    FOREIGN KEY (tracker_id) REFERENCES tracker(id) ON DELETE CASCADE
);`

// schemaCoverage is the standalone coverage DDL used when rebuilding the table
// to add the tracker foreign key during migration.
const schemaCoverage = `
CREATE TABLE IF NOT EXISTS coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tracker_id INTEGER NOT NULL REFERENCES tracker(id) ON DELETE CASCADE,
    revision TEXT NOT NULL,
    time DATETIME NOT NULL,
    contents TEXT NOT NULL,
    UNIQUE(tracker_id, revision)
);`

type (
	storableCoverage struct {
		ID        int64     `db:"id"`
		TrackerID int64     `db:"tracker_id"`
		Revision  string    `db:"revision"`
		Time      time.Time `db:"time"`
	}

	trackerCoverageRow struct {
		TrackerID int64  `db:"tracker_id"`
		Scm       int64  `db:"scm"`
		Namespace string `db:"namespace"`
		Name      string `db:"name"`
		URL       string `db:"url"`
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

func newCoverageStoreImpl(db *sqlx.DB) *coverageStoreImpl {
	query := "SELECT id, tracker_id, revision, time FROM coverage"
	return &coverageStoreImpl{db: db, selectQuery: query}
}

func NewCoverageStore(db *sqlx.DB) CoverageStore {
	return newCoverageStoreImpl(db)
}

func (s *coverageStoreImpl) Init() error {
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("coverage Init schema: %w", err)
	}

	if err := s.migrate(); err != nil {
		return fmt.Errorf("coverage Init migrate: %w", err)
	}

	if err := s.cleanupOrphanedCoverageTrackers(); err != nil {
		return fmt.Errorf("coverage Init cleanup: %w", err)
	}

	return nil
}

// migrate upgrades databases created by older versions of the schema. It
// rewrites the coverage table to be keyed by tracker_id (instead of repo_id)
// and recreates tracker_coverage as a self-contained repository description.
// It is a no-op for databases that already use the current schema.
func (s *coverageStoreImpl) migrate() error {
	if err := s.migrateCoverage(); err != nil {
		return err
	}
	return s.migrateTrackerCoverage()
}

// migrateCoverage rebuilds the coverage table when it still has a repo_id
// column: the column is renamed to tracker_id and the table is rebuilt so
// tracker_id carries a foreign key to tracker(id) ON DELETE CASCADE. Rows
// linked through the old tracker_coverage table keep their real tracker id;
// unlinked rows are marked with a negative repository id that
// MigrateCoverageTrackers resolves to a freshly created tracker at startup.
// Foreign key enforcement is temporarily disabled because the tracker table
// may not have been created yet at this point.
func (s *coverageStoreImpl) migrateCoverage() error {
	if !s.hasColumn("coverage", "repo_id") {
		return nil
	}

	prev, err := s.foreignKeys()
	if err != nil {
		return fmt.Errorf("migrateCoverage read foreign_keys: %w", err)
	}
	_, _ = s.db.Exec("PRAGMA foreign_keys = OFF")
	defer s.setForeignKeys(prev)

	if _, err := s.db.Exec("ALTER TABLE coverage RENAME COLUMN repo_id TO tracker_id"); err != nil {
		return fmt.Errorf("migrateCoverage rename column: %w", err)
	}

	// Rebuild the table so tracker_id gets the FK and the unique index is
	// recreated under the new column name.
	if _, err := s.db.Exec("ALTER TABLE coverage RENAME TO coverage_old"); err != nil {
		return fmt.Errorf("migrateCoverage rename old: %w", err)
	}
	if _, err := s.db.Exec(schemaCoverage); err != nil {
		return fmt.Errorf("migrateCoverage create: %w", err)
	}

	// Rows that still have a link in the old tracker_coverage table are copied
	// with their real tracker id. Rows without a link are marked with a negative
	// repository id so MigrateCoverageTrackers can resolve them to the tracker
	// created for the repository at startup; tracker ids are always positive,
	// so the negative marker can never collide with a real tracker id.
	copyQuery := `
		INSERT INTO coverage (id, tracker_id, revision, time, contents)
		SELECT id, -tracker_id, revision, time, contents FROM coverage_old`
	if s.hasColumn("tracker_coverage", "repo_id") {
		copyQuery = `
			INSERT INTO coverage (id, tracker_id, revision, time, contents)
			SELECT c.id,
			       COALESCE((SELECT tc.tracker_id FROM tracker_coverage tc
			                 WHERE tc.repo_id = c.tracker_id LIMIT 1), -c.tracker_id),
			       c.revision, c.time, c.contents
			FROM coverage_old c`
	}
	if _, err := s.db.Exec(copyQuery); err != nil {
		return fmt.Errorf("migrateCoverage copy: %w", err)
	}
	if _, err := s.db.Exec("DROP TABLE coverage_old"); err != nil {
		return fmt.Errorf("migrateCoverage drop old: %w", err)
	}

	return nil
}

// migrateTrackerCoverage replaces the old tracker_coverage table (which linked
// tracker_id to a repository) with the current schema that stores the scm,
// namespace, name and url directly. Existing links are carried over.
func (s *coverageStoreImpl) migrateTrackerCoverage() error {
	if !s.hasColumn("tracker_coverage", "repo_id") {
		return nil
	}

	var links []trackerCoverageRow
	if s.tableExists("repository") {
		if err := s.db.Select(&links, `
			SELECT tc.tracker_id, r.scm, r.namespace, r.name, r.url
			FROM tracker_coverage tc
			JOIN repository r ON r.id = tc.repo_id`); err != nil {
			return fmt.Errorf("migrateTrackerCoverage select old links: %w", err)
		}
	}

	prev, err := s.foreignKeys()
	if err != nil {
		return fmt.Errorf("migrateTrackerCoverage read foreign_keys: %w", err)
	}
	_, _ = s.db.Exec("PRAGMA foreign_keys = OFF")
	defer s.setForeignKeys(prev)

	if _, err := s.db.Exec("DROP TABLE tracker_coverage"); err != nil {
		return fmt.Errorf("migrateTrackerCoverage drop: %w", err)
	}
	if _, err := s.db.Exec(schemaTrackerCoverage); err != nil {
		return fmt.Errorf("migrateTrackerCoverage create: %w", err)
	}

	for _, link := range links {
		if _, err := s.db.Exec(
			"INSERT INTO tracker_coverage (tracker_id, scm, namespace, name, url) VALUES (?, ?, ?, ?, ?)",
			link.TrackerID, link.Scm, link.Namespace, link.Name, link.URL); err != nil {
			return fmt.Errorf("migrateTrackerCoverage insert: %w", err)
		}
	}

	return nil
}

func (s *coverageStoreImpl) hasColumn(table, column string) bool {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull, pk int
		var dfltValue *string
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

func (s *coverageStoreImpl) tableExists(name string) bool {
	var count int
	if err := s.db.Get(&count,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name); err != nil {
		return false
	}
	return count > 0
}

func (s *coverageStoreImpl) foreignKeys() (bool, error) {
	var v int
	if err := s.db.Get(&v, "PRAGMA foreign_keys"); err != nil {
		return false, fmt.Errorf("read PRAGMA foreign_keys: %w", err)
	}
	return v != 0, nil
}

func (s *coverageStoreImpl) setForeignKeys(on bool) {
	v := 0
	if on {
		v = 1
	}
	_, _ = s.db.Exec(fmt.Sprintf("PRAGMA foreign_keys = %d", v))
}

// cleanupOrphanedCoverageTrackers removes tracker_coverage rows whose tracker
// no longer exists. It is skipped when the tracker table has not been created
// yet (e.g. coverage store initialization runs before the tracker service).
func (s *coverageStoreImpl) cleanupOrphanedCoverageTrackers() error {
	var count int
	err := s.db.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tracker'")
	if err != nil {
		return fmt.Errorf("cleanupOrphanedCoverageTrackers check tracker table: %w", err)
	}
	if count == 0 {
		return nil
	}

	_, err = s.db.Exec(`
		DELETE FROM tracker_coverage
		WHERE tracker_id NOT IN (SELECT id FROM tracker)
	`)
	if err != nil {
		return fmt.Errorf("cleanupOrphanedCoverageTrackers delete: %w", err)
	}
	return nil
}

// FindRepoByTrackerID returns the repository described by the tracker_coverage
// row linked to a tracker, or nil when no link exists.
func (s *coverageStoreImpl) FindRepoByTrackerID(trackerID int64) (*core.Repository, error) {
	var row trackerCoverageRow
	err := s.db.Get(&row,
		"SELECT tracker_id, scm, namespace, name, url FROM tracker_coverage WHERE tracker_id = ?",
		trackerID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("FindRepoByTrackerID select: %w", err)
	}
	return &core.Repository{
		Id:                row.TrackerID,
		RepositoryManager: row.Scm,
		Namespace:         row.Namespace,
		Name:              row.Name,
		Url:               row.URL,
	}, nil
}

// trackerIDByURL returns the tracker_id linked to a repository URL, or 0 when
// none is linked.
func (s *coverageStoreImpl) trackerIDByURL(url string) (int64, error) {
	var id int64
	err := s.db.Get(&id, "SELECT tracker_id FROM tracker_coverage WHERE url = ?", url)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("trackerIDByURL select: %w", err)
	}
	return id, nil
}

func (s *coverageStoreImpl) linkTracker(trackerID int64, repo core.Repository) error {
	_, err := s.db.Exec(
		"INSERT INTO tracker_coverage (tracker_id, scm, namespace, name, url) VALUES (?, ?, ?, ?, ?)",
		trackerID, repo.RepositoryManager, repo.Namespace, repo.Name, repo.Url)
	if err != nil {
		return fmt.Errorf("linkTracker insert: %w", err)
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
		TrackerID: row.TrackerID,
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

func (s *coverageStoreImpl) scanLite(trackerID int64) ([]*Coverage, error) {
	type row struct {
		ID         int64     `db:"id"`
		TrackerID  int64     `db:"tracker_id"`
		Revision   string    `db:"revision"`
		Time       time.Time `db:"time"`
		EntryID    *int64    `db:"entry_id"`
		EntryName  *string   `db:"entry_name"`
		EntryHits  *int      `db:"entry_hits"`
		EntryLines *int      `db:"entry_lines"`
	}

	rows := []row{}
	if err := s.db.Select(&rows, `
		SELECT c.id, c.tracker_id, c.revision, c.time,
		       e.id        AS entry_id,
		       e.name      AS entry_name,
		       e.hits      AS entry_hits,
		       e.lines     AS entry_lines
		FROM coverage c
		LEFT JOIN coverage_entry e ON e.coverage_id = c.id
		WHERE c.tracker_id = ?
		ORDER BY c.id, e.id`, trackerID); err != nil {
		return nil, fmt.Errorf("scanLite select: %w", err)
	}

	var coverages []*Coverage
	var current *Coverage
	for _, r := range rows {
		if current == nil || current.ID != r.ID {
			current = &Coverage{
				ID:        r.ID,
				TrackerID: r.TrackerID,
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

func (s *coverageStoreImpl) FindRevision(trackerID int64, revision string) (*Coverage, error) {
	return s.findOne(
		s.selectQuery+" WHERE tracker_id = ? and revision = ?", trackerID, revision)
}

func (s *coverageStoreImpl) List(trackerID int64) ([]*Coverage, error) {
	return s.scanLite(trackerID)
}

func (s *coverageStoreImpl) Put(cov *Coverage) (int64, error) {
	contents, err := json.Marshal(cov.Entries)
	if err != nil {
		return 0, fmt.Errorf("Put marshal entries: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO coverage (tracker_id, revision, time, contents)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(tracker_id, revision) DO UPDATE SET contents = ?, time = ?`,
		cov.TrackerID, cov.Revision, cov.Timestamp, contents, contents, cov.Timestamp)
	if err != nil {
		return 0, fmt.Errorf("Put insert coverage: %w", err)
	}

	var id int64
	err = s.db.Get(&id,
		"SELECT id FROM coverage WHERE tracker_id = ? AND revision = ?",
		cov.TrackerID, cov.Revision)
	if err != nil {
		return 0, fmt.Errorf("Put select coverage id: %w", err)
	}

	if err := s.replaceEntries(id, cov.Entries); err != nil {
		return 0, err
	}

	s.touchLinkedTracker(cov.TrackerID)
	return id, nil
}

// touchLinkedTracker updates the last_updated_at of the coverage tracker that
// owns the coverage. It is a no-op when the tracker table has not been created
// yet (e.g. coverage store initialization runs before the tracker service).
func (s *coverageStoreImpl) touchLinkedTracker(trackerID int64) {
	if !s.tableExists("tracker") {
		return
	}
	_, _ = s.db.Exec(
		`UPDATE tracker SET last_updated_at = ? WHERE id = ?`,
		time.Now(), trackerID)
}

func (s *coverageStoreImpl) Timeline(trackerID int64, limit int) (map[string][]CoverageTimelinePoint, error) {
	type row struct {
		Time  time.Time `db:"time"`
		Name  string    `db:"name"`
		Hits  int       `db:"hits"`
		Lines int       `db:"lines"`
	}

	query := `
		SELECT c.time, e.name, e.hits, e.lines
		FROM coverage c
		JOIN coverage_entry e ON e.coverage_id = c.id
		WHERE c.tracker_id = ?
		ORDER BY c.time DESC`

	var rows []row
	if err := s.db.Select(&rows, query, trackerID); err != nil {
		return nil, fmt.Errorf("Timeline select: %w", err)
	}

	result := make(map[string][]CoverageTimelinePoint)
	counts := make(map[string]int)
	for _, r := range rows {
		if limit > 0 && counts[r.Name] >= limit {
			continue
		}
		pct := 0.0
		if r.Lines > 0 {
			pct = float64(r.Hits) / float64(r.Lines) * 100
		}
		result[r.Name] = append(result[r.Name], CoverageTimelinePoint{Time: r.Time, Value: pct})
		counts[r.Name]++
	}

	// Reverse each entry to ascending time order
	for name, points := range result {
		for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
			points[i], points[j] = points[j], points[i]
		}
		result[name] = points
	}

	return result, nil
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
