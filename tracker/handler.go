package tracker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/render"
	"github.com/rs/zerolog/log"
)

type (
	trackerHandler struct {
		store            *trackerStore
		coverageProvider CoverageTimelineProvider
	}

	TrackerModel struct {
		Id          int64  `json:"id"         db:"id"`
		Name        string `json:"name"       db:"name"`
		Visibility  string `json:"visibility" db:"visibility"`
		Type        string `json:"type"       db:"type"`
		RepoID      *int64 `json:"repo_id,omitempty" db:"repo_id"`
		ChartConfig string `json:"chart_config" db:"chart_config"`
	}

	SeriesModel struct {
		Id        int64  `json:"id"         db:"id"`
		TrackerId int64  `json:"tracker_id" db:"tracker_id"`
		Name      string `json:"name"       db:"name"`
		DataType  string `json:"data_type"  db:"data_type"`
		Config    string `json:"config"     db:"config"`
	}

	ValueModel struct {
		Id        int64     `db:"id"`
		SeriesId  int64     `db:"series_id"`
		Timestamp time.Time `json:"time"  db:"time"`
		Value     float64   `json:"value" db:"value"`
	}

	CreateTrackerRequest struct {
		Name        string  `json:"name"`
		Visibility  string  `json:"visibility"` // required: "public"|"private"
		Type        string  `json:"type"`       // "tracker" or "coverage", defaults to "tracker"
		RepoID      *int64  `json:"repo_id"`    // required if type="coverage"
		ChartConfig *string `json:"chart_config"`
	}

	CreateSeriesRequest struct {
		Name     string  `json:"name"`
		DataType string  `json:"data_type"`
		Config   *string `json:"config"`
	}

	CreateValueRequest struct {
		Timestamp time.Time `json:"time"`
		Value     float64   `json:"value"`
	}

	PatchSeriesRequest struct {
		Name     *string `json:"name"`
		DataType *string `json:"data_type"`
		Config   *string `json:"config"`
	}

	PatchTrackerRequest struct {
		Visibility  *string `json:"visibility"`
		ChartConfig *string `json:"chart_config"`
	}

	ListTrackersResponse struct {
		Trackers []TrackerResponse `json:"trackers"`
		Total    int               `json:"total"`
		Page     int               `json:"page"`
		PerPage  int               `json:"per_page"`
	}

	PreviewSeriesValues struct {
		Series SeriesModel  `json:"series"`
		Values []ValueModel `json:"values"`
	}

	PreviewResponse struct {
		Tracker TrackerResponse      `json:"tracker"`
		Series  []PreviewSeriesValues `json:"series"`
	}

	ListSeriesResponse struct {
		Tracker TrackerResponse `json:"tracker"`
		Series  []SeriesModel   `json:"series"`
	}

	ListValuesResponse struct {
		Series SeriesModel  `json:"series"`
		Values []ValueModel `json:"values"`
	}
)

type contextKey int

const (
	trackerContextKey contextKey = iota
	seriesContextKey
	authCtxKey
	userIDCtxKey
)

func withTracker(ctx context.Context, tracker TrackerModel) context.Context {
	return context.WithValue(ctx, trackerContextKey, tracker)
}

func trackerFrom(ctx context.Context) (TrackerModel, bool) {
	m, ok := ctx.Value(trackerContextKey).(TrackerModel)
	return m, ok
}

func withSeries(ctx context.Context, series SeriesModel) context.Context {
	return context.WithValue(ctx, seriesContextKey, series)
}

func seriesFrom(ctx context.Context) (SeriesModel, bool) {
	s, ok := ctx.Value(seriesContextKey).(SeriesModel)
	return s, ok
}

func ContextWithAuth(ctx context.Context, userID *int64) context.Context {
	ctx = context.WithValue(ctx, authCtxKey, true)
	if userID != nil {
		ctx = context.WithValue(ctx, userIDCtxKey, *userID)
	}
	return ctx
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	uid, ok := ctx.Value(userIDCtxKey).(int64)
	return uid, ok
}

func isAuthenticated(ctx context.Context) bool {
	v, ok := ctx.Value(authCtxKey).(bool)
	return ok && v
}

func renderNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// ----------------------------------------------------------------------
// Auth

func (h *trackerHandler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAuthenticated(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}
		r = r.WithContext(ContextWithAuth(r.Context(), nil))
		next.ServeHTTP(w, r)
	})
}

func (h *trackerHandler) requireEditPermission(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := UserIDFromContext(r.Context())
		if !ok || uid == 0 {
			render.Forbidden(w, errors.New("anonymous users cannot edit"))
			return
		}
		if uid == 1 {
			next.ServeHTTP(w, r)
			return
		}
		tracker, ok := trackerFrom(r.Context())
		if !ok {
			render.BadRequest(w, errors.New("no tracker in context"))
			return
		}
		member, _, err := h.store.isMember(uid, tracker.Id)
		if err != nil {
			log.Error().Err(err).Msg("requireEditPermission isMember")
			render.InternalError(w, err)
			return
		}
		if !member {
			render.Forbidden(w, errors.New("not a tracker member"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *trackerHandler) requireReadPermission(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracker, ok := trackerFrom(r.Context())
		if !ok {
			render.BadRequest(w, errors.New("no tracker in context"))
			return
		}
		if tracker.Visibility == "public" {
			next.ServeHTTP(w, r)
			return
		}
		uid, ok := UserIDFromContext(r.Context())
		if !ok {
			render.Forbidden(w, errors.New("this tracker is private"))
			return
		}
		if uid == 1 {
			next.ServeHTTP(w, r)
			return
		}
		member, _, err := h.store.isMember(uid, tracker.Id)
		if err != nil {
			log.Error().Err(err).Msg("requireReadPermission isMember")
			render.InternalError(w, err)
			return
		}
		if !member {
			render.Forbidden(w, errors.New("this tracker is private"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ----------------------------------------------------------------------
// Tracker

// ListTrackers godoc
// @Summary      List trackers for current user
// @Description  Return trackers owned, edited, or liked by the current user
// @Tags         tracker
// @Success      200  {object}  tracker.ListTrackersResponse
// @Failure      401  {object}  core.ErrorResponse
// @Router       /api/trackers [get]
func (h *trackerHandler) listTrackers(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFromContext(r.Context())
	if !ok {
		uid = 0
	}

	q := r.URL.Query().Get("q")

	if q == "" && !ok {
		render.JSON(w, ListTrackersResponse{
			Trackers: []TrackerResponse{}, Total: 0, Page: 1, PerPage: 0,
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

	trackers, total, err := h.store.listTrackers(uid, q, page, perPage)
	if err != nil {
		log.Error().Err(err).Msg("tracker.handler.listTrackers")
		render.InternalError(w, err)
		return
	}

	render.JSON(w, ListTrackersResponse{
		Trackers: trackers,
		Total:    total,
		Page:     page,
		PerPage:  perPage,
	}, http.StatusOK)
}

// CreateTracker godoc
// @Summary      Create a tracker
// @Description  Add a new tracker. The name must be unique. Creator becomes owner.
// @Tags         tracker
// @Accept       json
// @Produce      json
// @Param        body  body      tracker.CreateTrackerRequest  true  "Tracker information"
// @Success      201   {object}  tracker.TrackerModel
// @Failure      400   {object}  core.ErrorResponse
// @Failure      401   {object}  core.ErrorResponse
// @Router       /api/trackers [post]
func (h *trackerHandler) createTracker(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	var req CreateTrackerRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn().Err(err).Msg("invalid tracker request body")
		render.BadRequest(w, errors.New("invalid request body"))
		return
	}

	if req.Name == "" {
		render.BadRequest(w, errors.New("name is required"))
		return
	}
	if req.Visibility != "public" && req.Visibility != "private" {
		render.BadRequest(w, errors.New("visibility must be one of: public, private"))
		return
	}
	if req.Type == "" {
		req.Type = "tracker"
	}
	if req.Type != "tracker" && req.Type != "coverage" {
		render.BadRequest(w, errors.New("type must be 'tracker' or 'coverage'"))
		return
	}
	if req.Type == "coverage" && req.RepoID == nil {
		render.BadRequest(w, errors.New("repo_id is required for coverage type"))
		return
	}
	if req.Type == "tracker" && req.RepoID != nil {
		render.BadRequest(w, errors.New("repo_id is not allowed for tracker type"))
		return
	}

	uid, ok := UserIDFromContext(r.Context())
	if !ok {
		render.Forbidden(w, errors.New("anonymous users cannot create trackers"))
		return
	}

	tracker := TrackerModel{
		Name: req.Name, Visibility: req.Visibility, Type: req.Type,
		ChartConfig: "{}",
	}
	if req.ChartConfig != nil {
		if !json.Valid([]byte(*req.ChartConfig)) {
			render.BadRequest(w, errors.New("chart_config must be valid JSON"))
			return
		}
		tracker.ChartConfig = *req.ChartConfig
	}
	err = h.store.addTracker(&tracker, uid, req.RepoID)
	if err != nil {
		log.Warn().Err(err).Msg("addTracker")
		render.BadRequest(w, errors.New("failed to create tracker"))
		return
	}

	resp := TrackerResponse{
		Id:          tracker.Id,
		Name:        tracker.Name,
		Visibility:  tracker.Visibility,
		Type:        tracker.Type,
		RepoID:      req.RepoID,
		ChartConfig: tracker.ChartConfig,
		Role:        "owner",
		Liked:       false,
	}
	render.JSON(w, resp, http.StatusCreated)
}

// DeleteTracker godoc
// @Summary      Delete a tracker
// @Description  Delete the specified tracker. Child series and values are also cascade-deleted.
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/trackers/{trackerId} [delete]
func (h *trackerHandler) deleteTracker(w http.ResponseWriter, r *http.Request) {
	tracker, _ := trackerFrom(r.Context())
	err := h.store.deleteTracker(tracker.Id)
	if err != nil {
		log.Error().Err(err).Msg("deleteTracker")
		render.InternalError(w, err)
		return
	}

	renderNoContent(w)
}

// ----------------------------------------------------------------------
// Tracker (mutation)

// PatchTracker godoc
// @Summary      Update a tracker
// @Description  Update tracker fields (e.g. visibility). Requires edit permission.
// @Tags         tracker
// @Accept       json
// @Produce      json
// @Param        trackerId  path  int                      true  "Tracker ID"
// @Param        body       body  tracker.PatchTrackerRequest  true  "Fields to update"
// @Success      200  {object}  tracker.TrackerResponse
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/trackers/{trackerId} [patch]
func (h *trackerHandler) getTracker(w http.ResponseWriter, r *http.Request) {
	tracker, _ := trackerFrom(r.Context())
	uid, _ := UserIDFromContext(r.Context())

	resp, err := h.store.findTrackerResponseById(tracker.Id, uid)
	if err != nil {
		log.Error().Err(err).Msg("getTracker findTrackerResponseById")
		render.InternalError(w, err)
		return
	}
	render.JSON(w, resp, http.StatusOK)
}

func (h *trackerHandler) patchTracker(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	var req PatchTrackerRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn().Err(err).Msg("invalid patch body")
		render.BadRequest(w, errors.New("invalid request body"))
		return
	}

	if req.Visibility != nil {
		v := *req.Visibility
		if v != "public" && v != "private" {
			render.BadRequest(w, errors.New("visibility must be 'public' or 'private'"))
			return
		}
	}

	if req.ChartConfig != nil {
		if !json.Valid([]byte(*req.ChartConfig)) {
			render.BadRequest(w, errors.New("chart_config must be valid JSON"))
			return
		}
	}

	tracker, _ := trackerFrom(r.Context())
	err = h.store.updateTracker(tracker.Id, req.Visibility, req.ChartConfig)
	if err != nil {
		log.Error().Err(err).Msg("patchTracker updateTracker")
		render.InternalError(w, err)
		return
	}

	uid, _ := UserIDFromContext(r.Context())
	if req.Visibility != nil {
		tracker.Visibility = *req.Visibility
	}
	if req.ChartConfig != nil {
		tracker.ChartConfig = *req.ChartConfig
	}

	resp := TrackerResponse{
		Id:          tracker.Id,
		Name:        tracker.Name,
		Visibility:  tracker.Visibility,
		Type:        tracker.Type,
		RepoID:      tracker.RepoID,
		ChartConfig: tracker.ChartConfig,
	}
	if member, role, err := h.store.isMember(uid, tracker.Id); err == nil && member {
		resp.Role = role
	}
	if liked, err := h.store.isLiked(uid, tracker.Id); err == nil {
		resp.Liked = liked
	}

	render.JSON(w, resp, http.StatusOK)
}

// ----------------------------------------------------------------------
// Preview

// PreviewTracker godoc
// @Summary      Preview tracker data
// @Description  Return tracker info, all series, and latest values (up to 20 per series) for card previews
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Success      200  {object}  tracker.PreviewResponse
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/trackers/{trackerId}/preview [get]
func (h *trackerHandler) previewTracker(w http.ResponseWriter, r *http.Request) {
	tracker, _ := trackerFrom(r.Context())

	var previews []PreviewSeriesValues

	if tracker.Type == "coverage" {
		repoID, err := h.store.findRepoIDByTrackerID(tracker.Id)
		if err != nil {
			log.Error().Err(err).Msg("tracker.handler.previewTracker findRepoID")
			render.InternalError(w, err)
			return
		}
		if repoID != nil && h.coverageProvider != nil {
			timeline, err := h.coverageProvider.Timeline(*repoID, 20)
			if err != nil {
				log.Error().Err(err).Msg("tracker.handler.previewTracker Timeline")
				render.InternalError(w, err)
				return
			}
			for name, points := range timeline {
				values := make([]ValueModel, len(points))
				for i, p := range points {
					values[i] = ValueModel{Timestamp: p.Time, Value: p.Value}
				}
				previews = append(previews, PreviewSeriesValues{
					Series: SeriesModel{
						Id:        0,
						TrackerId: tracker.Id,
						Name:      name,
						DataType:  "float",
						Config:    `{"value_format":"%.1f%%"}`,
					},
					Values: values,
				})
			}
		}
	} else {
		series, err := h.store.listSeries(tracker.Id)
		if err != nil {
			log.Error().Err(err).Msg("tracker.handler.previewTracker listSeries")
			render.InternalError(w, err)
			return
		}

		for _, s := range series {
			values, err := h.store.listLatestValues(s.Id, 20)
			if err != nil {
				log.Error().Err(err).Msg("tracker.handler.previewTracker listLatestValues")
				continue
			}
			previews = append(previews, PreviewSeriesValues{
				Series: s,
				Values: values,
			})
		}
	}

	trackerResp := TrackerResponse{
		Id: tracker.Id, Name: tracker.Name, Visibility: tracker.Visibility, Type: tracker.Type, RepoID: tracker.RepoID,
		ChartConfig: tracker.ChartConfig,
	}
	if uid, ok := UserIDFromContext(r.Context()); ok {
		if member, role, err := h.store.isMember(uid, tracker.Id); err == nil && member {
			trackerResp.Role = role
		}
		if liked, err := h.store.isLiked(uid, tracker.Id); err == nil {
			trackerResp.Liked = liked
		}
	}

	render.JSON(w, PreviewResponse{Tracker: trackerResp, Series: previews}, http.StatusOK)
}

// ----------------------------------------------------------------------
// Series

// ListSeries godoc
// @Summary      List all series
// @Description  Return all series belonging to the specified tracker
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Success      200  {object}  tracker.ListSeriesResponse
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/trackers/{trackerId}/series [get]
func (h *trackerHandler) listSeries(w http.ResponseWriter, r *http.Request) {
	tracker, _ := trackerFrom(r.Context())

	var series []SeriesModel
	if tracker.Type != "coverage" {
		var err error
		series, err = h.store.listSeries(tracker.Id)
		if err != nil {
			log.Error().Err(err).Msg("tracker.handler.listSeries")
			render.InternalError(w, err)
			return
		}
	}

	trackerResp := TrackerResponse{Id: tracker.Id, Name: tracker.Name, Visibility: tracker.Visibility, Type: tracker.Type, RepoID: tracker.RepoID, ChartConfig: tracker.ChartConfig}
	if uid, ok := UserIDFromContext(r.Context()); ok {
		_, role, err := h.store.isMember(uid, tracker.Id)
		if err == nil {
			trackerResp.Role = role
		}
		liked, err := h.store.isLiked(uid, tracker.Id)
		if err == nil {
			trackerResp.Liked = liked
		}
	}
	if likeCount, err := h.store.countLikes(tracker.Id); err == nil {
		trackerResp.LikeCount = likeCount
	}

	resp := ListSeriesResponse{
		Tracker: trackerResp,
		Series:  series,
	}

	render.JSON(w, resp, http.StatusOK)
}

// CreateSeries godoc
// @Summary      Create a series
// @Description  Add a new series under a tracker. data_type must be "int" or "float" (defaults to "float").
// @Tags         tracker
// @Accept       json
// @Produce      json
// @Param        trackerId  path  int                       true  "Tracker ID"
// @Param        body       body  tracker.CreateSeriesRequest  true  "Series information"
// @Success      201  {object}  tracker.SeriesModel
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/trackers/{trackerId}/series [post]
func (h *trackerHandler) createSeries(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	tracker, _ := trackerFrom(r.Context())
	if tracker.Type == "coverage" {
		render.BadRequest(w, errors.New("cannot modify series for coverage tracker"))
		return
	}

	var req CreateSeriesRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn().Err(err).Msg("tracker.handler.createSeries")
		render.BadRequest(w, errors.New("invalid request body"))
		return
	}

	dataType := req.DataType
	if dataType == "" {
		dataType = "float"
	}
	if dataType != "int" && dataType != "float" {
		render.BadRequest(w, errors.New("invalid data_type: must be 'int' or 'float'"))
		return
	}

	config := "{}"
	if req.Config != nil {
		if !json.Valid([]byte(*req.Config)) {
			render.BadRequest(w, errors.New("config must be valid JSON"))
			return
		}
		config = *req.Config
	}

	series := SeriesModel{
		TrackerId: tracker.Id,
		Name:      req.Name,
		DataType:  dataType,
		Config:    config,
	}

	err = h.store.addSeries(&series)
	if err != nil {
		log.Warn().Err(err).Msg("createSeries")
		render.BadRequest(w, errors.New("failed to create series"))
		return
	}

	render.JSON(w, series, http.StatusCreated)
}

// DeleteSeries godoc
// @Summary      Delete a series
// @Description  Delete the specified series. Child values are also cascade-deleted.
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Param        seriesId   path  int  true  "Series ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/trackers/{trackerId}/series/{seriesId} [delete]
func (h *trackerHandler) deleteSeries(w http.ResponseWriter, r *http.Request) {
	tracker, _ := trackerFrom(r.Context())
	if tracker.Type == "coverage" {
		render.BadRequest(w, errors.New("cannot modify series for coverage tracker"))
		return
	}

	series, _ := seriesFrom(r.Context())

	err := h.store.deleteSeries(series.Id)
	if err != nil {
		log.Warn().Err(err).Msg("deleteSeries")
		render.InternalError(w, err)
		return
	}

	renderNoContent(w)
}

// PatchSeries godoc
// @Summary      Update a series
// @Description  Update fields of a series (e.g. data_type, config). Requires edit permission.
// @Tags         tracker
// @Accept       json
// @Produce      json
// @Param        trackerId  path  int                      true  "Tracker ID"
// @Param        seriesId   path  int                      true  "Series ID"
// @Param        body       body  tracker.PatchSeriesRequest  true  "Fields to update"
// @Success      200  {object}  tracker.SeriesModel
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/trackers/{trackerId}/series/{seriesId} [patch]
func (h *trackerHandler) patchSeries(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	tracker, _ := trackerFrom(r.Context())
	if tracker.Type == "coverage" {
		render.BadRequest(w, errors.New("cannot modify series for coverage tracker"))
		return
	}

	var req PatchSeriesRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn().Err(err).Msg("tracker.handler.patchSeries")
		render.BadRequest(w, errors.New("invalid request body"))
		return
	}

	if req.DataType != nil {
		dt := *req.DataType
		if dt != "int" && dt != "float" {
			render.BadRequest(w, errors.New("invalid data_type: must be 'int' or 'float'"))
			return
		}
	}

	if req.Config != nil {
		if !json.Valid([]byte(*req.Config)) {
			render.BadRequest(w, errors.New("config must be valid JSON"))
			return
		}
	}

	series, _ := seriesFrom(r.Context())
	err = h.store.updateSeries(series.Id, req.Name, req.DataType, req.Config)
	if err != nil {
		log.Error().Err(err).Msg("patchSeries updateSeries")
		render.InternalError(w, err)
		return
	}

	// Reload the updated series
	updated, err := h.store.findSeriesById(series.Id)
	if err != nil {
		log.Error().Err(err).Msg("patchSeries findSeriesById")
		render.InternalError(w, err)
		return
	}

	render.JSON(w, updated, http.StatusOK)
}

// ----------------------------------------------------------------------
// Value

// ListValues godoc
// @Summary      List all values
// @Description  Return time-series data for the specified series. The limit parameter restricts the maximum number of results.
// @Tags         tracker
// @Param        trackerId  path  int     true   "Tracker ID"
// @Param        seriesId   path  int     true   "Series ID"
// @Param        limit      query int     false  "Maximum number of results"
// @Success      200  {object}  tracker.ListValuesResponse
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/trackers/{trackerId}/series/{seriesId}/values [get]
func (h *trackerHandler) listValues(w http.ResponseWriter, r *http.Request) {
	series, _ := seriesFrom(r.Context())

	limitStr := r.URL.Query().Get("limit")
	var limit int
	if limitStr != "" {
		n, err := strconv.Atoi(limitStr)
		if err != nil || n < 0 {
			render.BadRequest(w, errors.New("invalid limit"))
			return
		}
		limit = n
	}

	values, err := h.store.listValues(series.Id, limit)
	if err != nil {
		log.Error().Err(err).Msg("tracker.handler.listValues")
		render.InternalError(w, err)
		return
	}

	resp := ListValuesResponse{
		Series: series,
		Values: values,
	}

	render.JSON(w, resp, http.StatusOK)
}

// CreateValue godoc
// @Summary      Add a value
// @Description  Add time-series data to a series. Duplicate timestamps within the same series are not allowed.
// @Tags         tracker
// @Accept       json
// @Produce      json
// @Param        trackerId  path  int                     true  "Tracker ID"
// @Param        seriesId   path  int                     true  "Series ID"
// @Param        body       body  tracker.CreateValueRequest  true  "Value data"
// @Success      201  {object}  tracker.ValueModel
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/trackers/{trackerId}/series/{seriesId}/values [post]
func (h *trackerHandler) createValue(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	tracker, _ := trackerFrom(r.Context())
	if tracker.Type == "coverage" {
		render.BadRequest(w, errors.New("cannot modify values for coverage tracker"))
		return
	}

	var req CreateValueRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn().Err(err).Msg("invalid value request body")
		render.BadRequest(w, errors.New("invalid request body"))
		return
	}

	series, _ := seriesFrom(r.Context())
	value := ValueModel{
		SeriesId:  series.Id,
		Timestamp: req.Timestamp,
		Value:     req.Value,
	}

	err = h.store.addValue(&value)
	if err != nil {
		log.Error().Err(err).Msg("addValue")
		render.InternalError(w, err)
		return
	}

	render.JSON(w, value, http.StatusCreated)
}

// DeleteValues godoc
// @Summary      Delete all values
// @Description  Delete all time-series data for the specified series
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Param        seriesId   path  int  true  "Series ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/trackers/{trackerId}/series/{seriesId}/values [delete]
func (h *trackerHandler) deleteValues(w http.ResponseWriter, r *http.Request) {
	tracker, _ := trackerFrom(r.Context())
	if tracker.Type == "coverage" {
		render.BadRequest(w, errors.New("cannot modify values for coverage tracker"))
		return
	}

	series, _ := seriesFrom(r.Context())
	err := h.store.deleteValues(series.Id)
	if err != nil {
		log.Error().Err(err).Msg("deleteValues")
		render.InternalError(w, err)
		return
	}

	renderNoContent(w)
}

// ----------------------------------------------------------------------
// Like

// LikeTracker godoc
// @Summary      Like a tracker
// @Description  Add a like to the specified tracker for the current user
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Success      201
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/trackers/{trackerId}/like [post]
func (h *trackerHandler) likeTracker(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFromContext(r.Context())
	if !ok {
		render.Forbidden(w, errors.New("anonymous users cannot like"))
		return
	}
	tracker, _ := trackerFrom(r.Context())
	err := h.store.addLike(uid, tracker.Id)
	if err != nil {
		log.Error().Err(err).Msg("likeTracker")
		render.InternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// UnlikeTracker godoc
// @Summary      Unlike a tracker
// @Description  Remove a like from the specified tracker for the current user
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/trackers/{trackerId}/like [delete]
func (h *trackerHandler) unlikeTracker(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFromContext(r.Context())
	if !ok {
		render.Forbidden(w, errors.New("anonymous users cannot unlike"))
		return
	}
	tracker, _ := trackerFrom(r.Context())
	err := h.store.removeLike(uid, tracker.Id)
	if err != nil {
		log.Error().Err(err).Msg("unlikeTracker")
		render.InternalError(w, err)
		return
	}
	renderNoContent(w)
}

// ----------------------------------------------------------------------
// Middleware

func (h *trackerHandler) injectTracker(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "trackerId"), 10, 64)
		if err != nil {
			log.Warn().Err(err).Msg("tracker.handler.injectTracker")
			render.BadRequest(w, errors.New("invalid tracker id"))
			return
		}

		tracker, err := h.store.findTrackerById(id)
		if err == errorTrackerNotFound {
			render.NotFound(w, errors.New("tracker not found"))
			return
		} else if err != nil {
			log.Warn().Err(err).Msg("tracker.handler.injectTracker")
			render.InternalError(w, err)
			return
		}

		r = r.WithContext(withTracker(r.Context(), *tracker))
		next.ServeHTTP(w, r)
	})
}

func (h *trackerHandler) injectSeries(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seriesId, err := strconv.ParseInt(chi.URLParam(r, "seriesId"), 10, 64)
		if err != nil {
			log.Warn().Err(err).Msg("invalid seriesId in URL")
			render.BadRequest(w, errors.New("invalid series id"))
			return
		}

		series, err := h.store.findSeriesById(seriesId)
		if err == errorSeriesNotFound {
			render.NotFound(w, errors.New("series not found"))
			return
		} else if err != nil {
			log.Warn().Err(err).Msg("tracker.handler.injectSeries")
			render.InternalError(w, err)
			return
		}

		// Verify series belongs to the tracker in URL
		tracker, _ := trackerFrom(r.Context())
		if series.TrackerId != tracker.Id {
			render.NotFound(w, errors.New("series not found"))
			return
		}

		r = r.WithContext(withSeries(r.Context(), *series))
		next.ServeHTTP(w, r)
	})
}

func newHandler(store *trackerStore, cp CoverageTimelineProvider) http.Handler {
	h := &trackerHandler{store: store, coverageProvider: cp}
	r := chi.NewRouter()

	r.Use(h.requireAuth)

	r.Route("/", func(r chi.Router) {
		r.Get("/", h.listTrackers)
		r.Post("/", h.createTracker)

		r.Route("/{trackerId}", func(r chi.Router) {
			r.Use(h.injectTracker)
			r.Use(h.requireReadPermission)
			r.Get("/", h.getTracker)
			r.With(h.requireEditPermission).Delete("/", h.deleteTracker)
			r.With(h.requireEditPermission).Patch("/", h.patchTracker)
			r.Post("/like", h.likeTracker)
			r.Delete("/like", h.unlikeTracker)
			r.Get("/preview", h.previewTracker)

			r.Route("/series", func(r chi.Router) {
				r.Get("/", h.listSeries)
				r.With(h.requireEditPermission).Post("/", h.createSeries)

				r.Route("/{seriesId}", func(r chi.Router) {
					r.Use(h.injectSeries)
					r.With(h.requireEditPermission).Patch("/", h.patchSeries)
					r.With(h.requireEditPermission).Delete("/", h.deleteSeries)

					r.Route("/values", func(r chi.Router) {
						r.Get("/", h.listValues)
						r.With(h.requireEditPermission).Post("/", h.createValue)
						r.With(h.requireEditPermission).Delete("/", h.deleteValues)
					})
				})
			})
		})
	})

	return r
}
