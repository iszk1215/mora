package tracker

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/render"
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

func (s *Service) FindRepoIDByTrackerID(trackerID int64) (*int64, error) {
	return s.store.findRepoIDByTrackerID(trackerID)
}

func (s *Service) CreateTracker(name, visibility string, userID int64, trackerType string, repoID *int64, chartConfig string) (*TrackerModel, error) {
	t := &TrackerModel{Name: name, Visibility: visibility, Type: trackerType, ChartConfig: chartConfig}
	if err := s.store.addTracker(t, userID, repoID); err != nil {
		return nil, fmt.Errorf("CreateTracker: %w", err)
	}
	return t, nil
}

func (s *Service) CreateSeries(trackerID int64, name, dataType, config string) (*SeriesModel, error) {
	se := &SeriesModel{TrackerId: trackerID, Name: name, DataType: dataType, Config: config}
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

func (s *Service) Like(userID, trackerID int64) error {
	return s.store.addLike(userID, trackerID)
}

func (s *Service) FindTrackerById(id int64) (*TrackerModel, error) {
	return s.store.findTrackerById(id)
}

func (s *Service) IsMember(userID, trackerID int64) (bool, string, error) {
	return s.store.isMember(userID, trackerID)
}

// RequireReadPermission checks tracker visibility and membership.
// Returns 404 for unauthorized access to avoid leaking tracker existence.
func (s *Service) RequireReadPermission(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "trackerId"), 10, 64)
		if err != nil {
			render.BadRequest(w, errors.New("invalid tracker id"))
			return
		}
		tracker, err := s.store.findTrackerById(id)
		if err != nil {
			render.NotFound(w, errors.New("tracker not found"))
			return
		}
		if tracker.Visibility == "public" {
			next.ServeHTTP(w, r)
			return
		}
		uid, ok := UserIDFromContext(r.Context())
		if !ok {
			render.NotFound(w, errors.New("tracker not found"))
			return
		}
		if uid == 1 {
			next.ServeHTTP(w, r)
			return
		}
		member, _, err := s.store.isMember(uid, tracker.Id)
		if err != nil {
			log.Error().Err(err).Msg("RequireReadPermission isMember")
			render.InternalError(w, err)
			return
		}
		if !member {
			render.NotFound(w, errors.New("tracker not found"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireEditPermission checks tracker edit access (membership).
// Returns 404 for unauthorized access to avoid leaking tracker existence.
func (s *Service) RequireEditPermission(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "trackerId"), 10, 64)
		if err != nil {
			render.BadRequest(w, errors.New("invalid tracker id"))
			return
		}
		tracker, err := s.store.findTrackerById(id)
		if err != nil {
			render.NotFound(w, errors.New("tracker not found"))
			return
		}
		uid, ok := UserIDFromContext(r.Context())
		if !ok {
			render.NotFound(w, errors.New("tracker not found"))
			return
		}
		if uid == 1 {
			_ = tracker
			next.ServeHTTP(w, r)
			return
		}
		member, _, err := s.store.isMember(uid, tracker.Id)
		if err != nil {
			log.Error().Err(err).Msg("RequireEditPermission isMember")
			render.InternalError(w, err)
			return
		}
		if !member {
			render.NotFound(w, errors.New("tracker not found"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
