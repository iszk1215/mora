package tracker

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Service struct {
	store            *trackerStore
	coverageProvider CoverageTimelineProvider
}

func NewService(db *sqlx.DB, cp CoverageTimelineProvider) (*Service, error) {
	log.Print("tracker.NewService")
	store := newTrackerStore(db)
	err := store.initialize()
	if err != nil {
		return nil, fmt.Errorf("tracker store initialize: %w", err)
	}
	return &Service{store: store, coverageProvider: cp}, nil
}

func (s *Service) Handler() http.Handler {
	return newHandler(s.store, s.coverageProvider)
}

func (s *Service) CreateTracker(name, visibility string, userID int64, trackerType string, repoID *int64) (*TrackerModel, error) {
	t := &TrackerModel{Name: name, Visibility: visibility, Type: trackerType}
	if err := s.store.addTracker(t, userID, repoID); err != nil {
		return nil, fmt.Errorf("CreateTracker: %w", err)
	}
	return t, nil
}

func (s *Service) CreateSeries(trackerID int64, name, dataType string) (*SeriesModel, error) {
	se := &SeriesModel{TrackerId: trackerID, Name: name, DataType: dataType}
	if err := s.store.addSeries(se); err != nil {
		return nil, fmt.Errorf("CreateSeries: %w", err)
	}
	return se, nil
}

func (s *Service) CreateValue(seriesID int64, timestamp time.Time, value float64) (*ValueModel, error) {
	v := &ValueModel{SeriesId: seriesID, Timestamp: timestamp, Value: value}
	if err := s.store.addValue(v); err != nil {
		return nil, fmt.Errorf("CreateValue: %w", err)
	}
	return v, nil
}
