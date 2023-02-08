package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/drone/go-login/login"
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

		token := login.Token{
			Access: "MockAccessToken",
		}

		ctx := login.WithToken(r.Context(), &token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func createTestLoginHandler(scm SCM) http.Handler {
	next := func(w http.ResponseWriter, r *http.Request) {}
	return LoginHandler([]SCM{scm}, http.HandlerFunc(next))
}

func NewGetRequestWithMoraSession(path string, sess *MoraSession) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
	return r.WithContext(WithMoraSession(r.Context(), sess))
}

func TestLoginSuccess(t *testing.T) {
	scm := NewMockSCM("scm")
	path := "/" + scm.Name()
	scm.loginHandler = MockLoginMiddleware{path}.Handler
	handler := createTestLoginHandler(scm)

	// First request

	req := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)
	res := got.Result()

	require.Equal(t, http.StatusFound, res.StatusCode)

	loc, err := res.Location()
	require.NoError(t, err)

	// Second request

	sess := NewMoraSession()
	req = NewGetRequestWithMoraSession(loc.String(), sess)
	got = httptest.NewRecorder()
	handler.ServeHTTP(got, req)
	res = got.Result()

	require.Equal(t, http.StatusOK, res.StatusCode)

	token, ok := sess.getToken(scm.ID())
	require.True(t, ok)
	assert.Equal(t, "MockAccessToken", token.Token)
}

func TestLoginError(t *testing.T) {
	scm := NewMockSCM("scm")
	r := createTestLoginHandler(scm)

	req := NewGetRequestWithMoraSession("/"+scm.Name(), NewMoraSession())
	got := httptest.NewRecorder()
	r.ServeHTTP(got, req)
	res := got.Result()

	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestLoginErrorOnUnknownSCM(t *testing.T) {
	scm := NewMockSCM("scm")
	r := createTestLoginHandler(scm)

	req := NewGetRequestWithMoraSession("/unknown_scm", NewMoraSession())
	got := httptest.NewRecorder()
	r.ServeHTTP(got, req)
	res := got.Result()

	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func testLogout(t *testing.T, logoutAll bool) {
	scm0 := NewMockSCMWithID(0, "scm0")
	scm1 := NewMockSCMWithID(1, "scm1")

	path := "/"
	if !logoutAll {
		path = "/" + strconv.FormatInt(scm0.ID(), 10)
	}
	w := httptest.NewRecorder()

	sess := NewMoraSession()
	sess.setToken(scm0.ID(), scm.Token{})
	sess.setToken(scm1.ID(), scm.Token{})
	req := NewGetRequestWithMoraSession(path, sess)

	next := func(w http.ResponseWriter, r *http.Request) {}
	scms := []SCM{scm0, scm1}
	r := LogoutHandler(scms, http.HandlerFunc(next))

	r.ServeHTTP(w, req)

	result := w.Result()
	require.Equal(t, http.StatusOK, result.StatusCode)

	_, hasToken0 := sess.getToken(scm0.ID())
	_, hasToken1 := sess.getToken(scm1.ID())

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
