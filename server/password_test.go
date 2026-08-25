package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestFindByUsername(t *testing.T) {
	store := newTestUserStore(t)

	created, err := store.CreateUser("testuser", "https://av.com/1")
	require.NoError(t, err)

	found, err := store.FindByUsername("testuser")
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)
	require.Equal(t, "testuser", found.Username)
}

func TestFindByUsername_NotFound(t *testing.T) {
	store := newTestUserStore(t)

	_, err := store.FindByUsername("nonexistent")
	require.Error(t, err)
}

func TestCreateUserWithPassword(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUserWithPassword("passuser", "supersecret")
	require.NoError(t, err)
	require.NotZero(t, user.ID)
	require.Equal(t, "passuser", user.Username)

	passwordHash, err := store.GetPasswordHash(user.ID)
	require.NoError(t, err)
	require.NotNil(t, passwordHash)

	err = bcrypt.CompareHashAndPassword([]byte(*passwordHash), []byte("supersecret"))
	require.NoError(t, err)

	err = bcrypt.CompareHashAndPassword([]byte(*passwordHash), []byte("wrongpassword"))
	require.Error(t, err)
}

func TestCreateUserWithPassword_EmptyPassword(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUserWithPassword("emptypass", "")
	require.NoError(t, err)

	passwordHash, err := store.GetPasswordHash(user.ID)
	require.NoError(t, err)
	require.NotNil(t, passwordHash)

	err = bcrypt.CompareHashAndPassword([]byte(*passwordHash), []byte(""))
	require.NoError(t, err)
}

func TestSetPassword(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUser("setpassuser", "")
	require.NoError(t, err)

	hash, err := bcrypt.GenerateFromPassword([]byte("newpassword"), bcrypt.DefaultCost)
	require.NoError(t, err)

	err = store.SetPassword(user.ID, string(hash))
	require.NoError(t, err)

	passwordHash, err := store.GetPasswordHash(user.ID)
	require.NoError(t, err)
	require.NotNil(t, passwordHash)

	err = bcrypt.CompareHashAndPassword([]byte(*passwordHash), []byte("newpassword"))
	require.NoError(t, err)
}

func TestCreateUserWithPassword_NoSideEffectOnFindByUsername(t *testing.T) {
	store := newTestUserStore(t)

	_, err := store.CreateUserWithPassword("sideuser", "mypassword")
	require.NoError(t, err)

	found, err := store.FindByUsername("sideuser")
	require.NoError(t, err)
	require.Equal(t, "sideuser", found.Username)
	require.Empty(t, found.AvatarURL)

	passwordHash, err := store.GetPasswordHash(found.ID)
	require.NoError(t, err)
	require.NotNil(t, passwordHash)
}

// HTTP handler tests using httptest.ResponseRecorder with manual cookie tracking

type cookieJar struct {
	cookies []*http.Cookie
}

func newCookieJar() *cookieJar {
	return &cookieJar{}
}

func (j *cookieJar) addFromResponse(resp *http.Response) {
	newCookies := resp.Cookies()
	for _, nc := range newCookies {
		replaced := false
		for i, oc := range j.cookies {
			if oc.Name == nc.Name {
				j.cookies[i] = nc
				replaced = true
				break
			}
		}
		if !replaced {
			j.cookies = append(j.cookies, nc)
		}
	}
}

func (j *cookieJar) setOnRequest(req *http.Request) {
	for _, c := range j.cookies {
		req.AddCookie(c)
	}
}

func setupPasswordAuthHandler(t *testing.T) (*MoraSessionManager, UserStore, http.Handler) {
	store := newTestUserStore(t)
	sm := NewMoraSessionManager(false)
	t.Cleanup(func() { _ = sm.Close() })

	r := chi.NewRouter()
	r.Use(sm.SessionMiddleware)
	r.Mount("/api/auth", PasswordAuthHandler(store, false))

	return sm, store, r
}

func serveRequest(handler http.Handler, method, target string, body []byte, jar *cookieJar, headers map[string]string) *http.Response {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if jar != nil {
		jar.setOnRequest(req)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	resp := w.Result()
	if jar != nil {
		jar.addFromResponse(resp)
	}
	return resp
}

func getCSRFToken(t *testing.T, handler http.Handler, jar *cookieJar) string {
	resp := serveRequest(handler, "GET", "/api/auth/csrf", nil, jar, nil)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	token, ok := result["csrf_token"]
	require.True(t, ok)
	require.NotEmpty(t, token)

	return token
}

func csrfHeader(token string) map[string]string {
	return map[string]string{"X-CSRF-Token": token}
}

func TestPasswordLogin_Success(t *testing.T) {
	_, store, handler := setupPasswordAuthHandler(t)

	_, err := store.CreateUserWithPassword("logintest", "testpass123")
	require.NoError(t, err)

	jar := newCookieJar()
	csrf := getCSRFToken(t, handler, jar)

	body, _ := json.Marshal(map[string]string{
		"username": "logintest",
		"password": "testpass123",
	})
	resp := serveRequest(handler, "POST", "/api/auth/login", body, jar, csrfHeader(csrf))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var user User
	err = json.NewDecoder(resp.Body).Decode(&user)
	require.NoError(t, err)
	require.Equal(t, "logintest", user.Username)
	require.NotZero(t, user.ID)
}

func TestPasswordLogin_WrongPassword(t *testing.T) {
	_, store, handler := setupPasswordAuthHandler(t)

	_, err := store.CreateUserWithPassword("wrongpassuser", "correctpass")
	require.NoError(t, err)

	jar := newCookieJar()
	csrf := getCSRFToken(t, handler, jar)

	body, _ := json.Marshal(map[string]string{
		"username": "wrongpassuser",
		"password": "wrongpass",
	})
	resp := serveRequest(handler, "POST", "/api/auth/login", body, jar, csrfHeader(csrf))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestPasswordLogin_NonexistentUser(t *testing.T) {
	_, _, handler := setupPasswordAuthHandler(t)

	jar := newCookieJar()
	csrf := getCSRFToken(t, handler, jar)

	body, _ := json.Marshal(map[string]string{
		"username": "doesnotexist",
		"password": "somepass",
	})
	resp := serveRequest(handler, "POST", "/api/auth/login", body, jar, csrfHeader(csrf))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestPasswordLogin_NoCSRF(t *testing.T) {
	_, store, handler := setupPasswordAuthHandler(t)

	_, err := store.CreateUserWithPassword("nocsrfuser", "testpass123")
	require.NoError(t, err)

	resp := serveRequest(handler, "POST", "/api/auth/login",
		[]byte(`{"username":"nocsrfuser","password":"testpass123"}`), nil, nil)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestPasswordLogin_CSRFWithoutHeader(t *testing.T) {
	_, store, handler := setupPasswordAuthHandler(t)

	_, err := store.CreateUserWithPassword("nocsrftoken", "testpass123")
	require.NoError(t, err)

	jar := newCookieJar()
	// establish session but don't set CSRF header

	body, _ := json.Marshal(map[string]string{
		"username": "nocsrftoken",
		"password": "testpass123",
	})
	resp := serveRequest(handler, "POST", "/api/auth/login", body, jar, nil)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestPasswordLogin_UserWithoutPassword(t *testing.T) {
	_, store, handler := setupPasswordAuthHandler(t)

	_, err := store.CreateUser("nopassuser", "")
	require.NoError(t, err)

	jar := newCookieJar()
	csrf := getCSRFToken(t, handler, jar)

	body, _ := json.Marshal(map[string]string{
		"username": "nopassuser",
		"password": "somepassword",
	})
	resp := serveRequest(handler, "POST", "/api/auth/login", body, jar, csrfHeader(csrf))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestPasswordAuth_SetsSession(t *testing.T) {
	_, store, handler := setupPasswordAuthHandler(t)

	_, err := store.CreateUserWithPassword("sessionsuser", "testpass123")
	require.NoError(t, err)

	jar := newCookieJar()
	csrf := getCSRFToken(t, handler, jar)

	body, _ := json.Marshal(map[string]string{
		"username": "sessionsuser",
		"password": "testpass123",
	})
	resp := serveRequest(handler, "POST", "/api/auth/login", body, jar, csrfHeader(csrf))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// verify CSRF still works (session is maintained)
	csrf2 := getCSRFToken(t, handler, jar)
	require.NotEmpty(t, csrf2)
}

func TestPasswordLogin_InvalidJSON(t *testing.T) {
	_, _, handler := setupPasswordAuthHandler(t)

	jar := newCookieJar()
	csrf := getCSRFToken(t, handler, jar)

	resp := serveRequest(handler, "POST", "/api/auth/login", []byte(`{invalid}`), jar, csrfHeader(csrf))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestPasswordLogin_EmptyFields(t *testing.T) {
	_, _, handler := setupPasswordAuthHandler(t)

	jar := newCookieJar()
	csrf := getCSRFToken(t, handler, jar)

	tests := []struct {
		name     string
		username string
		password string
	}{
		{"empty username", "", "somepass123"},
		{"empty password", "someuser", ""},
		{"both empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{
				"username": tt.username,
				"password": tt.password,
			})
			resp := serveRequest(handler, "POST", "/api/auth/login", body, jar, csrfHeader(csrf))
			defer func() { _ = resp.Body.Close() }()
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}

func TestPasswordAuth_CSRFEndpointSetsCookie(t *testing.T) {
	_, _, handler := setupPasswordAuthHandler(t)

	jar := newCookieJar()
	csrf := getCSRFToken(t, handler, jar)
	require.NotEmpty(t, csrf)

	var found bool
	for _, c := range jar.cookies {
		if c.Name == csrfCookieName {
			found = true
			require.Equal(t, csrf, c.Value)
			require.Equal(t, "/", c.Path)
			require.False(t, c.HttpOnly)
			require.True(t, c.Secure,
				"Secure should be enabled by default on the CSRF cookie")
			break
		}
	}
	require.True(t, found, "CSRF cookie should be set by /api/auth/csrf endpoint")
}

func TestPasswordAuth_CSRFEndpointInsecureCookie(t *testing.T) {
	store := newTestUserStore(t)
	sm := NewMoraSessionManager(true)
	t.Cleanup(func() { _ = sm.Close() })

	r := chi.NewRouter()
	r.Use(sm.SessionMiddleware)
	r.Mount("/api/auth", PasswordAuthHandler(store, true))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == csrfCookieName {
			found = true
			require.False(t, c.Secure,
				"Secure attribute should be omitted with insecure_cookie over HTTP")
			break
		}
	}
	require.True(t, found)
}

func TestPasswordLogin_CorrectCSRF(t *testing.T) {
	_, store, handler := setupPasswordAuthHandler(t)

	_, err := store.CreateUserWithPassword("csrfuser", "testpass123")
	require.NoError(t, err)

	jar := newCookieJar()
	csrf := getCSRFToken(t, handler, jar)

	// verify the CSRF cookie matches what we send
	var csrfCookie string
	for _, c := range jar.cookies {
		if c.Name == "csrf_token" {
			csrfCookie = c.Value
			break
		}
	}
	require.Equal(t, csrf, csrfCookie)

	body, _ := json.Marshal(map[string]string{
		"username": "csrfuser",
		"password": "testpass123",
	})
	resp := serveRequest(handler, "POST", "/api/auth/login", body, jar, csrfHeader(csrf))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
