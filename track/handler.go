package track

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/render"
	"github.com/rs/zerolog/log"
)

type (
	trackHandler struct {
		store  *trackStore
		apiKey string
	}

	TrackModel struct {
		Id   int64  `json:"id"   db:"id"`
		Name string `json:"name" db:"name"`
	}

	SeriesModel struct {
		Id       int64  `json:"id"        db:"id"`
		TrackId  int64  `json:"track_id"  db:"track_id"`
		Name     string `json:"name"      db:"name"`
		DataType string `json:"data_type" db:"data_type"`
	}

	ValueModel struct {
		Id        int64     `db:"id"`
		SeriesId  int64     `db:"series_id"`
		Timestamp time.Time `json:"time"  db:"time"`
		Value     float64   `json:"value" db:"value"`
	}

	CreateTrackRequest struct {
		Name string `json:"name"`
	}

	CreateSeriesRequest struct {
		Name     string `json:"name"`
		DataType string `json:"data_type"`
	}

	CreateValueRequest struct {
		Timestamp time.Time `json:"time"`
		Value     float64   `json:"value"`
	}

	ListTracksResponse struct {
		Tracks []TrackModel `json:"tracks"`
	}

	ListSeriesResponse struct {
		Track  TrackModel    `json:"track"`
		Series []SeriesModel `json:"series"`
	}

	ListValuesResponse struct {
		Series SeriesModel  `json:"series"`
		Values []ValueModel `json:"values"`
	}
)

type contextKey int

const (
	trackContextKey contextKey = iota
	seriesContextKey
)

func withTrack(ctx context.Context, track TrackModel) context.Context {
	return context.WithValue(ctx, trackContextKey, track)
}

func trackFrom(ctx context.Context) (TrackModel, bool) {
	m, ok := ctx.Value(trackContextKey).(TrackModel)
	return m, ok
}

func withSeries(ctx context.Context, series SeriesModel) context.Context {
	return context.WithValue(ctx, seriesContextKey, series)
}

func seriesFrom(ctx context.Context) (SeriesModel, bool) {
	s, ok := ctx.Value(seriesContextKey).(SeriesModel)
	return s, ok
}

func renderNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// ----------------------------------------------------------------------
// Auth

func (h *trackHandler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.apiKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == h.apiKey {
			next.ServeHTTP(w, r)
			return
		}
		render.Unauthorized(w, render.ErrInvalidToken)
	})
}

// ----------------------------------------------------------------------
// Track

// ListTracks godoc
// @Summary      List all tracks
// @Description  Return all registered tracks
// @Tags         track
// @Success      200  {object}  track.ListTracksResponse
// @Failure      401  {object}  core.ErrorResponse
// @Router       /api/track [get]
func (h *trackHandler) listTracks(w http.ResponseWriter, r *http.Request) {
	tracks, err := h.store.listTracks()
	if err != nil {
		log.Error().Err(err).Msg("track.handler.listTracks")
		render.InternalError(w, err)
		return
	}

	resp := ListTracksResponse{
		Tracks: tracks,
	}

	render.JSON(w, resp, http.StatusOK)
}

// CreateTrack godoc
// @Summary      Create a track
// @Description  Add a new track. The name must be unique.
// @Tags         track
// @Accept       json
// @Produce      json
// @Param        body  body      track.CreateTrackRequest  true  "Track information"
// @Success      201   {object}  track.TrackModel
// @Failure      400   {object}  core.ErrorResponse
// @Failure      401   {object}  core.ErrorResponse
// @Router       /api/track [post]
func (h *trackHandler) createTrack(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	var req CreateTrackRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn().Err(err).Msg("invalid track request body")
		render.BadRequest(w, errors.New("invalid request body"))
		return
	}

	track := TrackModel{Name: req.Name}
	err = h.store.addTrack(&track)
	if err != nil {
		log.Warn().Err(err).Msg("addTrack")
		render.BadRequest(w, errors.New("failed to create track"))
		return
	}

	render.JSON(w, track, http.StatusCreated)
}

// DeleteTrack godoc
// @Summary      Delete a track
// @Description  Delete the specified track. Child series and values are also cascade-deleted.
// @Tags         track
// @Param        trackId  path  int  true  "Track ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId} [delete]
func (h *trackHandler) deleteTrack(w http.ResponseWriter, r *http.Request) {
	track, _ := trackFrom(r.Context())
	err := h.store.deleteTrack(track.Id)
	if err != nil {
		log.Error().Err(err).Msg("deleteTrack")
		render.InternalError(w, err)
		return
	}

	renderNoContent(w)
}

// ----------------------------------------------------------------------
// Series

// ListSeries godoc
// @Summary      List all series
// @Description  Return all series belonging to the specified track
// @Tags         track
// @Param        trackId  path  int  true  "Track ID"
// @Success      200  {object}  track.ListSeriesResponse
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/series [get]
func (h *trackHandler) listSeries(w http.ResponseWriter, r *http.Request) {
	track, _ := trackFrom(r.Context())

	series, err := h.store.listSeries(track.Id)
	if err != nil {
		log.Error().Err(err).Msg("track.handler.listSeries")
		render.InternalError(w, err)
		return
	}

	resp := ListSeriesResponse{
		Track:  track,
		Series: series,
	}

	render.JSON(w, resp, http.StatusOK)
}

// CreateSeries godoc
// @Summary      Create a series
// @Description  Add a new series under a track. data_type must be "int" or "float" (defaults to "float").
// @Tags         track
// @Accept       json
// @Produce      json
// @Param        trackId  path  int                     true  "Track ID"
// @Param        body     body  track.CreateSeriesRequest  true  "Series information"
// @Success      201  {object}  track.SeriesModel
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/series [post]
func (h *trackHandler) createSeries(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	var req CreateSeriesRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn().Err(err).Msg("track.handler.createSeries")
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

	track, _ := trackFrom(r.Context())
	series := SeriesModel{
		TrackId:  track.Id,
		Name:     req.Name,
		DataType: dataType,
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
// @Tags         track
// @Param        trackId   path  int  true  "Track ID"
// @Param        seriesId  path  int  true  "Series ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/series/{seriesId} [delete]
func (h *trackHandler) deleteSeries(w http.ResponseWriter, r *http.Request) {
	series, _ := seriesFrom(r.Context())

	err := h.store.deleteSeries(series.Id)
	if err != nil {
		log.Warn().Err(err).Msg("deleteSeries")
		render.InternalError(w, err)
		return
	}

	renderNoContent(w)
}

// ----------------------------------------------------------------------
// Value

// ListValues godoc
// @Summary      List all values
// @Description  Return time-series data for the specified series. The limit parameter restricts the maximum number of results.
// @Tags         track
// @Param        trackId   path  int     true   "Track ID"
// @Param        seriesId  path  int     true   "Series ID"
// @Param        limit     query int     false  "Maximum number of results"
// @Success      200  {object}  track.ListValuesResponse
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/series/{seriesId}/values [get]
func (h *trackHandler) listValues(w http.ResponseWriter, r *http.Request) {
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
		log.Error().Err(err).Msg("track.handler.listValues")
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
// @Tags         track
// @Accept       json
// @Produce      json
// @Param        trackId   path  int                   true  "Track ID"
// @Param        seriesId  path  int                   true  "Series ID"
// @Param        body      body  track.CreateValueRequest  true  "Value data"
// @Success      201  {object}  track.ValueModel
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/series/{seriesId}/values [post]
func (h *trackHandler) createValue(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

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
// @Tags         track
// @Param        trackId   path  int  true  "Track ID"
// @Param        seriesId  path  int  true  "Series ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/series/{seriesId}/values [delete]
func (h *trackHandler) deleteValues(w http.ResponseWriter, r *http.Request) {
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
// Middleware

func (h *trackHandler) injectTrack(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "trackId"), 10, 64)
		if err != nil {
			log.Warn().Err(err).Msg("track.handler.injectTrack")
			render.BadRequest(w, errors.New("invalid track id"))
			return
		}

		track, err := h.store.findTrackById(id)
		if err == errorTrackNotFound {
			render.NotFound(w, errors.New("track not found"))
			return
		} else if err != nil {
			log.Warn().Err(err).Msg("track.handler.injectTrack")
			render.InternalError(w, err)
			return
		}

		r = r.WithContext(withTrack(r.Context(), *track))
		next.ServeHTTP(w, r)
	})
}

func (h *trackHandler) injectSeries(next http.Handler) http.Handler {
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
			log.Warn().Err(err).Msg("track.handler.injectSeries")
			render.InternalError(w, err)
			return
		}

		// Verify series belongs to the track in URL
		track, _ := trackFrom(r.Context())
		if series.TrackId != track.Id {
			render.NotFound(w, errors.New("series not found"))
			return
		}

		r = r.WithContext(withSeries(r.Context(), *series))
		next.ServeHTTP(w, r)
	})
}

func newHandler(store *trackStore, apiKey string) http.Handler {
	h := &trackHandler{store: store, apiKey: apiKey}
	r := chi.NewRouter()

	r.Use(h.requireAuth)

	r.Route("/", func(r chi.Router) {
		r.Get("/", h.listTracks)
		r.Post("/", h.createTrack)

		r.Route("/{trackId}", func(r chi.Router) {
			r.Use(h.injectTrack)
			r.Delete("/", h.deleteTrack)

			r.Route("/series", func(r chi.Router) {
				r.Get("/", h.listSeries)
				r.Post("/", h.createSeries)

				r.Route("/{seriesId}", func(r chi.Router) {
					r.Use(h.injectSeries)
					r.Delete("/", h.deleteSeries)

					r.Route("/values", func(r chi.Router) {
						r.Get("/", h.listValues)
						r.Post("/", h.createValue)
						r.Delete("/", h.deleteValues)
					})
				})
			})
		})
	})

	return r
}
