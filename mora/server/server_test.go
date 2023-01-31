package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/drone/drone/mock/mockscm"
	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
func requireLocation(t *testing.T, expected string, r *http.Response) {
	loc, err := r.Location()
	require.NoError(t, err)
	require.Equal(t, expected, loc.String())
}
*/

func assertEqualRepoResponse(t *testing.T, expected Repository, got RepoResponse) bool {
	// mora uses these three members
	ok := assert.Equal(t, expected.Namespace, got.Namespace)
	ok = ok && assert.Equal(t, expected.Name, got.Name)
	ok = ok && assert.Equal(t, expected.Link, got.Link)
	return ok
}

func requireEqualRepoList(t *testing.T, expected []Repository, res *http.Response) {
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var data []RepoResponse
	err = json.Unmarshal(body, &data)
	require.NoError(t, err)

	require.Equal(t, len(expected), len(data))
	for i, exp := range expected {
		assertEqualRepoResponse(t, exp, data[i])
	}
}

func createMockRepoService(controller *gomock.Controller, repos []Repository) scm.RepositoryService {
	mockRepoService := mockscm.NewMockRepositoryService(controller)
	mockRepoService.EXPECT().Find(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, repo string) (
			*scm.Repository, *scm.Response, error) {
			for _, r := range repos {
				if r.Namespace+"/"+r.Name == repo {
					ret := scm.Repository{
						Name:      r.Name,
						Namespace: r.Namespace,
						Link:      r.Link,
					}
					return &ret, &scm.Response{}, nil
				}
			}
			return nil, &scm.Response{}, fmt.Errorf("no repository found")
		}).AnyTimes()
	return mockRepoService
}

func Test_checkRepoAccess(t *testing.T) {
	scm := NewMockSCM("mock")

	repo0 := Repository{Namespace: "owner", Name: "repo0"}
	mockRepos := []Repository{repo0}

	controller := gomock.NewController(t)
	defer controller.Finish()
	scm.client.Repositories = createMockRepoService(controller, mockRepos)

	sess := NewMoraSessionWithTokenFor(scm.Name())

	cache := sess.getReposCache(scm.Name())
	require.Equal(t, 0, len(cache))

	got, err := checkRepoAccess(sess, scm, "owner", "repo0")
	require.NoError(t, err)
	require.Equal(t, repo0, got)

	// cache has repo0
	cache = sess.getReposCache(scm.Name())
	require.NotNil(t, cache)
	require.Equal(t, map[string]Repository{"owner/repo0": repo0}, cache)
}

func Test_checkRepoAccess_NoAccess(t *testing.T) {
	scm := NewMockSCM("mock")

	repo0 := Repository{Namespace: "owner", Name: "repo0"}
	mockRepos := []Repository{repo0}

	controller := gomock.NewController(t)
	defer controller.Finish()
	scm.client.Repositories = createMockRepoService(controller, mockRepos)

	sess := NewMoraSessionWithTokenFor(scm.Name())

	cache := sess.getReposCache(scm.Name())
	require.Equal(t, 0, len(cache))

	repo, err := checkRepoAccess(sess, scm, "owner", "repo1")
	require.Error(t, err)
	require.Equal(t, Repository{}, repo)

	// cache has nil
	cache = sess.getReposCache(scm.Name())
	require.Nil(t, cache)
	//require.False(t, ok)
	// require.Equal(t, map[string]Repository{"owner/repo1": Repository{}}, cache)
}

func doInjectRepo(sess *MoraSession, server *MoraServer, path string, handler http.HandlerFunc) *http.Response {
	r := chi.NewRouter()
	r.Route("/{scm}/{owner}/{repo}", func(r chi.Router) {
		r.Use(server.injectRepo)
		r.Get("/", handler.ServeHTTP)
	})

	req := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
	req = req.WithContext(WithMoraSession(req.Context(), sess))
	got := httptest.NewRecorder()
	r.ServeHTTP(got, req)

	return got.Result()
}

func Test_injectRepo_OK(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	scm := NewMockSCM("mock")
	repo := Repository{
		ID:        1215,
		Namespace: "owner",
		Name:      "repo",
		Link:      "http://mock.com/owner/repo",
	}
	scm.client.Repositories = createMockRepoService(controller, []Repository{repo})
	sess := NewMoraSessionWithTokenFor(scm.Name())

	server, err := NewMoraServer([]SCM{scm}, false)
	require.NoError(t, err)
	server.repositories = []Repository{repo}

	called := false
	handler := func(w http.ResponseWriter, r *http.Request) {
		got, ok := RepoFrom(r.Context())
		require.True(t, ok)
		require.Equal(t, repo, got)
		called = true
	}

	res := doInjectRepo(sess, server, "/mock/owner/repo", handler)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.True(t, called)
}

func Test_injectRepo_NoLogin(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()
	scm := NewMockSCM("mock")

	server, err := NewMoraServer([]SCM{scm}, false)
	require.NoError(t, err)

	sess := NewMoraSession() // without token
	res := doInjectRepo(sess, server, "/mock/owner/repo", nil)

	require.Equal(t, http.StatusForbidden, res.StatusCode)
	//requireLocation(t, "/scms", res)
}

func test_injectRepo_Error(t *testing.T, path string, expectedCode int) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	scm := NewMockSCM("mock")
	scm.client.Repositories = createMockRepoService(controller, []Repository{})
	sess := NewMoraSessionWithTokenFor(scm.Name())

	server, err := NewMoraServer([]SCM{scm}, false)
	require.NoError(t, err)

	res := doInjectRepo(sess, server, path, nil)
	require.Equal(t, expectedCode, res.StatusCode)
}

func Test_injectRepo_UnknownSCM(t *testing.T) {
	test_injectRepo_Error(t, "/err/owner/repo", http.StatusNotFound)
}

func TestRepoCheckerUnknownOwner(t *testing.T) {
	test_injectRepo_Error(t, "/mock/error/repo", http.StatusNotFound)
}

func TestRepoCheckerUnknownRepo(t *testing.T) {
	test_injectRepo_Error(t, "/mock/owner/unknown", http.StatusNotFound)
}

// API Test with ServerHandler

func requireLogin(t *testing.T, handler http.Handler, scm string) *http.Cookie {
	// 1st request to get code
	req := httptest.NewRequest(http.MethodGet, "/login/"+scm, strings.NewReader(""))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()
	require.Equal(t, http.StatusFound, res.StatusCode)

	cookie := res.Cookies()[0]
	loc, err := res.Location()
	require.NoError(t, err)

	// 2nd request to complete login
	req = httptest.NewRequest(http.MethodGet, loc.String(), strings.NewReader(""))
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res = w.Result()
	require.Equal(t, http.StatusSeeOther, res.StatusCode)

	return cookie
}

func setupServer(scm SCM, repos []Repository) (*MoraServer, error) {
	provider := NewMoraCoverageProvider(nil)
	for _, repo := range repos {
		cov := Coverage{RepoID: repo.ID}
		provider.AddCoverage(&cov)
	}

	coverage := NewCoverageHandler(provider, nil)

	server, err := NewMoraServer([]SCM{scm}, false)
	log.Print(err)
	if err == nil {
		server.coverage = coverage
	}

	server.repositories = repos

	return server, err
}

func TestServerSCMList(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	scm := NewMockSCM("scm")
	scm.loginHandler = MockLoginMiddleware{"/login/" + scm.Name()}.Handler

	server, err := NewMoraServer([]SCM{scm}, false)
	require.NoError(t, err)
	handler := server.Handler()

	cookie := requireLogin(t, handler, scm.Name())

	req := httptest.NewRequest(http.MethodGet, "/api/scms", strings.NewReader(""))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var scms []SCMResponse
	err = json.Unmarshal(body, &scms)
	require.NoError(t, err)

	require.Equal(t, 1, len(scms))
	expected := SCMResponse{
		URL:     scm.URL().String(),
		Name:    scm.Name(),
		Logined: true}
	require.Equal(t, expected, scms[0])
}

func TestServerRepoList(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	repo := Repository{
		Namespace: "owner",
		Name:      "repo",
		Link:      "https://scm.com/owner/repo"}

	scm := NewMockSCM("scm")
	scm.loginHandler = MockLoginMiddleware{"/login/" + scm.Name()}.Handler
	scm.client.Repositories = createMockRepoService(controller, []Repository{repo})
	server, err := setupServer(scm, []Repository{repo})
	require.NoError(t, err)

	handler := server.Handler()

	cookie := requireLogin(t, handler, scm.Name())

	req := httptest.NewRequest(http.MethodGet, "/api/repos", strings.NewReader(""))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()

	require.Equal(t, http.StatusOK, res.StatusCode)
	requireEqualRepoList(t, []Repository{repo}, res)
}

func TestServerRepoList2(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	repo0 := Repository{
		ID:        1215,
		Namespace: "owner",
		Name:      "repo0",
		Link:      "https://scm.com/owner/repo0"}

	repo1 := Repository{
		ID:        1976,
		Namespace: "owner",
		Name:      "repo1",
		Link:      "https://scm.com/owner/repo1"}

	scm := NewMockSCM("scm")
	scm.loginHandler = MockLoginMiddleware{"/login/" + scm.Name()}.Handler
	scm.client.Repositories = createMockRepoService(controller, []Repository{repo1})

	server, err := setupServer(scm, []Repository{repo0, repo1})
	require.NoError(t, err)
	handler := server.Handler()

	cookie := requireLogin(t, handler, scm.Name())

	req := httptest.NewRequest(http.MethodGet, "/api/repos", strings.NewReader(""))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()

	require.Equal(t, http.StatusOK, res.StatusCode)
	requireEqualRepoList(t, []Repository{repo1}, res)
}
