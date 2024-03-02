package udm

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/iszk1215/mora/mora/base"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.Disabled)
	}

	os.Exit(m.Run())
}

func newTestHandler(t *testing.T) http.Handler {
	return newHandler(initTestStore(t))
}

func unmarshalReponse(t *testing.T, r *http.Response, data any) {
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)

	err = json.Unmarshal(body, data)
	require.NoError(t, err)
}

func newRequestWithJSON(t *testing.T, method, path string, data any) *http.Request {
	body, err := json.Marshal(data)
	require.NoError(t, err)

	return httptest.NewRequest(method, path, bytes.NewBuffer(body))
}

func getReponseWithRepo(t *testing.T, expectedStatus int, h http.Handler, r *http.Request, repo base.Repository) *http.Response {
	r = r.WithContext(base.WithRepo(r.Context(), repo))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	res := w.Result()

	if expectedStatus != res.StatusCode {
		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		t.Log(string(body))
	}

	require.Equal(t, expectedStatus, res.StatusCode)

	return res
}

// ----------------------------------------------------------------------
// Metric

func TestHandlerCreateMetric(t *testing.T) {
	repo := base.Repository{
		Id: 1215,
	}

	newRequest := func(metric *metricModel) *http.Request {
		return newRequestWithJSON(t, http.MethodPost, "/metrics", metric)
	}

	t.Run("valid repo id", func(t *testing.T) {
		metric := &metricModel{
			RepoId: repo.Id,
			Name:   "metric1",
		}

		r := newRequest(metric)
		h := newTestHandler(t)
		res := getReponseWithRepo(t, http.StatusCreated, h, r, repo)

		var got metricModel
		unmarshalReponse(t, res, &got)
		require.Equal(t, int64(1), got.Id)
	})

	t.Run("repo id zero", func(t *testing.T) {
		metric := &metricModel{
			RepoId: 0,
			Name:   "metric1",
		}

		r := newRequest(metric)
		h := newTestHandler(t)
		res := getReponseWithRepo(t, http.StatusCreated, h, r, repo)

		var got metricModel
		unmarshalReponse(t, res, &got)
		require.Equal(t, int64(1), got.Id)
		require.Equal(t, int64(1215), got.RepoId)
	})

	t.Run("invalid repo id", func(t *testing.T) {
		metric := &metricModel{
			RepoId: 1976,
			Name:   "metric1",
		}

		r := newRequest(metric)
		h := newTestHandler(t)
		getReponseWithRepo(t, http.StatusBadRequest, h, r, repo)
	})
}

func TestHandlerDeleteMetric(t *testing.T) {
	repo := base.Repository{
		Id: 1215,
	}

	metric := &metricModel{
		RepoId: repo.Id,
		Name:   "metric1",
	}

	store := initTestStore(t)
	err := store.addMetric(metric)
	require.NoError(t, err)

	h := newHandler(store)

	t.Run("delete existing metric", func(t *testing.T) {
		path := fmt.Sprintf("/metrics/%d", metric.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		getReponseWithRepo(t, http.StatusNoContent, h, r, repo)

	})

	t.Run("delete metric with invalid URL", func(t *testing.T) {
		path := "/metrics/foo"
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		getReponseWithRepo(t, http.StatusBadRequest, h, r, repo)
	})

	t.Run("delete non existing metric", func(t *testing.T) {
		path := fmt.Sprintf("/metrics/%d", metric.Id) // delete again
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		getReponseWithRepo(t, http.StatusBadRequest, h, r, repo)
	})
}

func TestHandlerListMetric(t *testing.T) {
	repo := base.Repository{
		Id: 1215,
	}

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	t.Run("empty", func(t *testing.T) {
		h := newTestHandler(t)
		res := getReponseWithRepo(t, http.StatusOK, h, request, repo)

		var got listMetricsResponse
		unmarshalReponse(t, res, &got)

		require.Empty(t, got.Metrics)
	})

	t.Run("valid", func(t *testing.T) {
		metric := metricModel{
			RepoId: repo.Id,
			Name:   "metric1",
		}

		store := initTestStore(t)
		err := store.addMetric(&metric)
		require.NoError(t, err)

		h := newHandler(store)
		res := getReponseWithRepo(t, http.StatusOK, h, request, repo)

		var got listMetricsResponse
		unmarshalReponse(t, res, &got)

		require.Equal(t, []metricModel{metric}, got.Metrics)
	})
}

// ----------------------------------------------------------------------
// Item

func TestHandlerCreateItem(t *testing.T) {
	repo := base.Repository{
		Id: 1215,
	}

	metrics := []metricModel{
		{
			RepoId: repo.Id,
			Name:   "metric1",
		},
		{
			RepoId: repo.Id,
			Name:   "metric2",
		},
	}

	store := initTestStore(t)
	for i := range metrics {
		err := store.addMetric(&metrics[i])
		require.NoError(t, err)
	}

	newRequest := func(metricId int64, item itemModel) *http.Request {
		body, err := json.Marshal(item)
		require.NoError(t, err)

		url := fmt.Sprintf("/metrics/%d/items", metricId)
		return httptest.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	}

	t.Run("success", func(t *testing.T) {
		item := itemModel{
			MetricId:  metrics[0].Id,
			Name:      "item1",
			ValueType: 1, // FIXME
		}

		r := newRequest(metrics[0].Id, item)
		h := newHandler(store)
		res := getReponseWithRepo(t, http.StatusCreated, h, r, repo)

		var got metricModel
		unmarshalReponse(t, res, &got)
		require.Equal(t, int64(1), got.Id)
	})

	t.Run("invalid metric", func(t *testing.T) {
		item := itemModel{
			MetricId:  1976, // Invalid
			Name:      "item1",
			ValueType: 1, // FIXME
		}

		r := newRequest(metrics[0].Id, item)
		h := newHandler(store)
		getReponseWithRepo(t, http.StatusBadRequest, h, r, repo)
	})

	t.Run("metric id zero", func(t *testing.T) {
		item := itemModel{
			MetricId:  0,
			Name:      "item1",
			ValueType: 1, // FIXME
		}

		r := newRequest(metrics[1].Id, item)
		h := newHandler(store)
		res := getReponseWithRepo(t, http.StatusCreated, h, r, repo)

		var got itemModel
		unmarshalReponse(t, res, &got)
		require.Equal(t, int64(2), got.Id)
		require.Equal(t, metrics[1].Id, got.MetricId)
	})

	t.Run("metric id mismatch", func(t *testing.T) {
		item := itemModel{
			MetricId:  metrics[0].Id,
			Name:      "item1",
			ValueType: 1, // FIXME
		}

		r := newRequest(metrics[1].Id, item)
		h := newHandler(store)
		getReponseWithRepo(t, http.StatusBadRequest, h, r, repo)
	})
}

func TestHandlerListItems(t *testing.T) {
	repo := base.Repository{
		Id: 1215,
	}

	metric := metricModel{
		RepoId: repo.Id,
		Name:   "metric1",
	}

	store := initTestStore(t)
	err := store.addMetric(&metric)
	require.NoError(t, err)

	items := []itemModel{
		{
			MetricId:  metric.Id,
			Name:      "item1",
			ValueType: 1, // FIXME
		},
		{
			MetricId:  metric.Id,
			Name:      "metric2",
			ValueType: 1, // FIXME
		},
	}

	for i := range items {
		m := &items[i]
		err = store.addItem(m)
		require.NoError(t, err)
	}

	path := fmt.Sprintf("/metrics/%d/items", metric.Id)
	r := httptest.NewRequest(http.MethodGet, path, nil)
	h := newHandler(store)
	res := getReponseWithRepo(t, http.StatusOK, h, r, repo)

	var got listItemsResponse
	unmarshalReponse(t, res, &got)

	require.Equal(t, items, got.Items)
	require.Equal(t, metric, got.Metric)
}

func TestHandlerListItemsInvalidMetric(t *testing.T) {
	repo := base.Repository{
		Id: 1215,
	}

	store := initTestStore(t)

	path := fmt.Sprintf("/metrics/%d/items", 1976)
	r := httptest.NewRequest(http.MethodGet, path, nil)
	h := newHandler(store)
	getReponseWithRepo(t, http.StatusBadRequest, h, r, repo)
}

func TestHandlerListItemsInvalidMetricString(t *testing.T) {
	repo := base.Repository{
		Id: 1215,
	}

	store := initTestStore(t)

	path := "/metrics/foo/items"
	r := httptest.NewRequest(http.MethodGet, path, nil)
	h := newHandler(store)
	getReponseWithRepo(t, http.StatusBadRequest, h, r, repo)
}

func TestDeleteItem(t *testing.T) {
	repo := base.Repository{
		Id: 1215,
	}

	metric := metricModel{
		RepoId: repo.Id,
		Name:   "metric1",
	}

	store := initTestStore(t)
	err := store.addMetric(&metric)
	require.NoError(t, err)

	item := itemModel{
		MetricId:  metric.Id,
		Name:      "item1",
		ValueType: 1, // FIXME
	}

	err = store.addItem(&item)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		path := fmt.Sprintf("/metrics/%d/items/%d", metric.Id, item.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		h := newHandler(store)
		getReponseWithRepo(t, http.StatusNoContent, h, r, repo)
	})

	t.Run("invalid", func(t *testing.T) {
		path := fmt.Sprintf("/metrics/%d/items/%d", metric.Id, item.Id+1)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		h := newHandler(store)
		getReponseWithRepo(t, http.StatusBadRequest, h, r, repo)
	})

	err = store.addItem(&item)
	require.NoError(t, err)

	value := valueModel{
		ItemId:    item.Id,
		Revision:  "rev1",
		Timestamp: time.Now().Round(0),
		Value:     "10",
	}

	err = store.addValue(&value)
	require.NoError(t, err)

	t.Run("item in use", func(t *testing.T) {
		path := fmt.Sprintf("/metrics/%d/items/%d", metric.Id, item.Id)
		r := httptest.NewRequest(http.MethodDelete, path, nil)
		h := newHandler(store)
		getReponseWithRepo(t, http.StatusBadRequest, h, r, repo)
	})
}

// ----------------------------------------------------------------------
// Value

func setupTestItem(t *testing.T, store *udmStore) (base.Repository, metricModel, itemModel) {
	repo := base.Repository{
		Id: 1215,
	}

	metric := metricModel{
		RepoId: repo.Id,
		Name:   "metric1",
	}

	err := store.addMetric(&metric)
	require.NoError(t, err)

	item := itemModel{
		MetricId:  metric.Id,
		Name:      "item1",
		ValueType: 1, // FIXME
	}

	err = store.addItem(&item)
	require.NoError(t, err)

	return repo, metric, item
}

func TestHandlerCreateValue(t *testing.T) {
	store := initTestStore(t)
	repo, metric, item := setupTestItem(t, store)

	value := valueModel{
		ItemId:    metric.Id,
		Revision:  "rev1",
		Timestamp: time.Now().Round(0),
		Value:     "10",
	}

	body, err := json.Marshal(value)
	require.NoError(t, err)

	path := fmt.Sprintf("/metrics/%d/items/%d/values", metric.Id, item.Id)
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewBuffer(body))
	h := newHandler(store)
	res := getReponseWithRepo(t, http.StatusOK, h, r, repo)

	var got valueModel
	unmarshalReponse(t, res, &got)

	require.Equal(t, int64(1), got.Id)
}

func setupTestValues(t *testing.T, store *udmStore) (base.Repository, metricModel, itemModel, []valueModel) {
	repo := base.Repository{
		Id: 1215,
	}

	metric := metricModel{
		RepoId: repo.Id,
		Name:   "metric1",
	}

	err := store.addMetric(&metric)
	require.NoError(t, err)

	item := itemModel{
		MetricId:  metric.Id,
		Name:      "item1",
		ValueType: 1, // FIXME
	}

	err = store.addItem(&item)
	require.NoError(t, err)

	now := time.Now().Round(0)

	values := []valueModel{
		{
			ItemId:    item.Id,
			Revision:  "rev1",
			Timestamp: now.Add(time.Hour * 24),
			Value:     "10",
		},
	}

	for i := range values {
		value := &values[i]
		err := store.addValue(value)
		require.NoError(t, err)
	}

	return repo, metric, item, values
}

func TestHandlerListValues(t *testing.T) {
	store := initTestStore(t)
	repo, metric, item, values := setupTestValues(t, store)

	path := fmt.Sprintf("/metrics/%d/items/%d/values", metric.Id, item.Id)
	r := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
	h := newHandler(store)
	res := getReponseWithRepo(t, http.StatusOK, h, r, repo)

	var got listValuesResponse
	unmarshalReponse(t, res, &got)

	require.Equal(t, values, got.Values)
}

func TestHandlerDeleteValues(t *testing.T) {
	repo := base.Repository{Id: 1}
	metric := metricModel{Name: "metric1"}
	item := itemModel{Name: "item1"}

	initialzer := storeInitializer{
		repoId: repo.Id,
		metrics: []*metricInitializer{
			{
				metric: &metric,
				items: []*itemInitializer{
					{
						item: &item,
						values: []*valueModel{
							{Timestamp: time.Now().Round(0), Value: "10"},
							{Timestamp: time.Now().Round(0), Value: "11"},
						},
					},
				},
			},
		},
	}

	store := initTestStore(t)
	initMockStore(t, store, &initialzer)

	path := fmt.Sprintf("/metrics/%d/items/%d/values", metric.Id, item.Id)
	r := httptest.NewRequest(http.MethodDelete, path, strings.NewReader(""))
	h := newHandler(store)
	getReponseWithRepo(t, http.StatusNoContent, h, r, repo)
}
