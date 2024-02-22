package udm

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

type (
	itemInitializer struct {
		item   *itemModel
		values []*valueModel
	}

	metricInitializer struct {
		metric *metricModel
		items  []*itemInitializer
	}

	storeInitializer struct {
		repoId  int64
		metrics []*metricInitializer
	}
)

func initMockStore(t *testing.T, s *udmStore, init *storeInitializer) {
	for _, m := range init.metrics {
		m.metric.RepoId = init.repoId
		err := s.addMetric(m.metric)
		require.NoError(t, err)

		for _, itemInit := range m.items {
			itemInit.item.MetricId = m.metric.Id
			err := s.addItem(itemInit.item)
			require.NoError(t, err)

			for _, value := range itemInit.values {
				value.ItemId = itemInit.item.Id
				err := s.addValue(value)
				require.NoError(t, err)
			}
		}
	}
}

func initTestStore(t *testing.T) *udmStore {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	s := newUdmStore(db)

	err = s.initialize()
	require.NoError(t, err)

	return s
}

func TestUdmStoreNew(t *testing.T) {
	initTestStore(t)
}

func TestStoreMetric(t *testing.T) {
	s := initTestStore(t)

	t.Run("store metric", func(t *testing.T) {
		metric := &metricModel{
			RepoId: 1215,
			Name:   "test_metric",
		}

		err := s.addMetric(metric)
		require.NoError(t, err)
		require.Equal(t, int64(1), metric.Id)
	})

	t.Run("store metric with negative repoId", func(t *testing.T) {
		metric := &metricModel{
			RepoId: -1,
			Name:   "test_metric",
		}

		err := s.addMetric(metric)
		require.Error(t, err)
	})
}

func TestStoreFindMetric(t *testing.T) {
	metric := &metricModel{
		RepoId: 1215,
		Name:   "test_metric",
	}

	s := initTestStore(t)
	err := s.addMetric(metric)
	require.NoError(t, err)
	require.Equal(t, int64(1), metric.Id)

	t.Run("find by existing id", func(t *testing.T) {
		got, err := s.findMetricById(metric.Id)
		require.NoError(t, err)
		require.Equal(t, metric, got)
	})

	t.Run("find by non existing id", func(t *testing.T) {
		_, err := s.findMetricById( /* id= */ 1976)
		require.ErrorIs(t, errorMetricNotFound, err)
	})

	t.Run("list by existing repo id", func(t *testing.T) {
		metrics, err := s.listMetrics(metric.RepoId)
		require.NoError(t, err)
		require.Equal(t, []metricModel{*metric}, metrics)
	})

	t.Run("list by non existing repo id", func(t *testing.T) {
		metrics, err := s.listMetrics( /* repo_id= */ 1976)
		require.NoError(t, err)
		require.Empty(t, metrics)
	})
}

func TestStoreDeleteMetric(t *testing.T) {
	metrics := []*metricModel{
		{
			RepoId: 1215,
			Name:   "metric0",
		},
		{
			RepoId: 1215,
			Name:   "metric1",
		},
	}

	s := initTestStore(t)
	for _, metric := range metrics {
		err := s.addMetric(metric)
		require.NoError(t, err)
	}

	item := &itemModel{
		MetricId: metrics[1].Id,
		Name:     "test_item",
	}

	err := s.addItem(item)
	require.NoError(t, err)

	t.Run("delete existing metric", func(t *testing.T) {
		err := s.deleteMetric(metrics[0].Id)
		require.NoError(t, err)
	})

	t.Run("delete non existing metric", func(t *testing.T) {
		err := s.deleteMetric(1976)
		require.NoError(t, err)
	})

	t.Run("delete metric with items", func(t *testing.T) {
		err := s.deleteMetric(metrics[1].Id)
		require.Error(t, errorMetricInUse, err)
	})
}

func TestStoreAddItem(t *testing.T) {
	metric := &metricModel{
		RepoId: 1215,
		Name:   "test_metric",
	}

	s := initTestStore(t)
	err := s.addMetric(metric)
	require.NoError(t, err)

	t.Run("add item with existing metric id", func(t *testing.T) {
		item := &itemModel{
			MetricId: metric.Id,
			Name:     "test_item",
		}

		err = s.addItem(item)
		require.NoError(t, err)
		require.Equal(t, int64(1), item.Id)
	})

	t.Run("add item with non existing metric id", func(t *testing.T) {
		item := &itemModel{
			MetricId: metric.Id + 1,
			Name:     "test_item",
		}

		err = s.addItem(item)
		require.ErrorIs(t, errorMetricNotFound, err)
	})
}

func TestStoreFindItem(t *testing.T) {
	metric := &metricModel{
		RepoId: 1215,
		Name:   "test_metric",
	}

	s := initTestStore(t)
	err := s.addMetric(metric)
	require.NoError(t, err)

	item := &itemModel{
		MetricId: metric.Id,
		Name:     "test_item",
	}

	err = s.addItem(item)
	require.NoError(t, err)
	require.Equal(t, int64(1), item.Id)

	t.Run("find existing item by id", func(t *testing.T) {
		got, err := s.findItemById(item.Id)
		require.NoError(t, err)
		require.Equal(t, item, got)
	})

	t.Run("find non existing item by id", func(t *testing.T) {
		// invalid id
		_, err = s.findItemById( /* id=*/ 1215)
		require.ErrorIs(t, errorItemNotFound, err)
	})

	t.Run("list items by existing metric id", func(t *testing.T) {
		items, err := s.listItems(item.MetricId)
		require.NoError(t, err)
		require.Equal(t, []itemModel{*item}, items)

	})

	t.Run("list items by non existing metric id", func(t *testing.T) {
		items, err := s.listItems( /* metric_id=*/ 1215)
		require.NoError(t, err)
		require.Empty(t, items)
	})
}

func TestStoreDeleteItem(t *testing.T) {
	metric := &metricModel{
		RepoId: 1215,
		Name:   "test_metric",
	}

	s := initTestStore(t)
	err := s.addMetric(metric)
	require.NoError(t, err)

	items := []*itemModel{
		{
			MetricId: metric.Id,
			Name:     "item1",
		},
		{
			MetricId: metric.Id,
			Name:     "item2",
		},
	}

	for _, item := range items {
		err = s.addItem(item)
		require.NoError(t, err)
		require.Greater(t, item.Id, int64(0))
	}

	// items[0] has a value
	value := &valueModel{
		ItemId:    items[0].Id,
		Revision:  "reveision",
		Timestamp: time.Now().Round(0),
		Value:     "1976",
	}

	err = s.addValue(value)
	require.NoError(t, err)
	require.Equal(t, int64(1), value.Id)

	t.Run("delete existing item without value", func(t *testing.T) {
		err := s.deleteItem(items[1].Id)
		require.NoError(t, err)
	})

	t.Run("delete existing item with value", func(t *testing.T) {
		err := s.deleteItem(items[0].Id)
		require.ErrorIs(t, errorItemInUse, err)
	})

	t.Run("delete non existing item", func(t *testing.T) {
		err := s.deleteItem(items[1].Id + 1)
		require.NoError(t, err)
	})
}

func TestStoreValue(t *testing.T) {
	metric := &metricModel{
		RepoId: 1215,
		Name:   "test_metric",
	}

	s := initTestStore(t)
	err := s.addMetric(metric)
	require.NoError(t, err)

	item := &itemModel{
		MetricId: metric.Id,
		Name:     "test_item",
	}

	err = s.addItem(item)
	require.NoError(t, err)

	value := &valueModel{
		ItemId:    item.Id,
		Revision:  "reveision",
		Timestamp: time.Now().Round(0),
		Value:     "1976",
	}

	err = s.addValue(value)
	require.NoError(t, err)
	require.Equal(t, int64(1), value.Id)

	// non existing id
	values, err := s.listValues( /*metrid_id=*/ 130)
	require.NoError(t, err)
	require.Empty(t, values)

	// existing id
	values, err = s.listValues(item.Id)
	require.NoError(t, err)
	require.Equal(t, []valueModel{*value}, values)
}

func TestDeleteValues(t *testing.T) {
	metric := metricModel{Name: "metric1"}
	item := itemModel{Name: "item1"}

	initialzer := storeInitializer{
		repoId: 1,
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

	s := initTestStore(t)
	initMockStore(t, s, &initialzer)

	values, err := s.listValues(item.Id)
	require.NoError(t, err)
	require.Equal(t, 2, len(values))

	err = s.deleteValues(item.Id)
	require.NoError(t, err)

	values, err = s.listValues(item.Id)
	require.NoError(t, err)
	require.Empty(t, values)
}
