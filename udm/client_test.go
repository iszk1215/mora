package udm

import (
	"errors"
	"net/http"
	"testing"

	"github.com/iszk1215/mora/core"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

type funcDo func(method, path string, in any, out any) error
type funcListRepoistories func() ([]core.Repository, error)

type doHandler struct {
	fn funcDo
	ok bool
}

type mockAPIClient struct {
	t                       *testing.T
	ok_                     bool
	handle_Do               funcDo
	handle_ListRepositories funcListRepoistories
	nextDoHandler           int
	doHandlers              []*doHandler
}

func (c *mockAPIClient) ok() bool {
	return c.ok_
}

func (c *mockAPIClient) handleDo(fn funcDo) {
	c.handle_Do = fn
	c.ok_ = false
}

func (c *mockAPIClient) SetDoHandlers(funcs ...funcDo) {
	for _, fn := range funcs {
		c.doHandlers = append(c.doHandlers, &doHandler{fn: fn, ok: false})
	}
}

func (c *mockAPIClient) handleListRepositories(fn funcListRepoistories) {
	c.handle_ListRepositories = fn
}

func (c *mockAPIClient) Do(method, path string, in any, out any) error {
	if c.handle_Do != nil {
		c.ok_ = true
		return c.handle_Do(method, path, in, out)
	}

	if c.nextDoHandler < len(c.doHandlers) {
		err := c.doHandlers[c.nextDoHandler].fn(method, path, in, out)
		c.nextDoHandler++
		return err
	}

	require.FailNowf(c.t, "no Do handler", "method=%s", method)
	return nil
}

func (c *mockAPIClient) ListRepositories() ([]core.Repository, error) {
	if c.handle_ListRepositories != nil {
		return c.handle_ListRepositories()
	}
	require.FailNow(c.t, "no ListRepositories handler")
	c.t.FailNow()
	return []core.Repository{}, nil
}

func newMockAPIClient(t *testing.T) *mockAPIClient {
	return &mockAPIClient{t: t, ok_: true}
}

func TestUdmClientListRepositories2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := NewMockAPIClient(ctrl)
	c := &udmClient{client: m}

	t.Run("ok", func(t *testing.T) {
		m.EXPECT().ListRepositories().Return([]core.Repository{}, nil)

		repos, err := c.listRepositories()
		require.NoError(t, err)
		require.Empty(t, repos)
	})

	t.Run("error", func(t *testing.T) {
		m.EXPECT().ListRepositories().Return(nil, errors.New("error"))

		_, err := c.listRepositories()
		require.Error(t, err)
	})
}

func TestUdmClientListRepositories(t *testing.T) {
	api := newMockAPIClient(t)
	c := &udmClient{client: api}

	t.Run("ok", func(t *testing.T) {
		api.handleListRepositories(func() ([]core.Repository, error) {
			return []core.Repository{}, nil
		})
		repos, err := c.listRepositories()
		require.NoError(t, err)
		require.Empty(t, repos)
	})

	t.Run("error", func(t *testing.T) {
		api.handleListRepositories(func() ([]core.Repository, error) {
			return nil, errors.New("error")
		})
		_, err := c.listRepositories()
		require.Error(t, err)
	})
}

func TestUdmClientAddMetric(t *testing.T) {
	api := newMockAPIClient(t)
	c := &udmClient{client: api}

	metric := MetricModel{RepoId: 1215, Name: "metric_name"}

	api.handleDo(func(method, path string, in any, out any) error {
		require.Equal(t, method, http.MethodPost)
		require.Equal(t, path, "/api/repos/1215/udm/metrics")
		require.NotNil(t, in)
		require.NotNil(t, out)
		require.IsType(t, &MetricModel{}, in)
		require.IsType(t, &MetricModel{}, out)

		got, ok := in.(*MetricModel)
		require.True(t, ok)
		require.Equal(t, &metric, got)

		return nil
	})

	err := c.addMetric(metric.RepoId, &metric)
	require.NoError(t, err)
	require.True(t, api.ok())
}

func TestUdmClientListMetrics(t *testing.T) {
	api := newMockAPIClient(t)
	c := &udmClient{client: api}

	repoId := int64(1215)

	api.handleDo(func(method, path string, in any, out any) error {
		require.Equal(t, method, http.MethodGet)
		require.Equal(t, path, "/api/repos/1215/udm/metrics")
		require.Nil(t, in)
		require.NotNil(t, out)

		resp, ok := out.(*ListMetricsResponse)
		require.True(t, ok)

		resp.Metrics = append(resp.Metrics, MetricModel{Name: "metric"})
		return nil
	})

	got, err := c.listMetrics(repoId)
	require.NoError(t, err)
	require.True(t, api.ok())
	require.Equal(t, 1, len(got))
}

func TestUdmClientAddItem(t *testing.T) {
	api := newMockAPIClient(t)
	c := &udmClient{client: api}

	item := ItemModel{MetricId: 1976, Name: "item_name", ValueType: ValueTypeInt}

	api.handleDo(func(method, path string, in any, out any) error {
		require.Equal(t, method, http.MethodPost)
		require.Equal(t, path, "/api/repos/1215/udm/metrics/1976/items")
		require.NotNil(t, in)
		require.NotNil(t, out)
		require.IsType(t, &ItemModel{}, out)

		got, ok := in.(*ItemModel)
		require.True(t, ok)
		require.Equal(t, &item, got)

		return nil
	})

	err := c.addItem(1215, &item)
	require.NoError(t, err)
	require.True(t, api.ok())
}
