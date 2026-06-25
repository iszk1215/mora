package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iszk1215/mora/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorCode(t *testing.T) {
	w := httptest.NewRecorder()
	err := errors.New("test error")
	ErrorCode(w, err, http.StatusBadRequest)

	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

	var body core.ErrorResponse
	err = json.NewDecoder(res.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "test error", body.Message)
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	InternalError(w, errors.New("internal"))

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)

	var body core.ErrorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "internal server error", body.Message)
}

func TestInternalErrorf(t *testing.T) {
	w := httptest.NewRecorder()
	InternalErrorf(w, "error %d: %s", 42, "timeout")

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)

	var body core.ErrorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "internal server error", body.Message)
}

func TestNotImplemented(t *testing.T) {
	w := httptest.NewRecorder()
	NotImplemented(w, errors.New("not implemented"))

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	assert.Equal(t, http.StatusNotImplemented, res.StatusCode)

	var body core.ErrorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "not implemented", body.Message)
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	NotFound(w, errors.New("not found"))

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	assert.Equal(t, http.StatusNotFound, res.StatusCode)

	var body core.ErrorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "not found", body.Message)
}

func TestNotFoundf(t *testing.T) {
	w := httptest.NewRecorder()
	NotFoundf(w, "repo %d not found", 5)

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	assert.Equal(t, http.StatusNotFound, res.StatusCode)

	var body core.ErrorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "repo 5 not found", body.Message)
}

func TestUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	Unauthorized(w, errors.New("unauthorized"))

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)

	var body core.ErrorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "unauthorized", body.Message)
}

func TestForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	Forbidden(w, errors.New("forbidden"))

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	assert.Equal(t, http.StatusForbidden, res.StatusCode)

	var body core.ErrorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "forbidden", body.Message)
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	BadRequest(w, errors.New("bad request"))

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)

	var body core.ErrorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "bad request", body.Message)
}

func TestBadRequestf(t *testing.T) {
	w := httptest.NewRecorder()
	BadRequestf(w, "invalid %s: %s", "name", "empty")

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)

	var body core.ErrorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "invalid name: empty", body.Message)
}

func TestJSON(t *testing.T) {
	t.Run("custom struct", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}{Name: "test", Value: 42}
		JSON(w, &data, http.StatusOK)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

		var result map[string]interface{}
		err := json.NewDecoder(res.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "test", result["name"])
		assert.Equal(t, float64(42), result["value"])
	})

	t.Run("nil body", func(t *testing.T) {
		w := httptest.NewRecorder()
		JSON(w, nil, http.StatusNoContent)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		assert.Equal(t, http.StatusNoContent, res.StatusCode)
	})

	t.Run("status 201", func(t *testing.T) {
		w := httptest.NewRecorder()
		JSON(w, &core.ErrorResponse{Message: "created"}, http.StatusCreated)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		assert.Equal(t, http.StatusCreated, res.StatusCode)
	})
}

func TestJSONIndent(t *testing.T) {
	indent = true
	t.Cleanup(func() { indent = false })

	w := httptest.NewRecorder()
	JSON(w, &core.ErrorResponse{Message: "test"}, http.StatusOK)

	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	bodyBytes, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	bodyStr := string(bodyBytes)
	assert.True(t, strings.HasPrefix(bodyStr, "{\n"), "expected indented JSON, got: %s", bodyStr)

	var decoded core.ErrorResponse
	err = json.Unmarshal([]byte(bodyStr), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "test", decoded.Message)
}

func TestSentinelErrors(t *testing.T) {
	assert.Equal(t, "invalid or missing token", ErrInvalidToken.Error())
	assert.Equal(t, "unauthorized", ErrUnauthorized.Error())
	assert.Equal(t, "forbidden", ErrForbidden.Error())
	assert.Equal(t, "not found", ErrNotFound.Error())
	assert.Equal(t, "not implemented", ErrNotImplemented.Error())
}

func TestErrorCodeWithSentinel(t *testing.T) {
	tests := []struct {
		name       string
		fn         func(w http.ResponseWriter, err error)
		err        error
		wantStatus int
		wantMsg    string
	}{
		{"InternalError", InternalError, ErrInvalidToken, http.StatusInternalServerError, "internal server error"},
		{"NotImplemented", NotImplemented, ErrNotImplemented, http.StatusNotImplemented, "not implemented"},
		{"NotFound", NotFound, ErrNotFound, http.StatusNotFound, "not found"},
		{"Unauthorized", Unauthorized, ErrUnauthorized, http.StatusUnauthorized, "unauthorized"},
		{"Forbidden", Forbidden, ErrForbidden, http.StatusForbidden, "forbidden"},
		{"BadRequest", BadRequest, fmt.Errorf("wrapped: %w", ErrNotFound), http.StatusBadRequest, "wrapped: not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.fn(w, tt.err)

			res := w.Result()
			defer func() { _ = res.Body.Close() }()
			assert.Equal(t, tt.wantStatus, res.StatusCode)

			var body core.ErrorResponse
			require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
			assert.Equal(t, tt.wantMsg, body.Message)
		})
	}
}
