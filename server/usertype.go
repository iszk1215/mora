package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/core"
	"github.com/iszk1215/mora/render"
	"github.com/iszk1215/mora/tracker"
	"github.com/rs/zerolog/log"
)

type setUserTypeRequest struct {
	UserType string `json:"user_type"`
}

// isAdminUser reports whether the user identified by `userID` may perform
// administrative operations. The seeded admin (id=1) is always an admin.
func (s *MoraServer) isAdminUser(userID int64) bool {
	if userID == 1 {
		return true
	}
	user, err := s.userStore.FindByID(userID)
	if err != nil {
		return false
	}
	return user.Type == core.UserTypeAdmin
}

// handleUserSetType godoc
// @Summary      Set a user's type (admin only)
// @Description  Update the user type of the given username. Only admins may call this endpoint, and admins cannot change their own type.
// @Tags         server
// @Param        userName  path  string              true  "Username"
// @Param        body      body  server.setUserTypeRequest  true  "New user type"
// @Success      200  {object}  server.User
// @Failure      400  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/users/{userName}/type [patch]
func (s *MoraServer) handleUserSetType(w http.ResponseWriter, r *http.Request) {
	adminID, ok := tracker.UserIDFromContext(r.Context())
	if !ok {
		render.Forbidden(w, errors.New("authentication required"))
		return
	}
	if !s.isAdminUser(adminID) {
		render.Forbidden(w, errors.New("admin privileges required"))
		return
	}

	var req setUserTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn().Err(err).Msg("invalid set user type request body")
		render.BadRequest(w, errors.New("invalid request body"))
		return
	}
	if !core.IsValidUserType(req.UserType) {
		render.BadRequest(w, errors.New("user_type must be one of: free, pro, admin"))
		return
	}

	target, err := s.userStore.FindByUsername(chi.URLParam(r, "userName"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			render.NotFound(w, errors.New("user not found"))
			return
		}
		log.Error().Err(err).Msg("server.handleUserSetType FindByUsername")
		render.InternalError(w, err)
		return
	}
	if target.ID == adminID {
		render.Forbidden(w, errors.New("cannot change your own user type"))
		return
	}

	if err := s.userStore.UpdateUserType(target.ID, req.UserType); err != nil {
		log.Error().Err(err).Msg("server.handleUserSetType UpdateUserType")
		render.InternalError(w, err)
		return
	}

	updated, err := s.userStore.FindByID(target.ID)
	if err != nil {
		log.Error().Err(err).Msg("server.handleUserSetType FindByID")
		render.InternalError(w, err)
		return
	}
	log.Info().Int64("admin_id", adminID).Int64("user_id", target.ID).
		Str("user_type", req.UserType).Msg("user type updated")
	render.JSON(w, updated, http.StatusOK)
}
