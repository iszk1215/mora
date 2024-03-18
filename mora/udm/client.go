package udm

import (
	"fmt"
	"net/http"
	"os"

	"github.com/iszk1215/mora/mora/core"
	"github.com/rs/zerolog/log"
)

type (
	udmClient interface {
		init(serverAddr, token string)
		listRepositories() ([]core.Repository, error)
		listMetrics(repoId int64) ([]metricModel, error)
		listItems(repoId int64, metricId int64) ([]itemModel, error)
		addMetric(repoId int64, metric *metricModel) error
		addItem(repoId int64, metric *itemModel) error
		deleteItem(repoId, metricId, itemId int64) error

		addValue(repoId, metricId int64, value *valueModel) error
		listValues(repoId, metridId, itemId int64) ([]valueModel, error)
		deleteValues(repoId, metridId, itemId int64) error
	}

	udmClientImpl struct {
		serverAddr string
		token string
	}
)

func (c *udmClientImpl) newClient() *core.APIClient {
	return &core.APIClient{
		BaseURL: c.serverAddr,
		Token:   os.Getenv("MORA_API_KEY"),
	}
}

func (c *udmClientImpl) do(method, path string, in any, out any) error {
	return c.newClient().Do(method, path, in, out)
}


// ----------------------------------------------------------------------
// udmClient

func (c *udmClientImpl) init(serverAddr, token string) {
	c.serverAddr = serverAddr
	c.token = token
}

func (c *udmClientImpl) listRepositories() ([]core.Repository, error) {
	log.Print("udmClientImpl.listRepositories")
	return c.newClient().ListRepositories();
}

func (c *udmClientImpl) addMetric(repoId int64, metric *metricModel) error {
	path := fmt.Sprintf("/api/repos/%d/udm/metrics", repoId)
	return c.do(http.MethodPost, path, metric, metric)
}

func (c *udmClientImpl) listMetrics(repoId int64) ([]metricModel, error) {
	path := fmt.Sprintf("/api/repos/%d/udm/metrics", repoId)
	var resp listMetricsResponse
	err := c.do(http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Metrics, nil
}

func (c *udmClientImpl) addItem(repoId int64, item *itemModel) error {
	path := fmt.Sprintf("/api/repos/%d/udm/metrics/%d/items",
		repoId, item.MetricId)
	return c.do(http.MethodPost, path, item, item)
}

func (c *udmClientImpl) deleteItem(repoId int64, metricId int64, itemId int64) error {
	path := fmt.Sprintf("/api/repos/%d/udm/metrics/%d/items/%d",
		repoId, metricId, itemId)
	return c.do(http.MethodDelete, path, nil, nil)
}

func (c *udmClientImpl) listItems(repoId int64, metricId int64) ([]itemModel, error) {
	path := fmt.Sprintf("/api/repos/%d/udm/metrics/%d/items", repoId, metricId)

	var resp listItemsResponse
	err := c.do(http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Items, nil
}

func (c *udmClientImpl) addValue(repoId, metricId int64, value *valueModel) error {
	path := fmt.Sprintf("/api/repos/%d/udm/metrics/%d/items/%d/values",
		repoId, metricId, value.ItemId)
	return c.do(http.MethodPost, path, value, value)
}

func (c *udmClientImpl) listValues(repoId, metricId, itemId int64) ([]valueModel, error) {
	path := fmt.Sprintf("/api/repos/%d/udm/metrics/%d/items/%d/values",
		repoId, metricId, itemId)
	var resp listValuesResponse
	err := c.do(http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Values, nil
}

func (c *udmClientImpl) deleteValues(repoId, metricId, itemId int64) error {
	path := fmt.Sprintf("/api/repos/%d/udm/metrics/%d/items/%d/values",
		repoId, metricId, itemId)
	return c.do(http.MethodDelete, path, nil, nil)
}
