package server

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/drone/go-scm/scm"
	"github.com/stretchr/testify/require"
)

type failReader struct{}

func (f *failReader) Read(p []byte) (int, error) {
	return 0, errors.New("mock read error")
}

func newTestSessionManager() *MoraSessionManager {
	return &MoraSessionManager{
		cookiename: "morasessionid",
		store:      map[string]*MoraSession{},
		lifetime:   24 * time.Hour,
		stopCh:     make(chan struct{}),
	}
}

func TestSessionID_ReturnsErrorOnReadFailure(t *testing.T) {
	old := rand.Reader
	rand.Reader = &failReader{}
	defer func() { rand.Reader = old }()

	id, err := sessionID()
	require.Error(t, err)
	require.Empty(t, id)
}

func TestSessionMiddleware_Returns500OnSessionIDError(t *testing.T) {
	old := rand.Reader
	rand.Reader = &failReader{}
	defer func() { rand.Reader = old }()

	m := newTestSessionManager()
	next := func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}
	handler := m.SessionMiddleware(http.HandlerFunc(next))
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)

	require.Equal(t, http.StatusInternalServerError, got.Code)
}

func TestSessionManager(t *testing.T) {
	m := newTestSessionManager()
	next := func(w http.ResponseWriter, r *http.Request) {
		_, ok := MoraSessionFrom(r.Context())
		require.True(t, ok)
	}
	handler := m.SessionMiddleware(http.HandlerFunc(next))
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)
}

func TestSessionManager_SetsCookie(t *testing.T) {
	m := newTestSessionManager()
	next := func(w http.ResponseWriter, r *http.Request) {}
	handler := m.SessionMiddleware(http.HandlerFunc(next))
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)

	res := got.Result()
	cookies := res.Cookies()
	require.Len(t, cookies, 1)

	cookie := cookies[0]
	require.Equal(t, "morasessionid", cookie.Name)
	require.NotEmpty(t, cookie.Value)
	require.Equal(t, "/", cookie.Path)
	require.True(t, cookie.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	// Secure is enabled by default even when the server runs behind plain HTTP.
	require.True(t, cookie.Secure)
}

func TestSessionManager_InsecureCookie(t *testing.T) {
	m := newTestSessionManager()
	m.insecureCookie = true
	next := func(w http.ResponseWriter, r *http.Request) {}
	handler := m.SessionMiddleware(http.HandlerFunc(next))

	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)

	cookies := got.Result().Cookies()
	require.Len(t, cookies, 1)
	require.False(t, cookies[0].Secure,
		"Secure attribute should be omitted with insecure_cookie over HTTP")
}

func TestSessionManager_InsecureCookieStaysSecureOverHTTPS(t *testing.T) {
	tests := []struct {
		name string
		tls  bool
		xfp  string
	}{
		{name: "direct TLS connection", tls: true},
		{name: "X-Forwarded-Proto https", xfp: "https"},
		{name: "X-Forwarded-Proto HTTPS uppercase", xfp: "HTTPS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestSessionManager()
			m.insecureCookie = true
			handler := m.SessionMiddleware(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {}))

			req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			if tt.xfp != "" {
				req.Header.Set("X-Forwarded-Proto", tt.xfp)
			}
			got := httptest.NewRecorder()
			handler.ServeHTTP(got, req)

			cookies := got.Result().Cookies()
			require.Len(t, cookies, 1)
			require.True(t, cookies[0].Secure,
				"Secure should stay enabled when the request itself is over HTTPS")
		})
	}
}

func TestSessionManager_ReusesExistingSession(t *testing.T) {
	m := newTestSessionManager()

	// First request: no cookie -> middleware creates new session
	var firstSid string
	first := func(w http.ResponseWriter, r *http.Request) {
		sess, ok := MoraSessionFrom(r.Context())
		require.True(t, ok)
		sess.setToken(1, scm.Token{Token: "test-token"})
	}
	handler := m.SessionMiddleware(http.HandlerFunc(first))
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)

	res := got.Result()
	cookies := res.Cookies()
	require.Len(t, cookies, 1)
	firstSid = cookies[0].Value

	// Second request: include the cookie -> middleware reuses existing session
	var gotToken string
	second := func(w http.ResponseWriter, r *http.Request) {
		sess, ok := MoraSessionFrom(r.Context())
		require.True(t, ok)
		tok, ok := sess.getToken(1)
		require.True(t, ok, "token from first request should persist")
		gotToken = tok.Token
	}
	handler2 := m.SessionMiddleware(http.HandlerFunc(second))
	req2 := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	req2.AddCookie(&http.Cookie{Name: "morasessionid", Value: firstSid})
	got2 := httptest.NewRecorder()
	handler2.ServeHTTP(got2, req2)

	require.Equal(t, "test-token", gotToken)
}

func TestMoraSession_Remove(t *testing.T) {
	sess := NewMoraSession()
	sess.setToken(1, scm.Token{Token: "token1"})
	sess.setToken(2, scm.Token{Token: "token2"})
	sess.setReposCache(1, map[int64]bool{42: true})

	_, ok := sess.getToken(1)
	require.True(t, ok)
	_, ok = sess.getToken(2)
	require.True(t, ok)

	sess.Remove(1)

	_, ok = sess.getToken(1)
	require.False(t, ok, "token for rm 1 should be removed")
	_, ok = sess.getToken(2)
	require.True(t, ok, "token for rm 2 should remain")
	cache := sess.getReposCache(1)
	require.Nil(t, cache, "repos cache for rm 1 should be removed")
}

func TestMoraSessionDirectRace(t *testing.T) {
	t.Parallel()
	sess := NewMoraSession()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rmID := int64(i % 5)
			sess.setToken(rmID, scm.Token{Token: "token"})
			sess.getToken(rmID)
			sess.setReposCache(rmID, map[int64]bool{int64(i): true})
			sess.getReposCache(rmID)
		}(i)
	}
	wg.Wait()
}

func TestMoraSessionManager_GC(t *testing.T) {
	m := newTestSessionManager()
	m.lifetime = time.Hour

	now := time.Now()
	sess1 := NewMoraSession()
	sess1.timestamp = now.Add(-2 * time.Hour) // expired
	m.store["sid1"] = sess1

	sess2 := NewMoraSession()
	sess2.timestamp = now.Add(-30 * time.Minute) // still valid
	m.store["sid2"] = sess2

	m.GC()

	require.Len(t, m.store, 1)
	_, ok := m.store["sid1"]
	require.False(t, ok, "expired session should be removed")
	_, ok = m.store["sid2"]
	require.True(t, ok, "valid session should remain")
}

func TestMoraSessionConcurrentHTTPRace(t *testing.T) {
	t.Parallel()
	m := newTestSessionManager()
	handler := m.SessionMiddleware(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			sess, _ := MoraSessionFrom(r.Context())
			sess.setToken(1, scm.Token{Token: "test", Refresh: "refresh"})
			sess.getToken(1)
			sess.setReposCache(1, map[int64]bool{42: true})
			sess.getReposCache(1)
		},
	))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: "morasessionid", Value: "shared-sid"})
			handler.ServeHTTP(httptest.NewRecorder(), req)
		}()
	}
	wg.Wait()
}
