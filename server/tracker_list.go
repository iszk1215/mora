package server

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/iszk1215/mora/coverage"
	"github.com/iszk1215/mora/render"
	"github.com/iszk1215/mora/tracker"
	"github.com/rs/zerolog/log"
)

// handleTrackersList godoc
// @Summary      List trackers for current user
// @Description  Return trackers owned, edited, or liked by the current user. With include=preview, each tracker also embeds its preview data (all series plus their latest values) fetched in a few batched queries.
// @Tags         server
// @Param        include  query  string  false  "Set to \"preview\" to embed series preview data"
// @Param        q        query  string  false  "Search query (partial match on tracker name)"
// @Param        page     query  int     false  "Page number"
// @Param        per_page query  int     false  "Items per page"
// @Success      200  {object}  tracker.ListTrackersResponse
// @Router       /api/trackers [get]
func (s *MoraServer) handleTrackersList(w http.ResponseWriter, r *http.Request) {
	uid, ok := tracker.UserIDFromContext(r.Context())
	if !ok {
		uid = 0
	}

	q := r.URL.Query().Get("q")

	if q == "" && !ok {
		render.JSON(w, tracker.ListTrackersResponse{
			Trackers: []tracker.TrackerResponse{}, Total: 0, Page: 1, PerPage: 0,
		}, http.StatusOK)
		return
	}

	page := 1
	perPage := 0

	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if n, err := strconv.Atoi(pp); err == nil && n > 0 {
			perPage = n
		}
	}

	trackers, total, err := s.tracker.ListTrackers(uid, q, page, perPage)
	if err != nil {
		log.Error().Err(err).Msg("server.handleTrackersList ListTrackers")
		render.InternalError(w, err)
		return
	}

	if r.URL.Query().Get("include") == "preview" && len(trackers) > 0 {
		if err := s.attachPreviews(trackers); err != nil {
			log.Error().Err(err).Msg("server.handleTrackersList attachPreviews")
			render.InternalError(w, err)
			return
		}
	}

	render.JSON(w, tracker.ListTrackersResponse{
		Trackers: trackers,
		Total:    total,
		Page:     page,
		PerPage:  perPage,
	}, http.StatusOK)
}

// attachPreviews embeds preview data (all series plus their latest values)
// into the given trackers using batched queries. Access scoping is inherent in
// the list query that produced the trackers, so no per-tracker permission
// checks are needed here. Tracker-type trackers are served by the tracker
// service; coverage-type trackers use virtual series derived from the coverage
// timeline.
func (s *MoraServer) attachPreviews(trackers []tracker.TrackerResponse) error {
	var trackerIDs []int64
	var coverageIDs []int64
	for i := range trackers {
		if trackers[i].Type == tracker.TypeCoverage {
			coverageIDs = append(coverageIDs, trackers[i].Id)
		} else {
			trackerIDs = append(trackerIDs, trackers[i].Id)
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	if len(trackerIDs) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := s.tracker.BatchPreview(trackerIDs)
			if err != nil {
				errCh <- err
				return
			}
			for i := range trackers {
				if trackers[i].Type != tracker.TypeCoverage {
					trackers[i].Series = data[trackers[i].Id]
				}
			}
		}()
	}

	if len(coverageIDs) > 0 && s.coverage != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			timelineByTracker, err := s.coverage.Store().TimelineByTrackerIDs(coverageIDs, 20)
			if err != nil {
				errCh <- err
				return
			}
			for i := range trackers {
				if trackers[i].Type != tracker.TypeCoverage {
					continue
				}
				timeline := timelineByTracker[trackers[i].Id]
				trackers[i].Series = coverage.TimelineToPreviewSeries(trackers[i].Id, timeline)
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}