package tracker

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

var (
	errorTrackerNotFound = errors.New("no tracker found")
	errorSeriesNotFound  = errors.New("no series found")
)

var schemaTracker = `
CREATE TABLE IF NOT EXISTS tracker (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    visibility TEXT NOT NULL DEFAULT 'private',
    type TEXT NOT NULL DEFAULT 'tracker',
    chart_config TEXT NOT NULL DEFAULT '{}'
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
		db             *sqlx.DB
		coverageLinker CoverageLinkManager
	}
)

// TrackerResponse is returned in tracker lists and includes user-specific flags.
type TrackerResponse struct {
	Id          int64  `json:"id"    db:"id"`
	Name        string `json:"name"  db:"name"`
	Description string `json:"description" db:"description"`
	Visibility  string `json:"visibility"` // "public" | "private"
	Type        string `json:"type"`
	RepoID      *int64 `json:"repo_id,omitempty" db:"repo_id"`
	ChartConfig string `json:"chart_config" db:"chart_config"`
	Role        string `json:"role"`       // "" | "owner" | "editor"
	Liked       bool   `json:"liked"`
	LikeCount   int    `json:"like_count" db:"like_count"`
}

func newTrackerStore(db *sqlx.DB, cl CoverageLinkManager) *trackerStore {
	return &trackerStore{db: db, coverageLinker: cl}
}

// ----------------------------------------------------------------------
// Tracker

func (s *trackerStore) addTracker(tracker *TrackerModel, userID int64, repoID *int64) error {
	if tracker.Visibility == "" {
		tracker.Visibility = "private"
	}
	if tracker.ChartConfig == "" {
		tracker.ChartConfig = "{}"
	}
	query := "INSERT INTO tracker (name, description, visibility, type, chart_config) VALUES (?, ?, ?, ?, ?)"

	res, err := s.db.Exec(query, tracker.Name, tracker.Description, tracker.Visibility, tracker.Type, tracker.ChartConfig)
	if err != nil {
		return fmt.Errorf("addTracker insert: %w", err)
	}

	tracker.Id, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("addTracker LastInsertId: %w", err)
	}

	_, err = s.db.Exec(
		"INSERT INTO tracker_member (user_id, tracker_id, role) VALUES (?, ?, 'owner')",
		userID, tracker.Id)
	if err != nil {
		return fmt.Errorf("addTracker addMember: %w", err)
	}

	if repoID != nil {
		if s.coverageLinker == nil {
			return fmt.Errorf("addTracker link coverage: no coverage linker configured")
		}
		if err := s.coverageLinker.Link(tracker.Id, *repoID); err != nil {
			return fmt.Errorf("addTracker link coverage: %w", err)
		}
	}

	return nil
}

func (s *trackerStore) listTrackers(userID int64, searchQuery string, page, perPage int) ([]TrackerResponse, int, error) {
	loggedIn := userID > 0
	hasQuery := searchQuery != ""

	var whereClause string
	var args []interface{}

	// JOIN args are always needed (userID for member/like joins)
	joinArgs := []interface{}{userID, userID}

	switch {
	case !hasQuery && loggedIn:
		// No query, logged in: user's trackers only (members + liked)
		whereClause = "(m.user_id IS NOT NULL OR l.user_id IS NOT NULL)"
		args = joinArgs
	case hasQuery && loggedIn:
		// Query provided, logged in: user's trackers + public, filtered by name
		whereClause = "((m.user_id IS NOT NULL OR l.user_id IS NOT NULL) OR t.visibility = 'public') AND t.name LIKE ?"
		args = append(joinArgs, "%"+searchQuery+"%")
	case hasQuery && !loggedIn:
		// Query provided, not logged in: public only, filtered by name
		whereClause = "t.visibility = 'public' AND t.name LIKE ?"
		args = append(joinArgs, "%"+searchQuery+"%")
	default:
		// No query, not logged in: empty result
		return []TrackerResponse{}, 0, nil
	}

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM tracker t
		LEFT JOIN tracker_member m ON t.id = m.tracker_id AND m.user_id = ?
		LEFT JOIN tracker_like l ON t.id = l.tracker_id AND l.user_id = ?
		WHERE %s`, whereClause)

	var total int
	err := s.db.Get(&total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listTrackers count: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT t.id, t.name, t.description, t.visibility, t.type, t.chart_config,
		       tc.repo_id,
		       COALESCE(m.role, '') AS role,
		       CASE WHEN l.user_id IS NOT NULL THEN 1 ELSE 0 END AS liked,
		       (SELECT COUNT(*) FROM tracker_like WHERE tracker_id = t.id) AS like_count
		FROM tracker t
		LEFT JOIN tracker_coverage tc ON tc.tracker_id = t.id
		LEFT JOIN tracker_member m ON t.id = m.tracker_id AND m.user_id = ?
		LEFT JOIN tracker_like l ON t.id = l.tracker_id AND l.user_id = ?
		WHERE %s
		ORDER BY t.name`, whereClause)

	selectArgs := make([]interface{}, len(args))
	copy(selectArgs, args)

	if perPage > 0 {
		selectQuery += " LIMIT ? OFFSET ?"
		offset := (page - 1) * perPage
		selectArgs = append(selectArgs, perPage, offset)
	}

	rows := []TrackerResponse{}
	err = s.db.Select(&rows, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("listTrackers select: %w", err)
	}

	return rows, total, nil
}

func (s *trackerStore) findTrackerById(id int64) (*TrackerModel, error) {
	query := `SELECT t.id, t.name, t.description, t.visibility, t.type, t.chart_config, tc.repo_id
		FROM tracker t
		LEFT JOIN tracker_coverage tc ON tc.tracker_id = t.id
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
		SELECT t.id, t.name, t.description, t.visibility, t.type, t.chart_config,
		       tc.repo_id,
		       COALESCE(m.role, '') AS role,
		       CASE WHEN l.user_id IS NOT NULL THEN 1 ELSE 0 END AS liked,
		       (SELECT COUNT(*) FROM tracker_like WHERE tracker_id = t.id) AS like_count
		FROM tracker t
		LEFT JOIN tracker_coverage tc ON tc.tracker_id = t.id
		LEFT JOIN tracker_member m ON t.id = m.tracker_id AND m.user_id = ?
		LEFT JOIN tracker_like l ON t.id = l.tracker_id AND l.user_id = ?
		WHERE t.id = ?`

	rows := []TrackerResponse{}
	err := s.db.Select(&rows, query, userID, userID, id)
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

func (s *trackerStore) updateTracker(id int64, visibility, chartConfig, description *string) error {
	if visibility == nil && chartConfig == nil && description == nil {
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
	_, err := s.findSeriesById(value.SeriesId)
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
	query := "SELECT role FROM tracker_member WHERE user_id = ? AND tracker_id = ?"
	var role string
	err := s.db.Get(&role, query, userID, trackerID)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("isMember select: %w", err)
	}
	return true, role, nil
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

func (s *trackerStore) initialize() error {
	_, err := s.db.Exec(schemaTracker)
	if err != nil {
		return fmt.Errorf("initialize schemaTracker: %w", err)
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

	if err := s.migrateCoverageTrackers(1); err != nil {
		return fmt.Errorf("initialize migrateCoverageTrackers: %w", err)
	}

	return nil
}

func (s *trackerStore) migrateCoverageTrackers(adminUserID int64) error {
	// Check if repository table exists; skip if not (e.g., in tests)
	var count int
	err := s.db.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='repository'")
	if err != nil || count == 0 {
		return nil
	}

	// Check if tracker_coverage table exists (created by the coverage store);
	// skip if not yet initialized.
	err = s.db.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tracker_coverage'")
	if err != nil || count == 0 {
		return nil
	}

	type repoRow struct {
		ID        int64  `db:"id"`
		Namespace string `db:"namespace"`
		Name      string `db:"name"`
	}

	var repos []repoRow
	err = s.db.Select(&repos, `
		SELECT r.id, r.namespace, r.name
		FROM repository r
		WHERE NOT EXISTS (
			SELECT 1 FROM tracker_coverage tc WHERE tc.repo_id = r.id
		)
	`)
	if err != nil {
		return nil
	}

	for _, r := range repos {
		trackerName := r.Namespace + "/" + r.Name + " coverage"

		res, err := s.db.Exec(
			"INSERT INTO tracker (name, visibility, type, chart_config) VALUES (?, 'public', 'coverage', '{\"area\":false}')",
			trackerName)
		if err != nil {
			return fmt.Errorf("migrateCoverageTrackers insert tracker for repo %d: %w", r.ID, err)
		}

		trackerID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("migrateCoverageTrackers LastInsertId for repo %d: %w", r.ID, err)
		}

		_, err = s.db.Exec(
			"INSERT INTO tracker_member (user_id, tracker_id, role) VALUES (?, ?, 'owner')",
			adminUserID, trackerID)
		if err != nil {
			return fmt.Errorf("migrateCoverageTrackers insert member for repo %d: %w", r.ID, err)
		}

		if s.coverageLinker == nil {
			return fmt.Errorf("migrateCoverageTrackers link coverage for repo %d: no coverage linker configured", r.ID)
		}
		if err := s.coverageLinker.Link(trackerID, r.ID); err != nil {
			return fmt.Errorf("migrateCoverageTrackers link coverage for repo %d: %w", r.ID, err)
		}
	}

	return nil
}
