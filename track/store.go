package track

import (
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
    name TEXT NOT NULL UNIQUE
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

type (
	trackStore struct {
		db *sqlx.DB
	}
)

func newTrackStore(db *sqlx.DB) *trackStore {
	return &trackStore{db: db}
}

// ----------------------------------------------------------------------
// Track

func (s *trackStore) addTrack(track *TrackModel) error {
	query := "INSERT INTO track (name) VALUES (?)"

	res, err := s.db.Exec(query, track.Name)
	if err != nil {
		return fmt.Errorf("addTrack insert: %w", err)
	}

	track.Id, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("addTrack LastInsertId: %w", err)
	}

	return nil
}

func (s *trackStore) listTracks() ([]TrackModel, error) {
	query := "SELECT id, name FROM track"

	rows := []TrackModel{}
	err := s.db.Select(&rows, query)
	if err != nil {
		return nil, fmt.Errorf("listTracks select: %w", err)
	}

	return rows, nil
}

func (s *trackStore) findTrackById(id int64) (*TrackModel, error) {
	query := "SELECT id, name FROM track WHERE id = ?"

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

	return nil
}
