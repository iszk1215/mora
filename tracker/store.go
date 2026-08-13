package tracker

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	errorTrackerNotFound = errors.New("no tracker found")
	errorSeriesNotFound  = errors.New("no series found")
)

const (
	// TypeTracker is the regular tracker type that manages its own series/values.
	TypeTracker = "tracker"
	// TypeCoverage is the coverage-type tracker, managed by the coverage package.
	TypeCoverage = "coverage"
)

var schemaTracker = `
CREATE TABLE IF NOT EXISTS tracker (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    visibility TEXT NOT NULL DEFAULT 'private',
    type TEXT NOT NULL DEFAULT 'tracker',
    chart_config TEXT NOT NULL DEFAULT '{}',
    owner_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL,
    last_updated_at DATETIME NOT NULL
)`

var schemaSeries = `
CREATE TABLE IF NOT EXISTS tracker_series (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tracker_id INTEGER NOT NULL REFERENCES tracker(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    data_type TEXT NOT NULL DEFAULT 'float',
    config TEXT NOT NULL DEFAULT '{}',
    UNIQUE(tracker_id, name)
)`

var schemaValue = `
CREATE TABLE IF NOT EXISTS tracker_value (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id INTEGER NOT NULL REFERENCES tracker_series(id) ON DELETE CASCADE,
    time DATETIME NOT NULL,
    value REAL NOT NULL,
    UNIQUE(series_id, time)
)`

var schemaMember = `
CREATE TABLE IF NOT EXISTS tracker_member (
    user_id  INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    tracker_id INTEGER NOT NULL REFERENCES tracker(id) ON DELETE CASCADE,
    role     TEXT NOT NULL DEFAULT 'editor',
    PRIMARY KEY (user_id, tracker_id)
)`

var schemaLike = `
CREATE TABLE IF NOT EXISTS tracker_like (
    user_id  INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    tracker_id INTEGER NOT NULL REFERENCES tracker(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, tracker_id)
)`

type (
	trackerStore struct {
		db *sqlx.DB
	}
)

// TrackerResponse is returned in tracker lists and includes user-specific flags.
type TrackerResponse struct {
	Id            int64     `json:"id"    db:"id"`
	Name          string    `json:"name"  db:"name"`
	Description   string    `json:"description" db:"description"`
	Body          string    `json:"body" db:"body"`
	Visibility    string    `json:"visibility"` // "public" | "private"
	Type          string    `json:"type"`
	ChartConfig   string    `json:"chart_config" db:"chart_config"`
	OwnerId       int64     `json:"owner_id" db:"owner_id"`
	OwnerName     string    `json:"owner_name" db:"owner_name"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	LastUpdatedAt time.Time `json:"last_updated_at" db:"last_updated_at"`
	Role          string    `json:"role"` // "" | "owner" | "editor"
	Liked         bool      `json:"liked"`
	LikeCount     int       `json:"like_count" db:"like_count"`
}

func newTrackerStore(db *sqlx.DB) *trackerStore {
	return &trackerStore{db: db}
}

// ----------------------------------------------------------------------
// Tracker

func (s *trackerStore) addTracker(tracker *TrackerModel, userID int64) error {
	if tracker.Visibility == "" {
		tracker.Visibility = "private"
	}
	if tracker.Type == "" {
		tracker.Type = TypeTracker
	}
	if tracker.ChartConfig == "" {
		tracker.ChartConfig = "{}"
	}
	now := time.Now()
	tracker.OwnerId = userID
	tracker.CreatedAt = now
	tracker.LastUpdatedAt = now
	query := "INSERT INTO tracker (name, description, body, visibility, type, chart_config, owner_id, created_at, last_updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"

	res, err := s.db.Exec(query, tracker.Name, tracker.Description, tracker.Body, tracker.Visibility, tracker.Type, tracker.ChartConfig, tracker.OwnerId, tracker.CreatedAt, tracker.LastUpdatedAt)
	if err != nil {
		return fmt.Errorf("addTracker insert: %w", err)
	}

	tracker.Id, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("addTracker LastInsertId: %w", err)
	}

	return nil
}

func (s *trackerStore) listTrackers(userID int64, searchQuery string, page, perPage int) ([]TrackerResponse, int, error) {
	loggedIn := userID > 0
	hasQuery := searchQuery != ""

	// userScope uses EXISTS subqueries instead of LEFT JOINs with OR
	// conditions. SQLite's OR-optimizer returns wrong results when an OR
	// expression containing bound parameters is combined with another bound
	// parameter (e.g. a LIKE) in the same WHERE clause.
	userScope := "(t.owner_id = ? OR EXISTS (SELECT 1 FROM tracker_member m2 WHERE m2.tracker_id = t.id AND m2.user_id = ?) OR EXISTS (SELECT 1 FROM tracker_like l2 WHERE l2.tracker_id = t.id AND l2.user_id = ?))"

	var whereClause string
	var whereArgs []interface{}

	switch {
	case !hasQuery && loggedIn:
		// No query, logged in: user's trackers only (owned + members + liked)
		whereClause = userScope
		whereArgs = []interface{}{userID, userID, userID}
	case hasQuery && loggedIn:
		// Query provided, logged in: user's trackers + public, filtered by name
		whereClause = "(" + userScope + " OR t.visibility = 'public') AND t.name LIKE ?"
		whereArgs = []interface{}{userID, userID, userID, "%" + searchQuery + "%"}
	case hasQuery && !loggedIn:
		// Query provided, not logged in: public only, filtered by name
		whereClause = "t.visibility = 'public' AND t.name LIKE ?"
		whereArgs = []interface{}{"%" + searchQuery + "%"}
	default:
		// No query, not logged in: empty result
		return []TrackerResponse{}, 0, nil
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM tracker t WHERE %s`, whereClause)

	var total int
	err := s.db.Get(&total, countQuery, whereArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("listTrackers count: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT t.id, t.name, t.description, t.body, t.visibility, t.type, t.chart_config,
		       t.owner_id, u.username AS owner_name, t.created_at, t.last_updated_at,
		       CASE WHEN t.owner_id = ? THEN 'owner' ELSE COALESCE(m.role, '') END AS role,
		       CASE WHEN l.user_id IS NOT NULL THEN 1 ELSE 0 END AS liked,
		       (SELECT COUNT(*) FROM tracker_like WHERE tracker_id = t.id) AS like_count
		FROM tracker t
		LEFT JOIN tracker_member m ON t.id = m.tracker_id AND m.user_id = ?
		LEFT JOIN tracker_like l ON t.id = l.tracker_id AND l.user_id = ?
		LEFT JOIN user u ON u.id = t.owner_id
		WHERE %s
		ORDER BY t.name`, whereClause)

	selectArgs := make([]interface{}, 0, len(whereArgs)+3)
	selectArgs = append(selectArgs, userID) // owner_id in role CASE
	selectArgs = append(selectArgs, userID) // m.user_id in member join
	selectArgs = append(selectArgs, userID) // l.user_id in like join
	selectArgs = append(selectArgs, whereArgs...)

	// Inline pagination as literal integers: SQLite returns wrong row counts
	// when LIMIT/OFFSET are bound parameters on a query with LEFT JOINs
	// (observed with mattn/go-sqlite3). page/perPage are validated ints.
	if perPage > 0 {
		selectQuery += " LIMIT " + strconv.Itoa(perPage) + " OFFSET " + strconv.Itoa((page-1)*perPage)
	}

	rows := []TrackerResponse{}
	err = s.db.Select(&rows, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("listTrackers select: %w", err)
	}

	return rows, total, nil
}

// listTrackersByOwner lists trackers owned by `ownerID`. Public trackers are
// visible to everyone; private ones only to the page owner (viewerID == ownerID).
// A search query filters by tracker name (LIKE '%query%').
func (s *trackerStore) listTrackersByOwner(ownerID, viewerID int64, searchQuery string, page, perPage int) ([]TrackerResponse, int, error) {
	whereClause := "t.owner_id = ? AND (t.visibility = 'public' OR t.owner_id = ?)"
	whereArgs := []interface{}{ownerID, viewerID}

	if searchQuery != "" {
		whereClause += " AND t.name LIKE ?"
		whereArgs = append(whereArgs, "%"+searchQuery+"%")
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM tracker t WHERE %s`, whereClause)

	var total int
	err := s.db.Get(&total, countQuery, whereArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("listTrackersByOwner count: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT t.id, t.name, t.description, t.body, t.visibility, t.type, t.chart_config,
		       t.owner_id, u.username AS owner_name, t.created_at, t.last_updated_at,
		       CASE WHEN t.owner_id = ? THEN 'owner' ELSE COALESCE(m.role, '') END AS role,
		       CASE WHEN l.user_id IS NOT NULL THEN 1 ELSE 0 END AS liked,
		       (SELECT COUNT(*) FROM tracker_like WHERE tracker_id = t.id) AS like_count
		FROM tracker t
		LEFT JOIN tracker_member m ON t.id = m.tracker_id AND m.user_id = ?
		LEFT JOIN tracker_like l ON t.id = l.tracker_id AND l.user_id = ?
		LEFT JOIN user u ON u.id = t.owner_id
		WHERE %s
		ORDER BY t.name`, whereClause)

	selectArgs := make([]interface{}, 0, len(whereArgs)+3)
	selectArgs = append(selectArgs, viewerID) // owner_id in role CASE
	selectArgs = append(selectArgs, viewerID) // m.user_id in member join
	selectArgs = append(selectArgs, viewerID) // l.user_id in like join
	selectArgs = append(selectArgs, whereArgs...)

	if perPage > 0 {
		selectQuery += " LIMIT " + strconv.Itoa(perPage) + " OFFSET " + strconv.Itoa((page-1)*perPage)
	}

	rows := []TrackerResponse{}
	err = s.db.Select(&rows, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("listTrackersByOwner select: %w", err)
	}

	return rows, total, nil
}

func (s *trackerStore) findTrackerById(id int64) (*TrackerModel, error) {
	query := `SELECT t.id, t.name, t.description, t.body, t.visibility, t.type, t.chart_config,
		t.owner_id, t.created_at, t.last_updated_at
		FROM tracker t
		WHERE t.id = ?`

	rows := []TrackerModel{}
	err := s.db.Select(&rows, query, id)
	if err != nil {
		return nil, fmt.Errorf("findTrackerById select: %w", err)
	}

	if len(rows) == 0 {
		return nil, errorTrackerNotFound
	}

	return &rows[0], nil
}

func (s *trackerStore) findTrackerResponseById(id, userID int64) (*TrackerResponse, error) {
	query := `
		SELECT t.id, t.name, t.description, t.body, t.visibility, t.type, t.chart_config,
		       t.owner_id, u.username AS owner_name, t.created_at, t.last_updated_at,
		       CASE WHEN t.owner_id = ? THEN 'owner' ELSE COALESCE(m.role, '') END AS role,
		       CASE WHEN l.user_id IS NOT NULL THEN 1 ELSE 0 END AS liked,
		       (SELECT COUNT(*) FROM tracker_like WHERE tracker_id = t.id) AS like_count
		FROM tracker t
		LEFT JOIN tracker_member m ON t.id = m.tracker_id AND m.user_id = ?
		LEFT JOIN tracker_like l ON t.id = l.tracker_id AND l.user_id = ?
		LEFT JOIN user u ON u.id = t.owner_id
		WHERE t.id = ?`

	rows := []TrackerResponse{}
	err := s.db.Select(&rows, query, userID, userID, userID, id)
	if err != nil {
		return nil, fmt.Errorf("findTrackerResponseById select: %w", err)
	}
	if len(rows) == 0 {
		return nil, errorTrackerNotFound
	}
	return &rows[0], nil
}

func (s *trackerStore) deleteTracker(id int64) error {
	query := "DELETE FROM tracker WHERE id = ?"
	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("deleteTracker delete: %w", err)
	}
	return nil
}

func (s *trackerStore) updateTracker(id int64, visibility, chartConfig, description, body *string) error {
	if visibility == nil && chartConfig == nil && description == nil && body == nil {
		return nil
	}
	query := "UPDATE tracker SET "
	args := []any{}
	parts := []string{}
	if visibility != nil {
		parts = append(parts, "visibility = ?")
		args = append(args, *visibility)
	}
	if chartConfig != nil {
		parts = append(parts, "chart_config = ?")
		args = append(args, *chartConfig)
	}
	if description != nil {
		parts = append(parts, "description = ?")
		args = append(args, *description)
	}
	if body != nil {
		parts = append(parts, "body = ?")
		args = append(args, *body)
	}
	for i, p := range parts {
		if i > 0 {
			query += ", "
		}
		query += p
	}
	query += " WHERE id = ?"
	args = append(args, id)

	res, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("updateTracker exec: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("updateTracker RowsAffected: %w", err)
	}
	if n == 0 {
		return errorTrackerNotFound
	}
	return nil
}

// ----------------------------------------------------------------------
// Series

func (s *trackerStore) addSeries(series *SeriesModel) error {
	_, err := s.findTrackerById(series.TrackerId)
	if err != nil {
		return fmt.Errorf("addSeries findTrackerById: %w", err)
	}

	if series.Config == "" {
		series.Config = "{}"
	}

	query := "INSERT INTO tracker_series (tracker_id, name, data_type, config) VALUES (?, ?, ?, ?)"

	res, err := s.db.Exec(query, series.TrackerId, series.Name, series.DataType, series.Config)
	if err != nil {
		return fmt.Errorf("addSeries insert: %w", err)
	}

	series.Id, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("addSeries LastInsertId: %w", err)
	}

	return nil
}

func (s *trackerStore) findSeriesById(id int64) (*SeriesModel, error) {
	query := "SELECT id, tracker_id, name, data_type, config FROM tracker_series WHERE id = ?"

	rows := []SeriesModel{}
	err := s.db.Select(&rows, query, id)
	if err != nil {
		return nil, fmt.Errorf("findSeriesById select: %w", err)
	}

	if len(rows) == 0 {
		return nil, errorSeriesNotFound
	}

	return &rows[0], nil
}

func (s *trackerStore) listSeries(trackerId int64) ([]SeriesModel, error) {
	query := "SELECT id, tracker_id, name, data_type, config FROM tracker_series WHERE tracker_id = ?"

	rows := []SeriesModel{}
	err := s.db.Select(&rows, query, trackerId)
	if err != nil {
		return nil, fmt.Errorf("listSeries select: %w", err)
	}

	return rows, nil
}

func (s *trackerStore) deleteSeries(id int64) error {
	query := "DELETE FROM tracker_series WHERE id = ?"

	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("deleteSeries delete: %w", err)
	}
	return nil
}

func (s *trackerStore) updateSeries(id int64, name *string, dataType *string, config *string) error {
	if name == nil && dataType == nil && config == nil {
		return nil
	}
	query := "UPDATE tracker_series SET "
	args := []any{}
	parts := []string{}
	if name != nil {
		parts = append(parts, "name = ?")
		args = append(args, *name)
	}
	if dataType != nil {
		parts = append(parts, "data_type = ?")
		args = append(args, *dataType)
	}
	if config != nil {
		parts = append(parts, "config = ?")
		args = append(args, *config)
	}
	for i, p := range parts {
		if i > 0 {
			query += ", "
		}
		query += p
	}
	query += " WHERE id = ?"
	args = append(args, id)

	res, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("updateSeries exec: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("updateSeries RowsAffected: %w", err)
	}
	if n == 0 {
		return errorSeriesNotFound
	}
	return nil
}

// ----------------------------------------------------------------------
// Value

func (s *trackerStore) addValue(value *ValueModel) error {
	series, err := s.findSeriesById(value.SeriesId)
	if err != nil {
		return fmt.Errorf("addValue findSeriesById: %w", err)
	}

	query := "INSERT INTO tracker_value (series_id, time, value) VALUES (?, ?, ?)"

	res, err := s.db.Exec(query, value.SeriesId, value.Timestamp, value.Value)
	if err != nil {
		return fmt.Errorf("addValue insert: %w", err)
	}

	value.Id, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("addValue LastInsertId: %w", err)
	}

	if err := s.touchTracker(series.TrackerId); err != nil {
		return fmt.Errorf("addValue touchTracker: %w", err)
	}

	return nil
}

// touchTracker sets the tracker's last_updated_at to now.
func (s *trackerStore) touchTracker(trackerID int64) error {
	_, err := s.db.Exec("UPDATE tracker SET last_updated_at = ? WHERE id = ?", time.Now(), trackerID)
	if err != nil {
		return fmt.Errorf("touchTracker update: %w", err)
	}
	return nil
}

// setLastUpdatedAt sets the tracker's last_updated_at to the given time.
func (s *trackerStore) setLastUpdatedAt(trackerID int64, ts time.Time) error {
	_, err := s.db.Exec("UPDATE tracker SET last_updated_at = ? WHERE id = ?", ts, trackerID)
	if err != nil {
		return fmt.Errorf("setLastUpdatedAt update: %w", err)
	}
	return nil
}

func (s *trackerStore) listValues(seriesId int64, limit int) ([]ValueModel, error) {
	query := "SELECT id, series_id, time, value FROM tracker_value WHERE series_id = ? ORDER BY time"

	var rows []ValueModel
	var err error

	if limit > 0 {
		query += " LIMIT ?"
		err = s.db.Select(&rows, query, seriesId, limit)
	} else {
		err = s.db.Select(&rows, query, seriesId)
	}

	if err != nil {
		return nil, fmt.Errorf("listValues select: %w", err)
	}

	return rows, nil
}

func (s *trackerStore) listLatestValues(seriesId int64, limit int) ([]ValueModel, error) {
	query := "SELECT id, series_id, time, value FROM tracker_value WHERE series_id = ? ORDER BY time DESC"

	rows := make([]ValueModel, 0)
	var err error

	if limit > 0 {
		query += " LIMIT ?"
		err = s.db.Select(&rows, query, seriesId, limit)
	} else {
		err = s.db.Select(&rows, query, seriesId)
	}

	if err != nil {
		return nil, fmt.Errorf("listLatestValues select: %w", err)
	}

	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}

	return rows, nil
}

func (s *trackerStore) deleteValues(seriesId int64) error {
	query := "DELETE FROM tracker_value WHERE series_id = ?"
	_, err := s.db.Exec(query, seriesId)
	if err != nil {
		return fmt.Errorf("deleteValues delete: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------
// Member

func (s *trackerStore) isMember(userID, trackerID int64) (bool, string, error) {
	tracker, err := s.findTrackerById(trackerID)
	if err != nil {
		return false, "", err
	}
	if tracker.OwnerId == userID {
		return true, "owner", nil
	}
	query := "SELECT role FROM tracker_member WHERE user_id = ? AND tracker_id = ?"
	var role string
	err = s.db.Get(&role, query, userID, trackerID)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("isMember select: %w", err)
	}
	return true, role, nil
}

func (s *trackerStore) findUsername(userID int64) (string, error) {
	var username string
	err := s.db.Get(&username, "SELECT username FROM user WHERE id = ?", userID)
	if err != nil {
		return "", fmt.Errorf("findUsername select: %w", err)
	}
	return username, nil
}

// ----------------------------------------------------------------------
// Like

func (s *trackerStore) addLike(userID, trackerID int64) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO tracker_like (user_id, tracker_id) VALUES (?, ?)",
		userID, trackerID)
	if err != nil {
		return fmt.Errorf("addLike insert: %w", err)
	}
	return nil
}

func (s *trackerStore) removeLike(userID, trackerID int64) error {
	_, err := s.db.Exec(
		"DELETE FROM tracker_like WHERE user_id = ? AND tracker_id = ?",
		userID, trackerID)
	if err != nil {
		return fmt.Errorf("removeLike delete: %w", err)
	}
	return nil
}

func (s *trackerStore) isLiked(userID, trackerID int64) (bool, error) {
	query := "SELECT COUNT(*) FROM tracker_like WHERE user_id = ? AND tracker_id = ?"
	var count int
	err := s.db.Get(&count, query, userID, trackerID)
	if err != nil {
		return false, fmt.Errorf("isLiked select: %w", err)
	}
	return count > 0, nil
}

func (s *trackerStore) countLikes(trackerID int64) (int, error) {
	query := "SELECT COUNT(*) FROM tracker_like WHERE tracker_id = ?"
	var count int
	err := s.db.Get(&count, query, trackerID)
	if err != nil {
		return 0, fmt.Errorf("countLikes select: %w", err)
	}
	return count, nil
}

// ----------------------------------------------------------------------
// Init

// migrate applies additive schema changes to databases created by older
// versions of the schema (schemaTracker only creates new tables).
func (s *trackerStore) migrate() error {
	if !s.hasColumn("tracker", "body") {
		_, err := s.db.Exec("ALTER TABLE tracker ADD COLUMN body TEXT NOT NULL DEFAULT ''")
		if err != nil {
			return fmt.Errorf("migrate tracker body column: %w", err)
		}
	}
	return nil
}

func (s *trackerStore) hasColumn(table, column string) bool {
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

func (s *trackerStore) initialize() error {
	_, err := s.db.Exec(schemaTracker)
	if err != nil {
		return fmt.Errorf("initialize schemaTracker: %w", err)
	}

	// Migrations for databases created before new columns were introduced.
	if err := s.migrate(); err != nil {
		return err
	}

	_, err = s.db.Exec(schemaSeries)
	if err != nil {
		return fmt.Errorf("initialize schemaSeries: %w", err)
	}

	_, err = s.db.Exec(schemaValue)
	if err != nil {
		return fmt.Errorf("initialize schemaValue: %w", err)
	}

	_, err = s.db.Exec(schemaMember)
	if err != nil {
		return fmt.Errorf("initialize schemaMember: %w", err)
	}

	_, err = s.db.Exec(schemaLike)
	if err != nil {
		return fmt.Errorf("initialize schemaLike: %w", err)
	}

	// Index for owner-based queries (e.g. user pages listing owned trackers).
	_, err = s.db.Exec("CREATE INDEX IF NOT EXISTS idx_tracker_owner_id ON tracker(owner_id)")
	if err != nil {
		return fmt.Errorf("initialize idx_tracker_owner_id: %w", err)
	}

	return nil
}
