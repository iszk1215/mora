package udm

import (
    "errors"
    "testing"

    "github.com/iszk1215/mora/mora/model"
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

func (c *mockUdmClient) listRepositories() ([]model.Repository, error) {
    c.t.Log("mockUdmClient.listRepositories")
    return []model.Repository{{Id: 1}}, nil
}

func (c *mockUdmClient) addMetric(repoId int64, metric *metricModel) error {
    c.t.Logf("mockUdmClient.addMetric: repoId=%d metric.Id=%d metric.Name=%s",
        repoId, metric.Id, metric.Name)

    if repoId != c.repoId || metric.Id != 0 {
        return errors.New("unexpected metric")
    }

    metric.Id = 123 // FIXME
    c.metrics = append(c.metrics, *metric)

    return nil
}

func (c *mockUdmClient) listMetrics(repoId int64) ([]metricModel, error) {
    c.t.Log("mockUdmClient.listMetrics")
    return c.metrics, nil
}

func (c *mockUdmClient) listItems(repoId, itemId int64) ([]itemModel, error) {
    c.t.Log("mockUdmClient.listItems")
    return c.items, nil
}

func (c *mockUdmClient) addItem(repoId int64, item *itemModel) error {
    c.t.Logf("mockUdmClient.addItem: repoId=%d item.MetricId=%d item.Name=%s item.ValueType=%d",
        repoId, item.MetricId, item.Name, item.ValueType)

    validMetric := false
    for _, g := range c.metrics {
        c.t.Logf("mockUdmClient.addItem: g.Id=%d", g.Id)
        if g.Id == item.MetricId {
            validMetric = true
        }
    }

    if repoId != c.repoId || !validMetric {
        return errors.New("unexpected metric")
    }

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
    value.Id = 456; // FIXME
    c.values = append(c.values, *value)
    return nil
}

// tests

func TestCmdCreateMetricAndItem(t *testing.T) {
    cli := cli{
        client: &mockUdmClient{t: t, repoId: 1},
    }

    cmd := cli.newCommand()
    cmd.SetArgs([]string{"metric", "-c", "dummyMetric2/dummyItem", "-t", "1"})
    err := cmd.Execute()
    require.NoError(t, err)
}

func TestCmdCreateItem(t *testing.T) {
    cli := cli{
        client: &mockUdmClient{t: t, repoId: 1},
    }

    cmd := cli.newCommand()
    cmd.SetArgs([]string{"metric", "-c", "dummyMetric/dummyItem", "-t", "1"})
    err := cmd.Execute()
    require.NoError(t, err)
}

func TestCmdDeleteMetric(t *testing.T) {
    mock := &mockUdmClient{t: t, repoId: 1}
    cli := cli{client: mock}

    mock.metrics = append(mock.metrics,
        metricModel{Id: 2, RepoId: mock.repoId, Name: "dummyMetric"})
    mock.items = append(mock.items,
        itemModel{Id: 3, MetricId: 2, Name: "dummyItem", ValueType: 1})

    cmd := cli.newCommand()
    cmd.SetArgs([]string{"metric", "-d", "dummyMetric/dummyItem"})
    err := cmd.Execute()
    require.NoError(t, err)
}
