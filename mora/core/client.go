package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type (
	APIClient struct {
		BaseURL string
		Token   string
		Client  *http.Client
	}
)

func (c *APIClient) Do(method, path string, in any, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewBuffer(data)
	}

	url := fmt.Sprintf("%s%s", c.BaseURL, path)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	client := c.Client
	if client == nil {
		client = &http.Client{}
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	msg, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
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

func (c *APIClient) ListRepositories() ([]Repository, error) {
	var repos []Repository
	err := c.Do(http.MethodGet, "/api/repos", nil, &repos)
	return repos, err
}

