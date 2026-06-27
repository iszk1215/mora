package udm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/core"
	"github.com/iszk1215/mora/render"
	"github.com/rs/zerolog/log"
)

type (
	udmHandler struct {
		store *udmStore
	}

	metricModel struct {
		Id     int64  `json:"id"      db:"id"`
		RepoId int64  `json:"repo_id" db:"repo_id"`
		Name   string `json:"name"    db:"name"`
	}

	itemModel struct {
		Id        int64    `json:"id"        db:"id"`
		MetricId  int64    `json:"metric_id" db:"metric_id"`
		Name      string   `json:"name"      db:"name"`
		ValueType ValueType `json:"type"      db:"type"`
	}

	valueModel struct {
		Id        int64     `db:"id"`
		ItemId    int64     `db:"item_id"`
		Timestamp time.Time `json:"time"     db:"time"`
		Value     string    `json:"value"    db:"value"`
	}

	listMetricsResponse struct {
		Repo    core.Repository `json:"repo"`
		Metrics []metricModel   `json:"metrics"`
	}

	listItemsResponse struct {
		Repo   core.Repository `json:"repo"`
		Metric metricModel     `json:"metric"`
		Items  []itemModel     `json:"items"`
	}

	listValuesResponse struct {
		Repo   core.Repository `json:"repo"`
		Item   itemModel       `json:"items"`
		Values []valueModel    `json:"values"`
	}

	ContextKey int
)

const (
	metricContextKey ContextKey = iota
	itemContextKey
)

func withMetric(ctx context.Context, metric metricModel) context.Context {
	return context.WithValue(ctx, metricContextKey, metric)
}

func metricFrom(ctx context.Context) (metricModel, bool) {
	m, ok := ctx.Value(metricContextKey).(metricModel)
	return m, ok
}

func withItem(ctx context.Context, item itemModel) context.Context {
	return context.WithValue(ctx, itemContextKey, item)
}

func itemFrom(ctx context.Context) (itemModel, bool) {
	item, ok := ctx.Value(itemContextKey).(itemModel)
	return item, ok
}

func renderNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// ----------------------------------------------------------------------
// Metric

func (h *udmHandler) createMetric(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	var metric metricModel
	err := json.NewDecoder(r.Body).Decode(&metric)
	if err != nil {
		log.Warn().Err(err).Msg("invalid metric request body")
		render.BadRequest(w, errors.New("invalid request body"))
		return
	}

	repo, _ := core.RepoFrom(r.Context())
	if metric.RepoId == 0 {
		metric.RepoId = repo.Id
	} else if repo.Id != metric.RepoId {
		render.BadRequest(w, errors.New("repository id mismatch"))
		return
	}

	err = h.store.addMetric(&metric)
	if err != nil {
		log.Warn().Err(err).Msg("addMetric")
		render.BadRequest(w, errors.New("failed to create metric"))
		return
	}

	render.JSON(w, metric, http.StatusCreated)
}

func (h *udmHandler) listMetrics(w http.ResponseWriter, r *http.Request) {
	repo, _ := core.RepoFrom(r.Context())
	metrics, err := h.store.listMetrics(repo.Id)
	if err != nil {
		log.Error().Err(err).Msg("udm.handler.listMetrics")
		render.InternalError(w, err)
		return
	}

	resp := listMetricsResponse{
		Repo:    repo,
		Metrics: metrics,
	}

	render.JSON(w, resp, http.StatusOK)
}

func (h *udmHandler) deleteMetric(w http.ResponseWriter, r *http.Request) {
	metric, _ := metricFrom(r.Context())
	err := h.store.deleteMetric(metric.Id)
	if err != nil {
		log.Error().Err(err).Msg("deleteMetric")
		render.InternalError(w, err)
		return
	}

	renderNoContent(w)
}

// ----------------------------------------------------------------------
// item

func (h *udmHandler) createItem(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	var item itemModel
	err := json.NewDecoder(r.Body).Decode(&item)
	if err != nil {
		log.Warn().Err(err).Msg("udm.handler.createItem")
		render.BadRequest(w, errors.New("invalid request body"))
		return
	}

	metric, _ := metricFrom(r.Context())
	if item.MetricId == 0 {
		item.MetricId = metric.Id
	} else if item.MetricId != metric.Id {
		render.BadRequest(w, errors.New("metric id mismatch"))
		return
	}

	err = h.store.addItem(&item)
	if errors.Is(err, errorMetricNotFound) {
		log.Warn().Err(err).Msg("createItem")
		render.NotFound(w, err)
		return
	} else if err != nil {
		log.Error().Err(err).Msg("createItem")
		render.InternalError(w, err)
		return
	}

	render.JSON(w, item, http.StatusCreated)
}

func (h *udmHandler) listItems(w http.ResponseWriter, r *http.Request) {
	log.Info().Msg("udmHandler.listItems")

	repo, _ := core.RepoFrom(r.Context())
	metric, _ := metricFrom(r.Context())

	items, err := h.store.listItems(metric.Id)
	if err != nil {
		log.Error().Err(err).Msg("listMetrics")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	resp := listItemsResponse{
		Repo:   repo,
		Metric: metric,
		Items:  items,
	}

	render.JSON(w, resp, http.StatusOK)
}

func (h *udmHandler) deleteItem(w http.ResponseWriter, r *http.Request) {
	item, _ := itemFrom(r.Context())

	err := h.store.deleteItem(item.Id)
	if errors.Is(err, errorItemInUse) {
		render.BadRequest(w, err)
		return
	} else if err != nil {
		log.Warn().Err(err).Msg("deleteItem")
		render.InternalError(w, err)
		return
	}

	renderNoContent(w)
}

// ----------------------------------------------------------------------
// value

func (h *udmHandler) createValue(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	var value valueModel
	err := json.NewDecoder(r.Body).Decode(&value)
	if err != nil {
		log.Warn().Err(err).Msg("invalid value request body")
		render.BadRequest(w, errors.New("invalid request body"))
		return
	}

	item, _ := itemFrom(r.Context())
	if value.ItemId != item.Id {
		render.BadRequest(w, errors.New("item id mismatch"))
		return
	}

	err = h.store.addValue(&value)
	if err != nil {
		log.Error().Err(err).Msg("addValue")
		render.InternalError(w, err)
		return
	}

	render.JSON(w, value, http.StatusOK)
}

func (h *udmHandler) listValues(w http.ResponseWriter, r *http.Request) {
	repo, _ := core.RepoFrom(r.Context())
	item, _ := itemFrom(r.Context())

	values, err := h.store.listValues(item.Id)
	if err != nil {
		log.Error().Err(err).Msg("listValues")
		render.InternalError(w, err)
		return
	}

	resp := listValuesResponse{
		Repo:   repo,
		Item:   item,
		Values: values,
	}

	render.JSON(w, resp, http.StatusOK)
}

func (h *udmHandler) deleteValues(w http.ResponseWriter, r *http.Request) {
	item, _ := itemFrom(r.Context())
	err := h.store.deleteValues(item.Id)
	if err != nil {
		log.Error().Err(err).Msg("deleteValues")
		render.InternalError(w, err)
		return
	}

	renderNoContent(w)
}

func (h *udmHandler) injectMetric(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "metricId"), 10, 64)
		if err != nil {
			log.Warn().Err(err).Msg("udm.handler.injectMetric")
			render.BadRequest(w, errors.New("invalid metric id"))
			return
		}

		metric, err := h.store.findMetricById(id)
		if err == errorMetricNotFound {
			render.NotFound(w, errors.New("metric not found"))
			return
		} else if err != nil {
			log.Warn().Err(err).Msg("udm.handler.injectMetric")
			render.InternalError(w, err)
			return
		}

		r = r.WithContext(withMetric(r.Context(), *metric))
		next.ServeHTTP(w, r)
	})
}

func (h *udmHandler) injectItem(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		itemId, err := strconv.ParseInt(chi.URLParam(r, "itemId"), 10, 64)
		if err != nil {
			log.Warn().Err(err).Msg("invalid itemId in URL")
			render.BadRequest(w, errors.New("invalid item id"))
			return
		}

		item, err := h.store.findItemById(itemId)
		if err != nil {
			log.Warn().Err(err).Msg("udm.handler.injectItem")
			render.NotFound(w, errors.New("item not found"))
			return
		}

		r = r.WithContext(withItem(r.Context(), *item))
		next.ServeHTTP(w, r)
	})
}

func assertRepo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := core.RepoFrom(r.Context())
		if !ok {
			log.Error().Msg("no repository in a context")
			render.NotFound(w, render.ErrNotFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newHandler(store *udmStore) http.Handler {
	h := &udmHandler{store: store}
	r := chi.NewRouter()

	r.Route("/metrics", func(r chi.Router) {
		r.Use(assertRepo)

		r.Get("/", h.listMetrics)
		r.Post("/", h.createMetric)

		r.Route("/{metricId}", func(r chi.Router) {
			r.Use(h.injectMetric)
			r.Delete("/", h.deleteMetric)

			r.Route("/items", func(r chi.Router) {
				r.Get("/", h.listItems)
				r.Post("/", h.createItem)

				r.Route("/{itemId}", func(r chi.Router) {
					r.Use(h.injectItem)
					r.Delete("/", h.deleteItem)

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
