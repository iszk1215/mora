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

	MetricModel struct {
		Id     int64  `json:"id"      db:"id"`
		RepoId int64  `json:"repo_id" db:"repo_id"`
		Name   string `json:"name"    db:"name"`
	}

	ItemModel struct {
		Id        int64    `json:"id"        db:"id"`
		MetricId  int64    `json:"metric_id" db:"metric_id"`
		Name      string   `json:"name"      db:"name"`
		ValueType ValueType `json:"type"      db:"type"`
	}

	ValueModel struct {
		Id        int64     `db:"id"`
		ItemId    int64     `db:"item_id"`
		Timestamp time.Time `json:"time"     db:"time"`
		Value     string    `json:"value"    db:"value"`
	}

	ListMetricsResponse struct {
		Repo    core.Repository `json:"repo"`
		Metrics []MetricModel   `json:"metrics"`
	}

	ListItemsResponse struct {
		Repo   core.Repository `json:"repo"`
		Metric MetricModel     `json:"metric"`
		Items  []ItemModel     `json:"items"`
	}

	ListValuesResponse struct {
		Repo   core.Repository `json:"repo"`
		Item   ItemModel       `json:"items"`
		Values []ValueModel    `json:"values"`
	}

	ContextKey int
)

const (
	metricContextKey ContextKey = iota
	itemContextKey
)

func withMetric(ctx context.Context, metric MetricModel) context.Context {
	return context.WithValue(ctx, metricContextKey, metric)
}

func metricFrom(ctx context.Context) (MetricModel, bool) {
	m, ok := ctx.Value(metricContextKey).(MetricModel)
	return m, ok
}

func withItem(ctx context.Context, item ItemModel) context.Context {
	return context.WithValue(ctx, itemContextKey, item)
}

func itemFrom(ctx context.Context) (ItemModel, bool) {
	item, ok := ctx.Value(itemContextKey).(ItemModel)
	return item, ok
}

func renderNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// ----------------------------------------------------------------------
// Metric

// createMetric godoc
// @Summary      Create a metric
// @Description  Create a new UDM metric for the repository
// @Tags         udm
// @Accept       json
// @Produce      json
// @Param        repo_id  path  int  true  "Repository ID"
// @Param        body     body  udm.MetricModel  true  "Metric information"
// @Success      201  {object}  udm.MetricModel
// @Failure      400  {object}  core.ErrorResponse
// @Router       /api/repos/{repo_id}/udm/metrics [post]
func (h *udmHandler) createMetric(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	var metric MetricModel
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

// listMetrics godoc
// @Summary      List metrics
// @Description  Return all UDM metrics for the repository
// @Tags         udm
// @Produce      json
// @Param        repo_id  path  int  true  "Repository ID"
// @Success      200  {object}  udm.ListMetricsResponse
// @Router       /api/repos/{repo_id}/udm/metrics [get]
func (h *udmHandler) listMetrics(w http.ResponseWriter, r *http.Request) {
	repo, _ := core.RepoFrom(r.Context())
	metrics, err := h.store.listMetrics(repo.Id)
	if err != nil {
		log.Error().Err(err).Msg("udm.handler.listMetrics")
		render.InternalError(w, err)
		return
	}

	resp := ListMetricsResponse{
		Repo:    repo,
		Metrics: metrics,
	}

	render.JSON(w, resp, http.StatusOK)
}

// deleteMetric godoc
// @Summary      Delete a metric
// @Description  Delete a UDM metric and its items
// @Tags         udm
// @Param        repo_id   path  int  true  "Repository ID"
// @Param        metricId  path  int  true  "Metric ID"
// @Success      204
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/repos/{repo_id}/udm/metrics/{metricId} [delete]
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

// createItem godoc
// @Summary      Create an item
// @Description  Create a new UDM item for a metric
// @Tags         udm
// @Accept       json
// @Produce      json
// @Param        repo_id   path  int  true  "Repository ID"
// @Param        metricId  path  int  true  "Metric ID"
// @Param        body      body  udm.ItemModel  true  "Item information"
// @Success      201  {object}  udm.ItemModel
// @Failure      400  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/repos/{repo_id}/udm/metrics/{metricId}/items [post]
func (h *udmHandler) createItem(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	var item ItemModel
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

// listItems godoc
// @Summary      List items
// @Description  Return all UDM items for a metric
// @Tags         udm
// @Produce      json
// @Param        repo_id   path  int  true  "Repository ID"
// @Param        metricId  path  int  true  "Metric ID"
// @Success      200  {object}  udm.ListItemsResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/repos/{repo_id}/udm/metrics/{metricId}/items [get]
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

	resp := ListItemsResponse{
		Repo:   repo,
		Metric: metric,
		Items:  items,
	}

	render.JSON(w, resp, http.StatusOK)
}

// deleteItem godoc
// @Summary      Delete an item
// @Description  Delete a UDM item and its values
// @Tags         udm
// @Param        repo_id  path  int  true  "Repository ID"
// @Param        metricId  path  int  true  "Metric ID"
// @Param        itemId    path  int  true  "Item ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/repos/{repo_id}/udm/metrics/{metricId}/items/{itemId} [delete]
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

// createValue godoc
// @Summary      Add a value
// @Description  Add a value to a UDM item
// @Tags         udm
// @Accept       json
// @Produce      json
// @Param        repo_id  path  int  true  "Repository ID"
// @Param        metricId  path  int  true  "Metric ID"
// @Param        itemId    path  int  true  "Item ID"
// @Param        body      body  udm.ValueModel  true  "Value information"
// @Success      200  {object}  udm.ValueModel
// @Failure      400  {object}  core.ErrorResponse
// @Router       /api/repos/{repo_id}/udm/metrics/{metricId}/items/{itemId}/values [post]
func (h *udmHandler) createValue(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Body.Close")
		}
	}()

	var value ValueModel
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

// listValues godoc
// @Summary      List values
// @Description  Return all values for a UDM item
// @Tags         udm
// @Produce      json
// @Param        repo_id  path  int  true  "Repository ID"
// @Param        metricId  path  int  true  "Metric ID"
// @Param        itemId    path  int  true  "Item ID"
// @Success      200  {object}  udm.ListValuesResponse
// @Router       /api/repos/{repo_id}/udm/metrics/{metricId}/items/{itemId}/values [get]
func (h *udmHandler) listValues(w http.ResponseWriter, r *http.Request) {
	repo, _ := core.RepoFrom(r.Context())
	item, _ := itemFrom(r.Context())

	values, err := h.store.listValues(item.Id)
	if err != nil {
		log.Error().Err(err).Msg("listValues")
		render.InternalError(w, err)
		return
	}

	resp := ListValuesResponse{
		Repo:   repo,
		Item:   item,
		Values: values,
	}

	render.JSON(w, resp, http.StatusOK)
}

// deleteValues godoc
// @Summary      Delete all values
// @Description  Delete all values for a UDM item
// @Tags         udm
// @Param        repo_id  path  int  true  "Repository ID"
// @Param        metricId  path  int  true  "Metric ID"
// @Param        itemId    path  int  true  "Item ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Router       /api/repos/{repo_id}/udm/metrics/{metricId}/items/{itemId}/values [delete]
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
