package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type (
	APIClient interface {
		Do(method, path string, in any, out any) error
		ListRepositories() ([]Repository, error)
	}

	APIClientImpl struct {
		BaseURL string
		Token   string
		Client  *http.Client
	}
)

func (c *APIClientImpl) Do(method, path string, in any, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("json.Marshal request body: %w", err)
		}
		body = bytes.NewBuffer(data)
	}

	url := fmt.Sprintf("%s%s", c.BaseURL, path)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("http.NewRequest %s %s: %w", method, url, err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("client.Do %s %s: %w", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	msg, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("io.ReadAll response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var e ErrorResponse
		err = json.Unmarshal(msg, &e)
		if err != nil {
			return fmt.Errorf("json.Unmarshal error response: %w", err)
		}

		return errors.New(e.Message)
	}

	if out != nil {
		if err := json.Unmarshal(msg, out); err != nil {
			return fmt.Errorf("json.Unmarshal response: %w", err)
		}
	}

	return nil
}

func (c *APIClientImpl) ListRepositories() ([]Repository, error) {
	var repos []Repository
	err := c.Do(http.MethodGet, "/api/repos", nil, &repos)
	return repos, err
}
