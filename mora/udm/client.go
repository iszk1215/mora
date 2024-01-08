package udm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/iszk1215/mora/mora/model"
	"github.com/rs/zerolog/log"
)

type (
	udmClient interface {
		init(serverAddr, token string)
		listRepositories() ([]model.Repository, error)
		listMetrics(repoId int64) ([]metricModel, error)
		listItems(repoId int64, metricId int64) ([]itemModel, error)
		addMetric(repoId int64, metric *metricModel) error
		addItem(repoId int64, metric *itemModel) error
		deleteItem(repoId, metricId, itemId int64) error

		addValue(repoId, metricId int64, value *valueModel) error
	}

	udmClientImpl struct {
		serverAddr string
		// repoUrl    string
		token  string
		client *http.Client
	}
)

func (c *udmClientImpl) do(method, path string, in any, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewBuffer(data)
	}

	url := fmt.Sprintf("http://%s%s", c.serverAddr, path)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer " + c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	msg, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Print(string(msg))
		log.Print("URL=", req.URL)
		type Error struct {
			Message string `json:"message"`
		}

		var e Error
		err = json.Unmarshal(msg, &e)
		if err != nil {
			return err
		}

		return errors.New(e.Message)
	}

	if out != nil {
		return json.Unmarshal(msg, out)
	}

	return nil
}

// ----------------------------------------------------------------------
// udmClient

func (c *udmClientImpl) init(serverAddr, token string) {
	c.serverAddr = serverAddr
	c.token = token
}

func (c *udmClientImpl) listRepositories() ([]model.Repository, error) {
	log.Print("udmClientImpl.listRepositories")

	var repos []model.Repository
	err := c.do(http.MethodGet, "/api/repos", nil, &repos)
	if err != nil {
		return []model.Repository{}, err
	}

	return repos, nil
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
