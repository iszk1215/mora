package core

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/iszk1215/mora/mora/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type RoundTripFunc func(req *http.Request) (*http.Response, error)

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testRequest(t *testing.T, method, path string, req *http.Request) {
	baseURL := "http://mora.test:4000"
	token := "valid_token"

	url, err := url.Parse(baseURL + path)
	require.NoError(t, err)

	assert.Equal(t, method, req.Method)
	assert.Equal(t, url, req.URL)
	assert.Equal(t, "Bearer "+token, req.Header.Get("Authorization"))
}

func makeResponse(status int, resp any) (*http.Response, error) {
	var buf = new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(resp); err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(buf),
	}, nil
}

func newMockAPIClient(roundTrip RoundTripFunc) *APIClient {
	baseURL := "http://mora.test:4000"
	token := "valid_token"

	client := &http.Client{
		Transport: roundTrip,
	}

	return &APIClient{baseURL, token, client}
}

func TestAPIClientDo(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		fn := func(req *http.Request) (*http.Response, error) {
			testRequest(t, http.MethodGet, "/", req)
			return makeResponse(http.StatusOK, []Repository{})
		}

		c := newMockAPIClient(fn)
		err := c.Do(http.MethodGet, "/", nil, nil)
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		errorMessage := "error"

		fn := func(req *http.Request) (*http.Response, error) {
			testRequest(t, http.MethodGet, "/", req)
			return makeResponse(http.StatusNotFound, errors.New(errorMessage))
		}

		c := newMockAPIClient(fn)
		err := c.Do(http.MethodGet, "/", nil, nil)
		require.Error(t, err)
		require.Equal(t, errorMessage, err.Error())
	})
}

func TestAPIClientListRepositories(t *testing.T) {

	fn := func(req *http.Request) (*http.Response, error) {
		testRequest(t, http.MethodGet, "/api/repos", req)
		return makeResponse(http.StatusOK, []Repository{})
	}

	c := newMockAPIClient(fn)
	_, err := c.ListRepositories()
	require.NoError(t, err)
}
