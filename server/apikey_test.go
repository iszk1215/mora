package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

const testCSRFToken = "test-csrf-token-value"

func newTestAPIKeyHandler(t *testing.T) (http.Handler, *MoraSessionManager, UserStore) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	userStore := NewUserStore(db)
	require.NoError(t, userStore.Init())

	sm := NewMoraSessionManager()
	handler := APIKeyHandler(userStore)

	router := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.SessionMiddleware(handler).ServeHTTP(w, r)
	})

	return router, sm, userStore
}

func loggedInSession(t *testing.T, sm *MoraSessionManager, userID int64) []*http.Cookie {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	sm.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := MoraSessionFrom(r.Context())
		sess.SetUserID(userID)
	})).ServeHTTP(w, r)
	cookies := w.Result().Cookies()
	// Add a CSRF cookie for mutation endpoints
	cookies = append(cookies, &http.Cookie{
		Name:  csrfCookieName,
		Value: testCSRFToken,
		Path:  "/",
	})
	return cookies
}

func TestAPIKeyHandlerListRequiresAuth(t *testing.T) {
	handler, _, _ := newTestAPIKeyHandler(t)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestAPIKeyHandlerListEmpty(t *testing.T) {
	handler, sm, userStore := newTestAPIKeyHandler(t)

	user, err := userStore.CreateUser("listempty", "")
	require.NoError(t, err)

	cookies := loggedInSession(t, sm, user.ID)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	var keys []UserAPIKey
	err = json.NewDecoder(w.Result().Body).Decode(&keys)
	require.NoError(t, err)
	require.Empty(t, keys)
}

func TestAPIKeyHandlerCreate(t *testing.T) {
	handler, sm, userStore := newTestAPIKeyHandler(t)

	user, err := userStore.CreateUser("createkey", "")
	require.NoError(t, err)

	cookies := loggedInSession(t, sm, user.ID)

	body := bytes.NewReader([]byte(`{"name":"my key"}`))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", testCSRFToken)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusCreated, w.Code)

	var resp CreateAPIKeyResponse
	err = json.NewDecoder(w.Result().Body).Decode(&resp)
	require.NoError(t, err)
	require.Equal(t, "my key", resp.Name)
	require.Contains(t, resp.Key, "mora_")
	require.Len(t, resp.Key, 69)
	require.NotEmpty(t, resp.KeyPrefix)
}

func TestAPIKeyHandlerCreateNoName(t *testing.T) {
	handler, sm, userStore := newTestAPIKeyHandler(t)

	user, err := userStore.CreateUser("nocreate", "")
	require.NoError(t, err)

	cookies := loggedInSession(t, sm, user.ID)

	body := bytes.NewReader([]byte(`{}`))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", testCSRFToken)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)

	b, _ := io.ReadAll(w.Result().Body)
	require.Contains(t, string(b), "name is required")
}

func TestAPIKeyHandlerRevoke(t *testing.T) {
	handler, sm, userStore := newTestAPIKeyHandler(t)

	user, err := userStore.CreateUser("revokekey", "")
	require.NoError(t, err)

	plaintext, err := userStore.CreateAPIKey(user.ID, "torevoke")
	require.NoError(t, err)
	require.NotEmpty(t, plaintext)

	keys, err := userStore.ListAPIKeys(user.ID)
	require.NoError(t, err)
	require.Len(t, keys, 1)

	cookies := loggedInSession(t, sm, user.ID)

	r := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/%d", keys[0].ID), nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	r.Header.Set("X-CSRF-Token", testCSRFToken)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusNoContent, w.Code)

	keys, err = userStore.ListAPIKeys(user.ID)
	require.NoError(t, err)
	require.Empty(t, keys)
}

func TestAPIKeyHandlerRevokeNotFound(t *testing.T) {
	handler, sm, userStore := newTestAPIKeyHandler(t)

	user, err := userStore.CreateUser("revokenf", "")
	require.NoError(t, err)

	cookies := loggedInSession(t, sm, user.ID)

	r := httptest.NewRequest(http.MethodDelete, "/999", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	r.Header.Set("X-CSRF-Token", testCSRFToken)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestAPIKeyHandlerRevokeRequiresAuth(t *testing.T) {
	handler, _, _ := newTestAPIKeyHandler(t)

	r := httptest.NewRequest(http.MethodDelete, "/1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestAPIKeyHandlerCreateRequiresCSRF(t *testing.T) {
	handler, sm, userStore := newTestAPIKeyHandler(t)

	user, err := userStore.CreateUser("nocsrf", "")
	require.NoError(t, err)

	cookies := loggedInSession(t, sm, user.ID)

	body := bytes.NewReader([]byte(`{"name":"no csrf key"}`))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	r.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusForbidden, w.Code)
}
