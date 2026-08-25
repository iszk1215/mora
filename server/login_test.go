package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/drone/go-scm/scm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockLoginMiddleware struct {
	redirectURL string
}

func (m MockLoginMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		if len(code) == 0 {
			http.Redirect(w, r, m.redirectURL+"?code=12345",
				http.StatusFound)
			return
		}

		token := &scm.Token{
			Token: "MockAccessToken",
		}

		ctx := scm.WithContext(r.Context(), token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func createTestLoginHandler(rm RepositoryManager) http.Handler {
	next := func(w http.ResponseWriter, r *http.Request) {}
	return LoginHandler([]RepositoryManager{rm}, nil, false, http.HandlerFunc(next))
}

func NewGetRequestWithMoraSession(path string, sess *MoraSession) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
	return r.WithContext(WithMoraSession(r.Context(), sess))
}

func TestLoginSuccess(t *testing.T) {
	rm := NewMockRepositoryManager(12)
	path := "/" + strconv.FormatInt(rm.ID(), 10)
	rm.loginHandler = MockLoginMiddleware{"/"}.Handler
	handler := createTestLoginHandler(rm)

	// First request

	sess := NewMoraSession()
	req := NewGetRequestWithMoraSession(path, sess)
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)
	res := got.Result()

	require.Equal(t, http.StatusFound, res.StatusCode)

	loc, err := res.Location()
	require.NoError(t, err)
	t.Log("loc=", loc)

	// Second request

	req = NewGetRequestWithMoraSession(loc.String(), sess)
	got = httptest.NewRecorder()
	handler.ServeHTTP(got, req)
	res = got.Result()

	require.Equal(t, http.StatusOK, res.StatusCode)

	token, ok := sess.getToken(rm.ID())
	require.True(t, ok)
	assert.Equal(t, "MockAccessToken", token.Token)
}

func TestLoginSetsCSRFCookieSecure(t *testing.T) {
	tests := []struct {
		name           string
		insecureCookie bool
		wantSecure     bool
	}{
		{name: "default", wantSecure: true},
		{name: "insecure cookie", insecureCookie: true, wantSecure: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := NewMockRepositoryManager(12)
			path := "/" + strconv.FormatInt(rm.ID(), 10)
			rm.loginHandler = MockLoginMiddleware{"/"}.Handler
			handler := LoginHandler([]RepositoryManager{rm}, nil, tt.insecureCookie,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

			sess := NewMoraSession()
			req := NewGetRequestWithMoraSession(path, sess)
			got := httptest.NewRecorder()
			handler.ServeHTTP(got, req)
			res := got.Result()

			require.Equal(t, http.StatusFound, res.StatusCode)
			loc, err := res.Location()
			require.NoError(t, err)

			req = NewGetRequestWithMoraSession(loc.String(), sess)
			got = httptest.NewRecorder()
			handler.ServeHTTP(got, req)
			res = got.Result()

			var found bool
			for _, c := range res.Cookies() {
				if c.Name == csrfCookieName {
					found = true
					require.Equal(t, tt.wantSecure, c.Secure)
				}
			}
			require.True(t, found, "CSRF cookie should be set on OAuth login")
		})
	}
}

func TestLoginError(t *testing.T) {
	rm := NewMockRepositoryManager(1)
	r := createTestLoginHandler(rm)

	req := NewGetRequestWithMoraSession(fmt.Sprintf("/%d", rm.ID()), NewMoraSession())
	got := httptest.NewRecorder()
	r.ServeHTTP(got, req)
	res := got.Result()

	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestLoginErrorOnUnknownRepositoryManager(t *testing.T) {
	rm := NewMockRepositoryManager(1)
	r := createTestLoginHandler(rm)

	req := NewGetRequestWithMoraSession("/unknown_scm", NewMoraSession())
	got := httptest.NewRecorder()
	r.ServeHTTP(got, req)
	res := got.Result()

	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func testLogout(t *testing.T, logoutAll bool) {
	rm0 := NewMockRepositoryManager(0)
	rm1 := NewMockRepositoryManager(1)

	path := "/"
	if !logoutAll {
		path = "/" + strconv.FormatInt(rm0.ID(), 10)
	}
	w := httptest.NewRecorder()

	csrfToken := "test-csrf-token-for-testing"
	sess := NewMoraSession()
	sess.setToken(rm0.ID(), scm.Token{})
	sess.setToken(rm1.ID(), scm.Token{})
	body := strings.NewReader("csrf_token=" + csrfToken)
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		HttpOnly: false,
	})
	req = req.WithContext(WithMoraSession(req.Context(), sess))

	next := func(w http.ResponseWriter, r *http.Request) {}
	repositoryManagers := []RepositoryManager{rm0, rm1}
	r := LogoutHandler(repositoryManagers, http.HandlerFunc(next))

	r.ServeHTTP(w, req)

	result := w.Result()
	require.Equal(t, http.StatusOK, result.StatusCode)

	_, hasToken0 := sess.getToken(rm0.ID())
	_, hasToken1 := sess.getToken(rm1.ID())

	if logoutAll {
		assert.False(t, hasToken0)
		assert.False(t, hasToken1)
	} else {
		assert.False(t, hasToken0)
		assert.True(t, hasToken1)
	}
}

func TestLogoutHandlerAll(t *testing.T) {
	testLogout(t, true)
}

func TestLogoutHandlerOne(t *testing.T) {
	testLogout(t, false)
}

func TestLoginRedirect_NoLoggingInto(t *testing.T) {
	rm := NewMockRepositoryManager(1)
	rm.loginHandler = MockLoginMiddleware{"/"}.Handler
	handler := createTestLoginHandler(rm)

	sess := NewMoraSession()
	req := NewGetRequestWithMoraSession("/", sess)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestLoginRedirect_UnknownHandler(t *testing.T) {
	rm := NewMockRepositoryManager(1)
	rm.loginHandler = MockLoginMiddleware{"/"}.Handler
	handler := createTestLoginHandler(rm)

	sess := NewMoraSession()
	sess.loggingInto = 999
	req := NewGetRequestWithMoraSession("/", sess)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestLogout_InvalidScmID(t *testing.T) {
	rm := NewMockRepositoryManager(1)
	next := func(w http.ResponseWriter, r *http.Request) {}
	r := LogoutHandler([]RepositoryManager{rm}, http.HandlerFunc(next))

	csrfToken := "test-csrf-token"
	body := strings.NewReader("csrf_token=" + csrfToken)
	req := httptest.NewRequest(http.MethodPost, "/abc", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		HttpOnly: false,
	})
	sess := NewMoraSession()
	req = req.WithContext(WithMoraSession(req.Context(), sess))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	res := w.Result()
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestLogoutRedirectsToTopPage(t *testing.T) {
	rm := NewMockRepositoryManager(0)
	csrfToken := "test-csrf-token"
	sess := NewMoraSession()
	sess.setToken(rm.ID(), scm.Token{})

	body := strings.NewReader("csrf_token=" + csrfToken)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		HttpOnly: false,
	})
	req = req.WithContext(WithMoraSession(req.Context(), sess))

	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	handler := LogoutHandler([]RepositoryManager{rm}, redirectHandler)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()

	require.Equal(t, http.StatusSeeOther, res.StatusCode)
	loc, err := res.Location()
	require.NoError(t, err)
	assert.Equal(t, "/", loc.Path)
}

func TestLogout_NoSession(t *testing.T) {
	rm := NewMockRepositoryManager(1)
	next := func(w http.ResponseWriter, r *http.Request) {}
	r := LogoutHandler([]RepositoryManager{rm}, http.HandlerFunc(next))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	res := w.Result()
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}
