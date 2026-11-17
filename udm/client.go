package udm

import (
	"fmt"
	"net/http"

	"github.com/iszk1215/mora/core"
	"github.com/rs/zerolog/log"
)

type (
	udmClient struct {
		client core.APIClient
	}
)

func (c *udmClient) do(method, path string, in any, out any) error {
	return c.client.Do(method, path, in, out)
}

// ----------------------------------------------------------------------
// udmClient

func (c *udmClient) listRepositories() ([]core.Repository, error) {
	log.Print("udmClientImpl.listRepositories")
	return c.client.ListRepositories()
}

func (c *udmClient) addMetric(repoId string, metric *MetricModel) error {
	path := fmt.Sprintf("/api/repos/%s/udm/metrics", repoId)
	return c.do(http.MethodPost, path, metric, metric)
}

func (c *udmClient) listMetrics(repoId string) ([]MetricModel, error) {
	path := fmt.Sprintf("/api/repos/%s/udm/metrics", repoId)
	var resp ListMetricsResponse
	err := c.do(http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Metrics, nil
}

func (c *udmClient) addItem(repoId string, item *ItemModel) error {
	path := fmt.Sprintf("/api/repos/%s/udm/metrics/%d/items",
		repoId, item.MetricId)
	return c.do(http.MethodPost, path, item, item)
}

func (c *udmClient) deleteItem(repoId string, metricId int64, itemId int64) error {
	path := fmt.Sprintf("/api/repos/%s/udm/metrics/%d/items/%d",
		repoId, metricId, itemId)
	return c.do(http.MethodDelete, path, nil, nil)
}

func (c *udmClient) listItems(repoId string, metricId int64) ([]ItemModel, error) {
	path := fmt.Sprintf("/api/repos/%s/udm/metrics/%d/items", repoId, metricId)

	var resp ListItemsResponse
	err := c.do(http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Items, nil
}

func (c *udmClient) addValue(repoId string, metricId int64, value *ValueModel) error {
	path := fmt.Sprintf("/api/repos/%s/udm/metrics/%d/items/%d/values",
		repoId, metricId, value.ItemId)
	return c.do(http.MethodPost, path, value, value)
}

func (c *udmClient) listValues(repoId string, metricId, itemId int64) ([]ValueModel, error) {
	path := fmt.Sprintf("/api/repos/%s/udm/metrics/%d/items/%d/values",
		repoId, metricId, itemId)
	var resp ListValuesResponse
	err := c.do(http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Values, nil
}

func (c *udmClient) deleteValues(repoId string, metricId, itemId int64) error {
	path := fmt.Sprintf("/api/repos/%s/udm/metrics/%d/items/%d/values",
		repoId, metricId, itemId)
	return c.do(http.MethodDelete, path, nil, nil)
}

func newUdmClient(baseURL, token string) *udmClient {
	return &udmClient{
		client: &core.APIClientImpl{
			BaseURL: baseURL,
			Token:   token,
		},
	}
}
