package server

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOAuthHandler_ErrorQuery(t *testing.T) {
	var capturedErr error
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedErr = oauthErrorFrom(r.Context())
	})

	h := NewOAuthHandler(OAuthConfig{
		ClientID: "id", ClientSecret: "secret",
		AuthURL: "https://example.com/auth", TokenURL: "https://example.com/token",
	})
	handler := h.Handler(next)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?error=access_denied", nil)
	handler.ServeHTTP(w, r)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Error(t, capturedErr)
	require.Contains(t, capturedErr.Error(), "access_denied")
}

func TestOAuthHandler_NoCode(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called when no code")
	})

	h := NewOAuthHandler(OAuthConfig{
		ClientID: "id", ClientSecret: "secret",
		AuthURL: "https://example.com/auth", TokenURL: "https://example.com/token",
		RedirectURL: "https://example.com/callback",
	})
	handler := h.Handler(next)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusFound, res.StatusCode)
	loc, err := res.Location()
	require.NoError(t, err)
	require.Contains(t, loc.String(), "https://example.com/auth")
}

func TestOAuthHandler_StateCookieSecure(t *testing.T) {
	tests := []struct {
		name       string
		tls        bool
		xfp        string
		wantSecure bool
	}{
		{name: "plain HTTP request", wantSecure: false},
		{name: "direct TLS connection", tls: true, wantSecure: true},
		{name: "X-Forwarded-Proto https", xfp: "https", wantSecure: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("next should not be called when no code")
			})

			h := NewOAuthHandler(OAuthConfig{
				ClientID: "id", ClientSecret: "secret",
				AuthURL: "https://example.com/auth", TokenURL: "https://example.com/token",
				RedirectURL: "https://example.com/callback",
			})
			handler := h.Handler(next)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if tt.xfp != "" {
				r.Header.Set("X-Forwarded-Proto", tt.xfp)
			}
			handler.ServeHTTP(w, r)
			res := w.Result()
			defer func() { _ = res.Body.Close() }()

			require.Equal(t, http.StatusFound, res.StatusCode)
			var found bool
			for _, c := range res.Cookies() {
				if c.Name == "oauth_state" {
					found = true
					require.Equal(t, tt.wantSecure, c.Secure)
				}
			}
			require.True(t, found, "oauth_state cookie should be set")
		})
	}
}

func TestOAuthHandler_NoStateCookie(t *testing.T) {
	var capturedErr error
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedErr = oauthErrorFrom(r.Context())
	})

	h := NewOAuthHandler(OAuthConfig{
		ClientID: "id", ClientSecret: "secret",
		AuthURL: "https://example.com/auth", TokenURL: "https://example.com/token",
	})
	handler := h.Handler(next)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?code=abc&state=def", nil)
	handler.ServeHTTP(w, r)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Error(t, capturedErr)
	require.Contains(t, capturedErr.Error(), "state cookie not found")
}

func TestOAuthHandler_StateMismatch(t *testing.T) {
	var capturedErr error
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedErr = oauthErrorFrom(r.Context())
	})

	h := NewOAuthHandler(OAuthConfig{
		ClientID: "id", ClientSecret: "secret",
		AuthURL: "https://example.com/auth", TokenURL: "https://example.com/token",
	})
	handler := h.Handler(next)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?code=abc&state=expected", nil)
	r.AddCookie(&http.Cookie{Name: "oauth_state", Value: "wrong", Path: "/", HttpOnly: true})
	handler.ServeHTTP(w, r)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Error(t, capturedErr)
	require.Contains(t, capturedErr.Error(), "state mismatch")
}

func TestBaseRepositoryManager_LoginHandler(t *testing.T) {
	var capturedErr error
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedErr = oauthErrorFrom(r.Context())
	})

	h := NewOAuthHandler(OAuthConfig{
		ClientID: "id", ClientSecret: "secret",
		AuthURL: "https://example.com/auth", TokenURL: "https://example.com/token",
	})
	var b BaseRepositoryManager
	b.Init(1, nil, nil, h, "test")

	handler := b.LoginHandler(next)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?error=some_error", nil)
	handler.ServeHTTP(w, r)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Error(t, capturedErr)
	require.Contains(t, capturedErr.Error(), "some_error")
}

func TestOAuthErrorFrom(t *testing.T) {
	require.Nil(t, oauthErrorFrom(httptest.NewRequest(http.MethodGet, "/", nil).Context()))
}

func TestNewOAuthHandler(t *testing.T) {
	h := NewOAuthHandler(OAuthConfig{
		ClientID:     "my-client",
		ClientSecret: "my-secret",
		AuthURL:      "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		RedirectURL:  "https://example.com/cb",
		Scopes:       []string{"repo", "user"},
	})
	require.NotNil(t, h)
	require.NotNil(t, h.config)
	require.Equal(t, "my-client", h.config.ClientID)
	require.Equal(t, []string{"repo", "user"}, h.config.Scopes)
}

func TestWithOAuthError(t *testing.T) {
	baseErr := errors.New("test error")
	ctx := withOAuthError(httptest.NewRequest(http.MethodGet, "/", nil).Context(), baseErr)
	got := oauthErrorFrom(ctx)
	require.Equal(t, baseErr, got)
}
