package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iszk1215/mora/core"
	"github.com/stretchr/testify/require"
)

func setupUserTypeServer(t *testing.T) (*MoraServer, http.Handler) {
	return setupUserPageServer(t)
}

// newUserTypeSession registers a session for the given user and returns a
// function that attaches the session cookie to a request.

func newUserTypeSession(t *testing.T, server *MoraServer, userID int64, name string) func(r *http.Request) *http.Request {
	t.Helper()
	sess := NewMoraSession()
	sess.SetUserID(userID)
	server.sessionManager.store[name] = sess
	return func(r *http.Request) *http.Request {
		r.AddCookie(&http.Cookie{Name: "morasessionid", Value: name})
		return r
	}
}

func TestHandleUserSetType(t *testing.T) {
	server, handler := setupUserTypeServer(t)

	target, err := server.userStore.CreateUser("targetuser", "")
	require.NoError(t, err)

	adminReq := newUserTypeSession(t, server, 1, "usertype-admin-sess")
	userReq := newUserTypeSession(t, server, target.ID, "usertype-target-sess")

	request := func(r *http.Request) *http.Response {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Result()
	}

	patch := func(cookie func(*http.Request) *http.Request, userName, body string) *http.Request {
		r := httptest.NewRequest(http.MethodPatch,
			fmt.Sprintf("/api/users/%s/type", userName), strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		return cookie(r)
	}

	t.Run("admin sets user type to pro", func(t *testing.T) {
		res := request(patch(adminReq, "targetuser", `{"user_type":"pro"}`))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)

		var got User
		require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
		require.Equal(t, target.ID, got.ID)
		require.Equal(t, core.UserTypePro, got.Type)

		found, err := server.userStore.FindByID(target.ID)
		require.NoError(t, err)
		require.Equal(t, core.UserTypePro, found.Type)
	})

	t.Run("non-admin is forbidden", func(t *testing.T) {
		res := request(patch(userReq, "targetuser", `{"user_type":"pro"}`))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusForbidden, res.StatusCode)
	})

	t.Run("anonymous is forbidden", func(t *testing.T) {
		res := request(httptest.NewRequest(http.MethodPatch,
			"/api/users/targetuser/type", strings.NewReader(`{"user_type":"pro"}`)))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusForbidden, res.StatusCode)
	})

	t.Run("invalid type is rejected", func(t *testing.T) {
		res := request(patch(adminReq, "targetuser", `{"user_type":"enterprise"}`))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("malformed body is rejected", func(t *testing.T) {
		res := request(patch(adminReq, "targetuser", `not-json`))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("unknown user returns 404", func(t *testing.T) {
		res := request(patch(adminReq, "nobody", `{"user_type":"pro"}`))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("cannot change own type", func(t *testing.T) {
		res := request(patch(adminReq, "admin", `{"user_type":"free"}`))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusForbidden, res.StatusCode)
	})

	t.Run("admin can set back to free", func(t *testing.T) {
		res := request(patch(adminReq, "targetuser", `{"user_type":"free"}`))
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)

		found, err := server.userStore.FindByID(target.ID)
		require.NoError(t, err)
		require.Equal(t, core.UserTypeFree, found.Type)
	})
}
