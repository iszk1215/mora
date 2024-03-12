package udm

import (
	"errors"
	"testing"
	"time"

	"github.com/iszk1215/mora/mora/core"
	"github.com/stretchr/testify/require"
)

type mockUdmClient struct {
	t *testing.T

	initialized bool
	repoId      int64
	metrics     []metricModel
	items       []itemModel
	values      []valueModel
}

func (c *mockUdmClient) init(serverAddr, token string) {
	c.initialized = true
}

func (c *mockUdmClient) listRepositories() ([]core.Repository, error) {
	// c.t.Log("mockUdmClient.listRepositories")
	return []core.Repository{{Id: 1}}, nil
}

func (c *mockUdmClient) addMetric(repoId int64, metric *metricModel) error {
	c.t.Logf("mockUdmClient.addMetric: repoId=%d metric.Id=%d metric.Name=%s",
		repoId, metric.Id, metric.Name)

	if repoId != c.repoId || metric.Id != 0 {
		return errors.New("unexpected metric")
	}

	metric.Id = int64(len(c.metrics) + 1)
	c.metrics = append(c.metrics, *metric)

	return nil
}

func (c *mockUdmClient) listMetrics(repoId int64) ([]metricModel, error) {
	// c.t.Log("mockUdmClient.listMetrics")
	return c.metrics, nil
}

func (c *mockUdmClient) listItems(repoId, itemId int64) ([]itemModel, error) {
	// c.t.Log("mockUdmClient.listItems")
	return c.items, nil
}

func (c *mockUdmClient) addItem(repoId int64, item *itemModel) error {
	c.t.Logf("mockUdmClient.addItem: repoId=%d item.MetricId=%d item.Name=%s item.ValueType=%d",
		repoId, item.MetricId, item.Name, item.ValueType)

	validMetric := false
	for _, m := range c.metrics {
		c.t.Logf("mockUdmClient.addItem: m.Id=%d", m.Id)
		if m.Id == item.MetricId {
			validMetric = true
		}
	}

	if repoId != c.repoId || !validMetric {
		return errors.New("unexpected metric")
	}

	c.items = append(c.items, *item)

	return nil
}

func (c *mockUdmClient) deleteItem(repoId, metricId, itemId int64) error {
	c.t.Logf("mockUdmClient.deleteItem: repoId=%d metricId=%d itemId=%d", itemId, metricId, itemId)

	valid := false
	for _, m := range c.items {
		if m.Id == itemId && m.MetricId == metricId {
			valid = true
		}
	}

	if repoId != c.repoId || !valid {
		return errors.New("deleteMetric failed")
	}

	return nil
}

func (c *mockUdmClient) addValue(repoId, metricId int64, value *valueModel) error {
	value.Id = 456 // FIXME
	c.values = append(c.values, *value)
	return nil
}

func (c *mockUdmClient) listValues(repoId, metricId, itemId int64) ([]valueModel, error) {
	return c.values, nil
}

func (c *mockUdmClient) deleteValues(repoId, metricId, itemId int64) error {
	c.values = []valueModel{}
	return nil
}

// tests

func TestCmdCreateMetricAndItem(t *testing.T) {
	mock := &mockUdmClient{t: t, repoId: 1}

	cli := udmCommand{
		client: mock,
	}

	cmd := cli.newCommand()
	cmd.SetArgs([]string{"metric", "-c", "foo/bar"})
	err := cmd.Execute()
	require.NoError(t, err)

	require.Equal(t, 1, len(mock.metrics))
	require.Equal(t, "foo", mock.metrics[0].Name)
	require.Equal(t, 1, len(mock.items))
	require.Equal(t, "bar", mock.items[0].Name)
}

func TestCmdCreateItem(t *testing.T) {
	cli := udmCommand{
		client: &mockUdmClient{t: t, repoId: 1},
	}

	cmd := cli.newCommand()
	cmd.SetArgs([]string{"metric", "-c", "dummyMetric/dummyItem", "--token", "1"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestCmdDeleteMetric(t *testing.T) {
	mock := &mockUdmClient{t: t, repoId: 1}
	cli := udmCommand{client: mock}

	mock.metrics = append(mock.metrics,
		metricModel{Id: 2, RepoId: mock.repoId, Name: "dummyMetric"})
	mock.items = append(mock.items,
		itemModel{Id: 3, MetricId: 2, Name: "dummyItem", ValueType: 1})

	cmd := cli.newCommand()
	cmd.SetArgs([]string{"metric", "-d", "dummyMetric/dummyItem"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestCmdAddValue(t *testing.T) {
	mock := &mockUdmClient{t: t, repoId: 1}
	mock.metrics = append(mock.metrics,
		metricModel{Id: 2, RepoId: mock.repoId, Name: "metric"})
	mock.items = append(mock.items,
		itemModel{Id: 3, MetricId: 2, Name: "item", ValueType: 1})

	cli := &udmCommand{client: mock}
	cmd := cli.newCommand()

	t.Run("success", func(t *testing.T) {
		cmd.SetArgs([]string{"value", "--add", "metric/item", "--time", "2024-02-26", "10"})
		err := cmd.Execute()
		require.NoError(t, err)
		require.Equal(t, 1, len(mock.values))
		require.Equal(t, "10", mock.values[0].Value)
		require.Equal(t, "2024-02-26", mock.values[0].Timestamp.Format("2006-01-02"))
	})

	t.Run("no value", func(t *testing.T) {
		cmd.SetArgs([]string{"value", "--add", "metric/item"})
		err := cmd.Execute()
		require.Error(t, err)
	})

	t.Run("no timestamp", func(t *testing.T) {
		cmd.SetArgs([]string{"value", "--add", "metric/item", "10"})
		err := cmd.Execute()
		require.NoError(t, err)
		require.Equal(t, 2, len(mock.values))
	})

	t.Run("invalid timestamp", func(t *testing.T) {
		cmd.SetArgs([]string{"value", "--add", "metric/item", "--time", "2024-02-30", "10"})
		err := cmd.Execute()
		require.Error(t, err)
	})
}

func TestCmdListValues(t *testing.T) {
	mock := &mockUdmClient{t: t, repoId: 1}
	mock.metrics = append(mock.metrics,
		metricModel{Id: 2, RepoId: mock.repoId, Name: "metric"})
	mock.items = append(mock.items,
		itemModel{Id: 3, MetricId: 2, Name: "item", ValueType: 1})

	cli := udmCommand{client: mock}
	cmd := cli.newCommand()

	t.Run("success", func(t *testing.T) {
		cmd.SetArgs([]string{"value", "--list", "metric/item"})
		err := cmd.Execute()
		require.NoError(t, err)
	})

	t.Run("without metric", func(t *testing.T) {
		cmd.SetArgs([]string{"value", "--list"})
		err := cmd.Execute()
		require.Error(t, err)
	})

	t.Run("with invalid metric", func(t *testing.T) {
		cmd.SetArgs([]string{"value", "--list", "hoge"})
		err := cmd.Execute()
		require.Error(t, err)
	})

	t.Run("with non-existing metric", func(t *testing.T) {
		cmd.SetArgs([]string{"value", "--list", "foo/bar"})
		err := cmd.Execute()
		require.Error(t, err)
	})

	t.Run("with non-existing item", func(t *testing.T) {
		cmd.SetArgs([]string{"value", "--list", "metric/bar"})
		err := cmd.Execute()
		require.Error(t, err)
	})
}

func TestCmdClearValues(t *testing.T) {
	mock := &mockUdmClient{t: t, repoId: 1}
	mock.metrics = append(mock.metrics,
		metricModel{Id: 2, RepoId: mock.repoId, Name: "dummyMetric"})
	mock.items = append(mock.items,
		itemModel{Id: 3, MetricId: 2, Name: "dummyItem", ValueType: 1})
	mock.values = append(mock.values,
		valueModel{Id: 1, ItemId: 3, Timestamp: time.Now().Round(0), Value: "10"})

	cli := udmCommand{client: mock}
	cmd := cli.newCommand()

	cmd.SetArgs([]string{"value", "--clear", "dummyMetric/dummyItem"})
	err := cmd.Execute()
	require.NoError(t, err)

	require.Empty(t, mock.values)
}
