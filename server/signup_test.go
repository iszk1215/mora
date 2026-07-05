package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignupHandler_NoSession(t *testing.T) {
	h := SignupHandler(nil)

	t.Run("GET /pending", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/pending", nil)
		h.ServeHTTP(w, r)
		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("POST /cancel", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/cancel", nil)
		h.ServeHTTP(w, r)
		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("POST /confirm", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/confirm", nil)
		h.ServeHTTP(w, r)
		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})
}

func TestSignupHandler_NoPendingSignup(t *testing.T) {
	h := SignupHandler(nil)
	sess := NewMoraSession()

	t.Run("GET /pending", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/pending", nil)
		r = r.WithContext(WithMoraSession(r.Context(), sess))
		h.ServeHTTP(w, r)
		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("POST /cancel", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/cancel", nil)
		r = r.WithContext(WithMoraSession(r.Context(), sess))
		h.ServeHTTP(w, r)
		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("POST /confirm", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/confirm", nil)
		r = r.WithContext(WithMoraSession(r.Context(), sess))
		h.ServeHTTP(w, r)
		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})
}

func TestSignupHandler_GetPending(t *testing.T) {
	h := SignupHandler(nil)
	sess := NewMoraSession()
	sess.SetPendingSignup(&pendingSignup{
		provider: "gitea",
		username: "newuser",
		avatarURL: "https://example.com/avatar.jpg",
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/pending", nil)
	r = r.WithContext(WithMoraSession(r.Context(), sess))
	h.ServeHTTP(w, r)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusOK, res.StatusCode)
	body, _ := io.ReadAll(res.Body)
	var got PendingSignupResponse
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "gitea", got.Provider)
	require.Equal(t, "newuser", got.Username)
	require.Equal(t, "https://example.com/avatar.jpg", got.AvatarURL)
}

func TestSignupHandler_Cancel(t *testing.T) {
	h := SignupHandler(nil)
	sess := NewMoraSession()
	sess.SetUserID(1)
	sess.SetPendingSignup(&pendingSignup{
		rmID:     1,
		provider: "gitea",
		username: "newuser",
	})

	csrfToken := "test-csrf"
	body := strings.NewReader("csrf_token=" + csrfToken)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/cancel", body)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken, Path: "/", HttpOnly: false})
	r = r.WithContext(WithMoraSession(r.Context(), sess))
	h.ServeHTTP(w, r)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusNoContent, res.StatusCode)
	require.Nil(t, sess.PendingSignup())
	require.Nil(t, sess.UserID())
}

func TestSignupHandler_Cancel_CSRFMismatch(t *testing.T) {
	h := SignupHandler(nil)
	sess := NewMoraSession()
	sess.SetPendingSignup(&pendingSignup{
		provider: "gitea",
		username: "newuser",
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/cancel", strings.NewReader("csrf_token=wrong"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "expected", Path: "/", HttpOnly: false})
	r = r.WithContext(WithMoraSession(r.Context(), sess))
	h.ServeHTTP(w, r)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusForbidden, res.StatusCode)
}

func TestSignupHandler_Confirm_CSRFMismatch(t *testing.T) {
	store := newTestUserStore(t)
	h := SignupHandler(store)
	sess := NewMoraSession()
	sess.SetPendingSignup(&pendingSignup{
		provider: "gitea",
		username: "newuser",
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/confirm", strings.NewReader("csrf_token=wrong"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "expected", Path: "/", HttpOnly: false})
	r = r.WithContext(WithMoraSession(r.Context(), sess))
	h.ServeHTTP(w, r)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusForbidden, res.StatusCode)
}

func TestSignupHandler_Confirm_Success(t *testing.T) {
	store := newTestUserStore(t)
	h := SignupHandler(store)
	sess := NewMoraSession()
	sess.SetPendingSignup(&pendingSignup{
		provider:       "gitea",
		providerUserID: "42",
		username:       "confirmeduser",
		avatarURL:      "https://example.com/av.jpg",
	})

	csrfToken := "test-csrf"
	body := strings.NewReader("csrf_token=" + csrfToken)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/confirm", body)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken, Path: "/", HttpOnly: false})
	r = r.WithContext(WithMoraSession(r.Context(), sess))
	h.ServeHTTP(w, r)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusCreated, res.StatusCode)

	user, err := store.FindByUsername("confirmeduser")
	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, "confirmeduser", user.Username)
	require.Equal(t, "https://example.com/av.jpg", user.AvatarURL)

	require.NotNil(t, sess.UserID())
	require.Equal(t, user.ID, *sess.UserID())
	require.Nil(t, sess.PendingSignup())
}

