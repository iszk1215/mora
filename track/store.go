package track

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

var (
	errorTrackNotFound  = errors.New("no track found")
	errorSeriesNotFound = errors.New("no series found")
)

var schemaTrack = `
CREATE TABLE IF NOT EXISTS track (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    visibility TEXT NOT NULL DEFAULT 'private'
)`

var schemaSeries = `
CREATE TABLE IF NOT EXISTS track_series (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    track_id INTEGER NOT NULL REFERENCES track(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    data_type TEXT NOT NULL DEFAULT 'float',
    UNIQUE(track_id, name)
)`

var schemaValue = `
CREATE TABLE IF NOT EXISTS track_value (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id INTEGER NOT NULL REFERENCES track_series(id) ON DELETE CASCADE,
    time DATETIME NOT NULL,
    value REAL NOT NULL,
    UNIQUE(series_id, time)
)`

var schemaMember = `
CREATE TABLE IF NOT EXISTS track_member (
    user_id  INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    track_id INTEGER NOT NULL REFERENCES track(id) ON DELETE CASCADE,
    role     TEXT NOT NULL DEFAULT 'editor',
    PRIMARY KEY (user_id, track_id)
)`

var schemaLike = `
CREATE TABLE IF NOT EXISTS track_like (
    user_id  INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    track_id INTEGER NOT NULL REFERENCES track(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, track_id)
)`

type (
	trackStore struct {
		db *sqlx.DB
	}
)

// TrackResponse is returned in track lists and includes user-specific flags.
type TrackResponse struct {
	Id         int64  `json:"id"    db:"id"`
	Name       string `json:"name"  db:"name"`
	Visibility string `json:"visibility"` // "public" | "unlisted" | "private"
	Role       string `json:"role"`       // "" | "owner" | "editor"
	Liked      bool   `json:"liked"`
}

func newTrackStore(db *sqlx.DB) *trackStore {
	return &trackStore{db: db}
}

// ----------------------------------------------------------------------
// Track

func (s *trackStore) addTrack(track *TrackModel, userID int64) error {
	if track.Visibility == "" {
		track.Visibility = "private"
	}
	query := "INSERT INTO track (name, visibility) VALUES (?, ?)"

	res, err := s.db.Exec(query, track.Name, track.Visibility)
	if err != nil {
		return fmt.Errorf("addTrack insert: %w", err)
	}

	track.Id, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("addTrack LastInsertId: %w", err)
	}

	_, err = s.db.Exec(
		"INSERT INTO track_member (user_id, track_id, role) VALUES (?, ?, 'owner')",
		userID, track.Id)
	if err != nil {
		return fmt.Errorf("addTrack addMember: %w", err)
	}

	return nil
}

func (s *trackStore) listTracks(userID int64) ([]TrackResponse, error) {
	query := `
		SELECT t.id, t.name, t.visibility,
		       COALESCE(m.role, '') AS role,
		       CASE WHEN l.user_id IS NOT NULL THEN 1 ELSE 0 END AS liked
		FROM track t
		LEFT JOIN track_member m ON t.id = m.track_id AND m.user_id = ?
		LEFT JOIN track_like l ON t.id = l.track_id AND l.user_id = ?
		WHERE m.user_id IS NOT NULL OR l.user_id IS NOT NULL
		ORDER BY t.name`

	rows := []TrackResponse{}
	err := s.db.Select(&rows, query, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("listTracks select: %w", err)
	}

	return rows, nil
}

func (s *trackStore) findTrackById(id int64) (*TrackModel, error) {
	query := "SELECT id, name, visibility FROM track WHERE id = ?"

	rows := []TrackModel{}
	err := s.db.Select(&rows, query, id)
	if err != nil {
		return nil, fmt.Errorf("findTrackById select: %w", err)
	}

	if len(rows) == 0 {
		return nil, errorTrackNotFound
	}

	return &rows[0], nil
}

func (s *trackStore) deleteTrack(id int64) error {
	query := "DELETE FROM track WHERE id = ?"
	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("deleteTrack delete: %w", err)
	}
	return nil
}

func (s *trackStore) updateVisibility(id int64, visibility string) error {
	query := "UPDATE track SET visibility = ? WHERE id = ?"
	res, err := s.db.Exec(query, visibility, id)
	if err != nil {
		return fmt.Errorf("updateVisibility exec: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("updateVisibility RowsAffected: %w", err)
	}
	if n == 0 {
		return errorTrackNotFound
	}
	return nil
}

// ----------------------------------------------------------------------
// Series

func (s *trackStore) addSeries(series *SeriesModel) error {
	_, err := s.findTrackById(series.TrackId)
	if err != nil {
		return fmt.Errorf("addSeries findTrackById: %w", err)
	}

	query := "INSERT INTO track_series (track_id, name, data_type) VALUES (?, ?, ?)"

	res, err := s.db.Exec(query, series.TrackId, series.Name, series.DataType)
	if err != nil {
		return fmt.Errorf("addSeries insert: %w", err)
	}

	series.Id, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("addSeries LastInsertId: %w", err)
	}

	return nil
}

func (s *trackStore) findSeriesById(id int64) (*SeriesModel, error) {
	query := "SELECT id, track_id, name, data_type FROM track_series WHERE id = ?"

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

func (s *trackStore) listSeries(trackId int64) ([]SeriesModel, error) {
	query := "SELECT id, track_id, name, data_type FROM track_series WHERE track_id = ?"

	rows := []SeriesModel{}
	err := s.db.Select(&rows, query, trackId)
	if err != nil {
		return nil, fmt.Errorf("listSeries select: %w", err)
	}

	return rows, nil
}

func (s *trackStore) deleteSeries(id int64) error {
	query := "DELETE FROM track_series WHERE id = ?"

	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("deleteSeries delete: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------
// Value

func (s *trackStore) addValue(value *ValueModel) error {
	_, err := s.findSeriesById(value.SeriesId)
	if err != nil {
		return fmt.Errorf("addValue findSeriesById: %w", err)
	}

	query := "INSERT INTO track_value (series_id, time, value) VALUES (?, ?, ?)"

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

func (s *trackStore) listValues(seriesId int64, limit int) ([]ValueModel, error) {
	query := "SELECT id, series_id, time, value FROM track_value WHERE series_id = ? ORDER BY time"

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

func (s *trackStore) deleteValues(seriesId int64) error {
	query := "DELETE FROM track_value WHERE series_id = ?"
	_, err := s.db.Exec(query, seriesId)
	if err != nil {
		return fmt.Errorf("deleteValues delete: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------
// Member

func (s *trackStore) isMember(userID, trackID int64) (bool, string, error) {
	query := "SELECT role FROM track_member WHERE user_id = ? AND track_id = ?"
	var role string
	err := s.db.Get(&role, query, userID, trackID)
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

func (s *trackStore) addLike(userID, trackID int64) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO track_like (user_id, track_id) VALUES (?, ?)",
		userID, trackID)
	if err != nil {
		return fmt.Errorf("addLike insert: %w", err)
	}
	return nil
}

func (s *trackStore) removeLike(userID, trackID int64) error {
	_, err := s.db.Exec(
		"DELETE FROM track_like WHERE user_id = ? AND track_id = ?",
		userID, trackID)
	if err != nil {
		return fmt.Errorf("removeLike delete: %w", err)
	}
	return nil
}

func (s *trackStore) isLiked(userID, trackID int64) (bool, error) {
	query := "SELECT COUNT(*) FROM track_like WHERE user_id = ? AND track_id = ?"
	var count int
	err := s.db.Get(&count, query, userID, trackID)
	if err != nil {
		return false, fmt.Errorf("isLiked select: %w", err)
	}
	return count > 0, nil
}

// ----------------------------------------------------------------------
// Init

func (s *trackStore) initialize() error {
	_, err := s.db.Exec(schemaTrack)
	if err != nil {
		return fmt.Errorf("initialize schemaTrack: %w", err)
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
