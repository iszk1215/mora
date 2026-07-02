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
    name TEXT NOT NULL UNIQUE,
    visibility TEXT NOT NULL DEFAULT 'private'
)`

var schemaSeries = `
CREATE TABLE IF NOT EXISTS tracker_series (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tracker_id INTEGER NOT NULL REFERENCES tracker(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    data_type TEXT NOT NULL DEFAULT 'float',
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
	Id         int64  `json:"id"    db:"id"`
	Name       string `json:"name"  db:"name"`
	Visibility string `json:"visibility"` // "public" | "unlisted" | "private"
	Role       string `json:"role"`       // "" | "owner" | "editor"
	Liked      bool   `json:"liked"`
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
	query := "INSERT INTO tracker (name, visibility) VALUES (?, ?)"

	res, err := s.db.Exec(query, tracker.Name, tracker.Visibility)
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

	return nil
}

func (s *trackerStore) listTrackers(userID int64, page, perPage int) ([]TrackerResponse, int, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM tracker t
		LEFT JOIN tracker_member m ON t.id = m.tracker_id AND m.user_id = ?
		LEFT JOIN tracker_like l ON t.id = l.tracker_id AND l.user_id = ?
		WHERE m.user_id IS NOT NULL OR l.user_id IS NOT NULL`

	var total int
	err := s.db.Get(&total, countQuery, userID, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("listTrackers count: %w", err)
	}

	query := `
		SELECT t.id, t.name, t.visibility,
		       COALESCE(m.role, '') AS role,
		       CASE WHEN l.user_id IS NOT NULL THEN 1 ELSE 0 END AS liked
		FROM tracker t
		LEFT JOIN tracker_member m ON t.id = m.tracker_id AND m.user_id = ?
		LEFT JOIN tracker_like l ON t.id = l.tracker_id AND l.user_id = ?
		WHERE m.user_id IS NOT NULL OR l.user_id IS NOT NULL
		ORDER BY t.name`

	rows := []TrackerResponse{}
	if perPage > 0 {
		query += " LIMIT ? OFFSET ?"
		offset := (page - 1) * perPage
		err = s.db.Select(&rows, query, userID, userID, perPage, offset)
	} else {
		err = s.db.Select(&rows, query, userID, userID)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("listTrackers select: %w", err)
	}

	return rows, total, nil
}

func (s *trackerStore) findTrackerById(id int64) (*TrackerModel, error) {
	query := "SELECT id, name, visibility FROM tracker WHERE id = ?"

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

func (s *trackerStore) deleteTracker(id int64) error {
	query := "DELETE FROM tracker WHERE id = ?"
	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("deleteTracker delete: %w", err)
	}
	return nil
}

func (s *trackerStore) updateVisibility(id int64, visibility string) error {
	query := "UPDATE tracker SET visibility = ? WHERE id = ?"
	res, err := s.db.Exec(query, visibility, id)
	if err != nil {
		return fmt.Errorf("updateVisibility exec: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("updateVisibility RowsAffected: %w", err)
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

	query := "INSERT INTO tracker_series (tracker_id, name, data_type) VALUES (?, ?, ?)"

	res, err := s.db.Exec(query, series.TrackerId, series.Name, series.DataType)
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
	query := "SELECT id, tracker_id, name, data_type FROM tracker_series WHERE id = ?"

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
	query := "SELECT id, tracker_id, name, data_type FROM tracker_series WHERE tracker_id = ?"

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

	var rows []ValueModel
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

// ----------------------------------------------------------------------
// Init

func (s *trackerStore) initialize() error {
	// Drop old track tables if they exist
	_, err := s.db.Exec("DROP TABLE IF EXISTS track_like")
	if err != nil {
		return fmt.Errorf("drop old track_like: %w", err)
	}
	_, err = s.db.Exec("DROP TABLE IF EXISTS track_value")
	if err != nil {
		return fmt.Errorf("drop old track_value: %w", err)
	}
	_, err = s.db.Exec("DROP TABLE IF EXISTS track_series")
	if err != nil {
		return fmt.Errorf("drop old track_series: %w", err)
	}
	_, err = s.db.Exec("DROP TABLE IF EXISTS track_member")
	if err != nil {
		return fmt.Errorf("drop old track_member: %w", err)
	}
	_, err = s.db.Exec("DROP TABLE IF EXISTS track")
	if err != nil {
		return fmt.Errorf("drop old track: %w", err)
	}

	_, err = s.db.Exec(schemaTracker)
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

	return nil
}
