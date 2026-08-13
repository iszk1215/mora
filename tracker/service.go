package tracker

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Service struct {
	store *trackerStore
}

func NewService(db *sqlx.DB) (*Service, error) {
	log.Print("tracker.NewService")
	store := newTrackerStore(db)
	err := store.initialize()
	if err != nil {
		return nil, fmt.Errorf("tracker store initialize: %w", err)
	}
	return &Service{store: store}, nil
}

func (s *Service) Handler() http.Handler {
	return newHandler(s.store)
}

func (s *Service) CreateTracker(name, description, body, visibility string, userID int64, trackerType string, chartConfig string) (*TrackerModel, error) {
	t := &TrackerModel{Name: name, Description: description, Body: body, Visibility: visibility, Type: trackerType, ChartConfig: chartConfig}
	if err := s.store.addTracker(t, userID); err != nil {
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

// SetLastUpdatedAt sets the tracker's last_updated_at timestamp.
func (s *Service) SetLastUpdatedAt(trackerID int64, ts time.Time) error {
	return s.store.setLastUpdatedAt(trackerID, ts)
}

// ListTrackersByOwner lists trackers owned by `ownerID`, filtered by the
// viewer's access (private trackers only visible to the owner) and an
// optional name search query. Used by the user page (/users/:userName).
func (s *Service) ListTrackersByOwner(ownerID, viewerID int64, searchQuery string, page, perPage int) ([]TrackerResponse, int, error) {
	return s.store.listTrackersByOwner(ownerID, viewerID, searchQuery, page, perPage)
}

func (s *Service) FindTrackerById(id int64) (*TrackerModel, error) {
	return s.store.findTrackerById(id)
}

func (s *Service) IsMember(userID, trackerID int64) (bool, string, error) {
	return s.store.isMember(userID, trackerID)
}

// InjectTracker loads the tracker from the URL into context.
func (s *Service) InjectTracker(next http.Handler) http.Handler {
	return InjectTracker(s.store, next)
}

// RequireReadPermission checks tracker visibility and membership.
// Returns 404 for unauthorized access to avoid leaking tracker existence.
func (s *Service) RequireReadPermission(next http.Handler) http.Handler {
	return RequireReadPermission(s.store, next)
}

// RequireEditPermission checks tracker edit access (membership).
// Returns 404 for unauthorized access to avoid leaking tracker existence.
func (s *Service) RequireEditPermission(next http.Handler) http.Handler {
	return RequireEditPermission(s.store, next)
}

// RequireOwnerPermission checks tracker ownership (superuser is always allowed).
// Returns 404 for unauthorized access to avoid leaking tracker existence.
func (s *Service) RequireOwnerPermission(next http.Handler) http.Handler {
	return RequireOwnerPermission(s.store, next)
}
