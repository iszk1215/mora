package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/drone/go-scm/scm"
	"github.com/stretchr/testify/require"
)

func TestSessionManager(t *testing.T) {
	m := NewMoraSessionManager()
	next := func(w http.ResponseWriter, r *http.Request) {
		_, ok := MoraSessionFrom(r.Context())
		require.True(t, ok)
	}
	handler := m.SessionMiddleware(http.HandlerFunc(next))
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)
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

func TestMoraSessionConcurrentHTTPRace(t *testing.T) {
	t.Parallel()
	m := NewMoraSessionManager()
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
