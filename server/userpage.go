package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/render"
	"github.com/iszk1215/mora/tracker"
	"github.com/rs/zerolog/log"
)

// handleUserGet godoc
// @Summary      Get a user by username
// @Description  Return the public user profile for the given username
// @Tags         server
// @Param        userName  path  string  true  "Username"
// @Success      200  {object}  server.User
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/users/{userName} [get]
func (s *MoraServer) handleUserGet(w http.ResponseWriter, r *http.Request) {
	userName := chi.URLParam(r, "userName")
	user, err := s.userStore.FindByUsername(userName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			render.NotFound(w, errors.New("user not found"))
			return
		}
		log.Error().Err(err).Msg("server.handleUserGet FindByUsername")
		render.InternalError(w, err)
		return
	}
	render.JSON(w, user, http.StatusOK)
}

// handleUserTrackers godoc
// @Summary      List trackers owned by a user
// @Description  Return trackers owned by the user. Public trackers are visible to everyone; private ones only to the owner.
// @Tags         server
// @Param        userName  path  string  true  "Username"
// @Param        q         query  string  false  "Search query (partial match on tracker name)"
// @Param        page      query  int     false  "Page number"
// @Param        per_page  query  int     false  "Items per page"
// @Success      200  {object}  tracker.ListTrackersResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/users/{userName}/trackers [get]
func (s *MoraServer) handleUserTrackers(w http.ResponseWriter, r *http.Request) {
	userName := chi.URLParam(r, "userName")
	user, err := s.userStore.FindByUsername(userName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			render.NotFound(w, errors.New("user not found"))
			return
		}
		log.Error().Err(err).Msg("server.handleUserTrackers FindByUsername")
		render.InternalError(w, err)
		return
	}

	viewerID, _ := tracker.UserIDFromContext(r.Context())

	q := r.URL.Query().Get("q")

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

	trackers, total, err := s.tracker.ListTrackersByOwner(user.ID, viewerID, q, page, perPage)
	if err != nil {
		log.Error().Err(err).Msg("server.handleUserTrackers ListTrackersByOwner")
		render.InternalError(w, err)
		return
	}

	render.JSON(w, tracker.ListTrackersResponse{
		Trackers: trackers,
		Total:    total,
		Page:     page,
		PerPage:  perPage,
	}, http.StatusOK)
}
