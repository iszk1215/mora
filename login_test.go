package mora

import (
	"net/http"
	"net/http/httptest"
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

func createTestLoginHandler(client Client) http.Handler {
	clients := []Client{client}
	next := func(w http.ResponseWriter, r *http.Request) {}
	return LoginHandler(clients, http.HandlerFunc(next))
}

func NewGetRequestWithMoraSession(path string, sess *MoraSession) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
	return r.WithContext(WithMoraSession(r.Context(), sess))
}

func TestLoginSuccess(t *testing.T) {
	mock := NewMockSCMClient("scm")
	path := "/" + mock.Name()
	mock.loginHandler = MockLoginMiddleware{path}.Handler
	r := createTestLoginHandler(mock)

	// First request

	req := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
	got := httptest.NewRecorder()
	r.ServeHTTP(got, req)
	res := got.Result()

	require.Equal(t, http.StatusFound, res.StatusCode)

	loc, err := res.Location()
	require.NoError(t, err)

	// Second request

	sess := NewMoraSession()
	req = NewGetRequestWithMoraSession(loc.String(), sess)
	got = httptest.NewRecorder()
	r.ServeHTTP(got, req)
	res = got.Result()

	require.Equal(t, http.StatusOK, res.StatusCode)

	token, ok := sess.getToken(mock.Name())
	require.True(t, ok)
	assert.Equal(t, "MockAccessToken", token.Token)
}

func TestLoginError(t *testing.T) {
	mock := NewMockSCMClient("scm")
	r := createTestLoginHandler(mock)

	req := NewGetRequestWithMoraSession("/"+mock.Name(), NewMoraSession())
	got := httptest.NewRecorder()
	r.ServeHTTP(got, req)
	res := got.Result()

	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestLoginErrorOnUnknownSCM(t *testing.T) {
	mock := NewMockSCMClient("scm")
	r := createTestLoginHandler(mock)

	req := NewGetRequestWithMoraSession("/unknown_scm", NewMoraSession())
	got := httptest.NewRecorder()
	r.ServeHTTP(got, req)
	res := got.Result()

	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func testLogout(t *testing.T, logoutAll bool) {
	client0 := NewMockSCMClient("client0")
	client1 := NewMockSCMClient("client1")

	path := "/"
	if !logoutAll {
		path = "/" + client0.Name()
	}
	got := httptest.NewRecorder()

	sess := NewMoraSession()
	sess.setToken(client0.Name(), scm.Token{})
	sess.setToken(client1.Name(), scm.Token{})
	req := NewGetRequestWithMoraSession(path, sess)

	next := func(w http.ResponseWriter, r *http.Request) {}
	clients := []Client{client0, client1}
	r := LogoutHandler(clients, http.HandlerFunc(next))

	r.ServeHTTP(got, req)

	_, hasToken0 := sess.getToken(client0.Name())
	_, hasToken1 := sess.getToken(client1.Name())

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
