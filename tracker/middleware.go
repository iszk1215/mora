package tracker

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/render"
	"github.com/rs/zerolog/log"
)

func InjectTracker(store *trackerStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "trackerId"), 10, 64)
		if err != nil {
			log.Warn().Err(err).Msg("tracker.middleware.InjectTracker")
			render.BadRequest(w, errors.New("invalid tracker id"))
			return
		}

		tracker, err := store.findTrackerById(id)
		if err == errorTrackerNotFound {
			render.NotFound(w, errors.New("tracker not found"))
			return
		} else if err != nil {
			log.Warn().Err(err).Msg("tracker.middleware.InjectTracker")
			render.InternalError(w, err)
			return
		}

		r = r.WithContext(withTracker(r.Context(), *tracker))
		next.ServeHTTP(w, r)
	})
}

func RequireReadPermission(store *trackerStore, next http.Handler) http.Handler {
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
			render.NotFound(w, errors.New("tracker not found"))
			return
		}
		if store.isSuperuser(uid) {
			next.ServeHTTP(w, r)
			return
		}
		member, _, err := store.isMember(uid, tracker.Id)
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

func RequireEditPermission(store *trackerStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := UserIDFromContext(r.Context())
		if !ok || uid == 0 {
			render.NotFound(w, errors.New("tracker not found"))
			return
		}
		if store.isSuperuser(uid) {
			next.ServeHTTP(w, r)
			return
		}
		tracker, ok := trackerFrom(r.Context())
		if !ok {
			render.BadRequest(w, errors.New("no tracker in context"))
			return
		}
		member, _, err := store.isMember(uid, tracker.Id)
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

func RequireOwnerPermission(store *trackerStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := UserIDFromContext(r.Context())
		if !ok || uid == 0 {
			render.NotFound(w, errors.New("tracker not found"))
			return
		}
		if store.isSuperuser(uid) {
			next.ServeHTTP(w, r)
			return
		}
		tracker, ok := trackerFrom(r.Context())
		if !ok {
			render.BadRequest(w, errors.New("no tracker in context"))
			return
		}
		if tracker.OwnerId != uid {
			render.NotFound(w, errors.New("tracker not found"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func InjectSeries(store *trackerStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seriesId, err := strconv.ParseInt(chi.URLParam(r, "seriesId"), 10, 64)
		if err != nil {
			log.Warn().Err(err).Msg("invalid seriesId in URL")
			render.BadRequest(w, errors.New("invalid series id"))
			return
		}

		series, err := store.findSeriesById(seriesId)
		if err == errorSeriesNotFound {
			render.NotFound(w, errors.New("series not found"))
			return
		} else if err != nil {
			log.Warn().Err(err).Msg("tracker.middleware.InjectSeries")
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
