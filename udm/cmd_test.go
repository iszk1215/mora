package udm

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/iszk1215/mora/core"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

var (
	isListMetricsResponse = gomock.AssignableToTypeOf(&listMetricsResponse{})
	isListItemsResponse   = gomock.AssignableToTypeOf(&listItemsResponse{})
	isListValuesResponse  = gomock.AssignableToTypeOf(&listValuesResponse{})
	isMetric              = gomock.AssignableToTypeOf(&metricModel{})
	isItem                = gomock.AssignableToTypeOf(&itemModel{})
	isValue               = gomock.AssignableToTypeOf(&valueModel{})
)

func newCommandWithMock(m core.APIClient, repoURL string) *udmCommand {
	return &udmCommand{
		client: &udmClient{client: m},
		config: core.ClientConfig{
			RepositoryURL: repoURL,
		},
	}
}

type mockHelper struct {
	mock *MockAPIClient
	repo core.Repository
	base string
}

type mockListRepositoriesCall struct {
	*gomock.Call
}

type mockListMetricsCall struct {
	*gomock.Call
	helper *mockHelper
}

func (c *mockListMetricsCall) Return(metrics ...metricModel) *gomock.Call {
	return c.Do(func(method, path string, in, out any) {
		resp, _ := out.(*listMetricsResponse)
		resp.Repo = c.helper.repo
		resp.Metrics = metrics
	})
}

func newMockHelper(ctrl *gomock.Controller, repo core.Repository) *mockHelper {
	base := fmt.Sprintf("/api/repos/%d/udm", repo.Id)
	return &mockHelper{mock: NewMockAPIClient(ctrl), repo: repo, base: base}
}

func (h *mockHelper) expectListRepositories() *gomock.Call {
	return h.mock.EXPECT().ListRepositories().Return([]core.Repository{h.repo}, nil)
}

func (h *mockHelper) expectListRepositories2() *mockListRepositoriesCall {
	call := h.mock.EXPECT().ListRepositories()
	mcall := mockListRepositoriesCall{call}
	return &mcall
}

func (h *mockHelper) expectListMetrics(metrics ...metricModel) *gomock.Call {
	return h.mock.EXPECT().
		Do(http.MethodGet, h.base+"/metrics", nil, isListMetricsResponse).
		Do(func(method, path string, in, out any) {
			resp, _ := out.(*listMetricsResponse)
			resp.Metrics = metrics
		})
}

func (h *mockHelper) expectListMetrics2() *mockListMetricsCall {
	return &mockListMetricsCall{
		h.mock.EXPECT().
			Do(http.MethodGet, h.base+"/metrics", nil, isListMetricsResponse),
		h,
	}
}

func (h *mockHelper) expectAddMetric(metric metricModel, returnId int64) *gomock.Call {
	return h.mock.EXPECT().
		Do(http.MethodPost, h.base+"/metrics", &metric, isMetric).
		Do(func(method, path string, in, out any) {
			resp, _ := out.(*metricModel)
			resp.Id = returnId
		})
}

func (h *mockHelper) expectListItems(metricId int64, items ...itemModel) *gomock.Call {
	path := fmt.Sprintf("%s/metrics/%d/items", h.base, metricId)
	return h.mock.EXPECT().
		Do(http.MethodGet, path, nil, isListItemsResponse).
		Do(func(method, path string, in, out any) {
			resp, _ := out.(*listItemsResponse)
			resp.Items = items
		})
}

func (h *mockHelper) expectAddItem(metricId int64, item itemModel, returnId int64) *gomock.Call {
	path := fmt.Sprintf("%s/metrics/%d/items", h.base, metricId)
	return h.mock.EXPECT().
		Do(http.MethodPost, path, &item, isItem).
		Do(func(method, path string, in, out any) {
			resp, _ := out.(*itemModel)
			resp.Id = returnId
		})
}

func (h *mockHelper) expectDeleteItem(metricId, itemId int64) *gomock.Call {
	path := fmt.Sprintf("%s/metrics/%d/items/%d", h.base, metricId, itemId)
	return h.mock.EXPECT().Do(http.MethodDelete, path, nil, nil)
}

func (h *mockHelper) expectAddValue(metricId, itemId int64, value valueModel) *gomock.Call {
	path := fmt.Sprintf("%s/metrics/%d/items/%d/values", h.base, metricId, itemId)
	return h.mock.EXPECT().
		Do(http.MethodPost, path, &value, isValue)
}

func (h *mockHelper) expectListValues(metricId, itemId int64, values ...valueModel) *gomock.Call {
	path := fmt.Sprintf("%s/metrics/%d/items/%d/values", h.base, metricId, itemId)
	return h.mock.EXPECT().
		Do(http.MethodGet, path, nil, isListValuesResponse).
		Do(func(method, path string, in, out any) {
			resp, _ := out.(*listValuesResponse)
			resp.Values = values
		})
}

func (h *mockHelper) expectDeleteValues(metricId, itemId int64) *gomock.Call {
	path := fmt.Sprintf("%s/metrics/%d/items/%d/values", h.base, metricId, itemId)
	return h.mock.EXPECT().
		Do(http.MethodDelete, path, nil, nil)
}

// tests

func TestCmdCreateMetric(t *testing.T) {
	repo := core.Repository{Id: 1215, Url: "repo_url"}

	t.Run("new", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := newMockHelper(ctrl, repo)

		metric := metricModel{Id: 0, Name: "foo"}
		item := itemModel{Id: 0, Name: "bar", MetricId: 1976, ValueType: ValueTypeInt}

		gomock.InOrder(
			h.expectListRepositories2().Return([]core.Repository{h.repo}, nil),
			h.expectListMetrics(),
			h.expectAddMetric(metric, 1976),
			h.expectAddItem(1976, item, 2024),
		)

		cmd := newCommandWithMock(h.mock, repo.Url).newMetricCommand()
		cmd.SetArgs([]string{"-c", "foo/bar"})
		err := cmd.Execute()
		require.NoError(t, err)
	})

	t.Run("addItemToMetric", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := newMockHelper(ctrl, repo)

		item := itemModel{Id: 0, Name: "bar", MetricId: 1976, ValueType: ValueTypeInt}

		gomock.InOrder(
			h.expectListRepositories(),
			h.expectListMetrics2().Return(metricModel{Id: 1976, Name: "foo"}),
			h.expectAddItem(1976, item, 2024),
		)

		cmd := newCommandWithMock(h.mock, repo.Url).newMetricCommand()
		cmd.SetArgs([]string{"-c", "foo/bar"})
		err := cmd.Execute()
		require.NoError(t, err)
	})

	t.Run("no metric", func(t *testing.T) {
		cmd := (&udmCommand{}).newMetricCommand()
		cmd.SetArgs([]string{"-c"})
		err := cmd.Execute()
		require.Error(t, err)
		require.False(t, cmd.SilenceUsage)
	})

	t.Run("invalid metric", func(t *testing.T) {
		cmd := (&udmCommand{}).newMetricCommand()
		cmd.SetArgs([]string{"-c", "foo"})
		err := cmd.Execute()
		require.Error(t, err)
		require.False(t, cmd.SilenceUsage)
	})
}

func TestCmdListMetrics(t *testing.T) {
	repo := core.Repository{Id: 1215, Url: "repo_url"}

	t.Run("list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := newMockHelper(ctrl, repo)

		gomock.InOrder(
			h.expectListRepositories(),
			// h.expectListMetrics(metricModel{Id: 1976, Name: "foo"}),
			h.expectListMetrics2().Return(metricModel{Id: 1976, Name: "foo"}),
			h.expectListItems(1976, itemModel{Id: 2024, Name: "bar", MetricId: 1976}),
		)

		cmd := newCommandWithMock(h.mock, repo.Url).newMetricCommand()
		cmd.SetArgs([]string{"-l"})
		err := cmd.Execute()
		require.NoError(t, err)
	})

	t.Run("no metric", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := newMockHelper(ctrl, repo)

		gomock.InOrder(
			h.expectListRepositories(),
			h.expectListMetrics(),
		)

		cmd := newCommandWithMock(h.mock, repo.Url).newMetricCommand()
		cmd.SetArgs([]string{"-l"})
		err := cmd.Execute()
		require.NoError(t, err)
	})

	// no metric
	// no item
}

func TestCmdDeleteMetric(t *testing.T) {
	repo := core.Repository{Id: 1215, Url: "repo_url"}

	t.Run("delete", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := newMockHelper(ctrl, repo)

		gomock.InOrder(
			h.expectListRepositories(),
			h.expectListMetrics(metricModel{Id: 1976, Name: "foo"}),
			h.expectListItems(1976, itemModel{Id: 2024, Name: "bar", MetricId: 1976}),
			h.expectDeleteItem(1976, 2024),
		)

		cmd := newCommandWithMock(h.mock, repo.Url).newMetricCommand()
		cmd.SetArgs([]string{"-d", "foo/bar"})
		err := cmd.Execute()
		require.NoError(t, err)
	})
}

func TestCmdAddValue(t *testing.T) {
	repo := core.Repository{Id: 1215, Url: "repo_url"}

	t.Run("add_value", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := newMockHelper(ctrl, repo)

		value := valueModel{
			Id: 0, ItemId: 2024, Value: "10",
			Timestamp: time.Date(2024, 2, 26, 0, 0, 0, 0, time.UTC)}

		gomock.InOrder(
			h.expectListRepositories(),
			h.expectListMetrics(metricModel{Id: 1976, Name: "foo"}),
			h.expectListItems(
				1976, itemModel{Id: 2024, Name: "bar", MetricId: 1976}),
			h.expectAddValue(1976, 2024, value),
		)

		cmd := newCommandWithMock(h.mock, repo.Url).newValueCommand()
		cmd.SetArgs([]string{"--add", "foo/bar", "--time", "2024-02-26", "10"})
		err := cmd.Execute()
		require.NoError(t, err)
	})

	t.Run("no_timestamp", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := newMockHelper(ctrl, repo)

		want := valueModel{Id: 0, ItemId: 2024, Value: "10"}

		gomock.InOrder(
			h.expectListRepositories(),
			h.expectListMetrics(metricModel{Id: 1976, Name: "hoge"}),
			h.expectListItems(
				1976, itemModel{Id: 2024, Name: "fuga", MetricId: 1976}),
			h.mock.EXPECT().
				Do(
					http.MethodPost, h.base+"/metrics/1976/items/2024/values",
					gomock.Any(), isValue).
				Do(func(method, path string, in, out any) {
					got, ok := in.(*valueModel)
					require.True(t, ok)
					require.Equal(t, want.Id, got.Id)
					require.Equal(t, want.ItemId, got.ItemId)
					require.Equal(t, want.Value, got.Value)
				}),
		)

		cmd := newCommandWithMock(h.mock, repo.Url).newValueCommand()
		cmd.SetArgs([]string{"--add", "hoge/fuga", "10"})
		err := cmd.Execute()
		require.NoError(t, err)
	})

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	setup := newMockHelper(ctrl, repo)
	cmd := newCommandWithMock(setup.mock, repo.Url).newValueCommand()

	t.Run("no_value", func(t *testing.T) {
		cmd.SetArgs([]string{"--add", "metric/item"})
		err := cmd.Execute()
		require.Error(t, err)
		require.False(t, cmd.SilenceUsage)
	})

	t.Run("invalid timestamp", func(t *testing.T) {
		cmd.SetArgs([]string{"--add", "metric/item", "--time", "2024-02-30", "10"})
		err := cmd.Execute()
		require.Error(t, err)
	})
}

func TestCmdListValues(t *testing.T) {
	repo := core.Repository{Id: 1215, Url: "repo_url"}

	t.Run("list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := newMockHelper(ctrl, repo)

		value := valueModel{
			Id: 0, ItemId: 2024, Value: "10",
			Timestamp: time.Date(2024, 2, 26, 0, 0, 0, 0, time.UTC)}

		gomock.InOrder(
			h.expectListRepositories(),
			h.expectListMetrics(metricModel{Id: 1976, Name: "foo"}),
			h.expectListItems(
				1976, itemModel{Id: 2024, Name: "bar", MetricId: 1976}),
			h.expectListValues(1976, 2024, value),
		)

		cmd := newCommandWithMock(h.mock, repo.Url).newValueCommand()
		cmd.SetArgs([]string{"--list", "foo/bar"})
		err := cmd.Execute()
		require.NoError(t, err)
	})

	t.Run("with non-existing metric", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := newMockHelper(ctrl, repo)

		gomock.InOrder(
			h.expectListRepositories(),
			h.expectListMetrics(metricModel{Id: 1976, Name: "foo"}),
		)

		cmd := newCommandWithMock(h.mock, repo.Url).newValueCommand()

		cmd.SetArgs([]string{"--list", "fuu/bar"})
		err := cmd.Execute()
		require.Error(t, err)
		require.True(t, cmd.SilenceUsage)
	})

	t.Run("with non-existing item", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := newMockHelper(ctrl, repo)

		gomock.InOrder(
			h.expectListRepositories(),
			h.expectListMetrics(metricModel{Id: 1976, Name: "foo"}),
		)

		cmd := newCommandWithMock(h.mock, repo.Url).newValueCommand()

		cmd.SetArgs([]string{"--list", "metric/bar"})
		err := cmd.Execute()
		require.Error(t, err)
	})

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	setup := newMockHelper(ctrl, repo)
	cmd := newCommandWithMock(setup.mock, repo.Url).newValueCommand()

	t.Run("with invalid metric", func(t *testing.T) {
		cmd.SetArgs([]string{"--list", "foo"})
		err := cmd.Execute()
		require.Error(t, err)
		require.False(t, cmd.SilenceUsage)
	})

	t.Run("without metric", func(t *testing.T) {
		cmd.SetArgs([]string{"--list"})
		err := cmd.Execute()
		require.Error(t, err)
		require.False(t, cmd.SilenceUsage)
	})
}

func TestCmdClearValues2(t *testing.T) {
	repo := core.Repository{Id: 1215, Url: "repo_url"}

	t.Run("clear", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := newMockHelper(ctrl, repo)

		gomock.InOrder(
			h.expectListRepositories(),
			h.expectListMetrics(metricModel{Id: 1976, Name: "foo"}),
			h.expectListItems(
				1976, itemModel{Id: 2024, Name: "bar", MetricId: 1976}),
			h.expectDeleteValues(1976, 2024),
		)

		cmd := newCommandWithMock(h.mock, repo.Url).newValueCommand()
		cmd.SetArgs([]string{"--clear", "foo/bar"})
		err := cmd.Execute()
		require.NoError(t, err)
	})
}
