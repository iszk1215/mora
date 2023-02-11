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
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRepositoryStore(t *testing.T, repos ...*Repository) RepositoryStore {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	store := NewRepositoryStore(db)
	err = store.Init()
	require.NoError(t, err)

	for _, repo := range repos {
		err = store.Put(repo)
		require.NoError(t, err)
	}

	return store
}

func assertEqualRepoResponse(t *testing.T, expected Repository, got RepoResponse) bool {
	ok := assert.Equal(t, expected.ID, got.ID)
	ok = ok && assert.Equal(t, expected.Namespace, got.Namespace)
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
	scm := NewMockSCM(1)

	repo0 := Repository{ID: 3, Namespace: "owner", Name: "repo0"}
	mockRepos := []Repository{repo0}

	controller := gomock.NewController(t)
	defer controller.Finish()
	scm.client.Repositories = createMockRepoService(controller, mockRepos)

	sess := NewMoraSessionWithTokenFor(scm)

	cache := sess.getReposCache(scm.ID())
	require.Equal(t, 0, len(cache))

	err := checkRepoAccess(sess, scm, repo0)
	require.NoError(t, err)

	// cache has repo0
	cache = sess.getReposCache(scm.ID())
	require.NotNil(t, cache)
	require.Equal(t, map[int64]bool{repo0.ID: true}, cache)
}

func Test_checkRepoAccess_NoAccess(t *testing.T) {
	scm := NewMockSCM(1)

	repo0 := Repository{ID: 12, Namespace: "owner", Name: "repo0"}
	repo1 := Repository{ID: 13, Namespace: "owner", Name: "repo1"}
	mockRepos := []Repository{repo0}

	controller := gomock.NewController(t)
	defer controller.Finish()
	scm.client.Repositories = createMockRepoService(controller, mockRepos)

	sess := NewMoraSessionWithTokenFor(scm)

	cache := sess.getReposCache(scm.ID())
	require.Equal(t, 0, len(cache))

	err := checkRepoAccess(sess, scm, repo1)
	require.Error(t, err)

	// cache has nil
	cache = sess.getReposCache(scm.ID())
	require.Nil(t, cache)
	//require.False(t, ok)
	// require.Equal(t, map[string]Repository{"owner/repo1": Repository{}}, cache)
}

func setupMoraServer(t *testing.T, scms []SCM) *MoraServer {
	s := &MoraServer{}
	s.scms = scms
	return s
}

func doInjectRepo(sess *MoraSession, server *MoraServer, path string, handler http.HandlerFunc) *http.Response {
	r := chi.NewRouter()
	r.Route("/{repo_id}", func(r chi.Router) {
		r.Use(server.injectRepoByID)
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

	repo := Repository{
		SCM:       1,
		Namespace: "owner",
		Name:      "repo",
		Link:      "http://mock.com/owner/repo",
	}

	scm := NewMockSCM(1)
	scm.client.Repositories = createMockRepoService(controller, []Repository{repo})
	sess := NewMoraSessionWithTokenFor(scm)

	server := setupMoraServer(t, []SCM{scm})
	server.repos = setupRepositoryStore(t, &repo)

	called := false
	handler := func(w http.ResponseWriter, r *http.Request) {
		got, ok := RepoFrom(r.Context())
		require.True(t, ok)
		require.Equal(t, repo, got)
		called = true
	}

	res := doInjectRepo(sess, server, fmt.Sprintf("/%d", repo.ID), handler)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.True(t, called)
}

func Test_injectRepo_NoLogin(t *testing.T) {
	scm := NewMockSCM(1)

	repo := Repository{
		SCM:       1,
		Namespace: "owner",
		Name:      "repo",
		Link:      "http://mock.com/owner/repo",
	}
	server := setupMoraServer(t, []SCM{scm})
	server.repos = setupRepositoryStore(t, &repo)

	sess := NewMoraSession() // without token
	res := doInjectRepo(sess, server, fmt.Sprintf("/%d", repo.ID), nil)

	require.Equal(t, http.StatusForbidden, res.StatusCode)
}

func test_injectRepo_Error(t *testing.T, path string, expectedCode int) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	scm := NewMockSCM(1)
	scm.client.Repositories = createMockRepoService(controller, []Repository{})
	sess := NewMoraSessionWithTokenFor(scm)

	server := setupMoraServer(t, []SCM{scm})

	res := doInjectRepo(sess, server, path, nil)
	require.Equal(t, expectedCode, res.StatusCode)
}

func Test_injectRepo_InvalidRepoID(t *testing.T) {
	test_injectRepo_Error(t, "/abc", http.StatusNotFound)
}

func TestRepoCheckerUnknownOwner(t *testing.T) {
	test_injectRepo_Error(t, "/mock/error/repo", http.StatusNotFound)
}

func TestRepoCheckerUnknownRepo(t *testing.T) {
	test_injectRepo_Error(t, "/mock/owner/unknown", http.StatusNotFound)
}

// API Test with ServerHandler

func requireLogin(t *testing.T, handler http.Handler, scmID int64) *http.Cookie {
	// 1st request to get code
	path := fmt.Sprintf("/login/%d", scmID)
	req := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
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

func setupServer(t *testing.T, scm SCM, repos []*Repository) *MoraServer {
	covStore := setupCoverageStore(t)
	for _, repo := range repos {
		covStore.Put(&Coverage{RepoID: repo.ID})
	}

	repoStore := setupRepositoryStore(t, repos...)

	coverage := NewCoverageHandler(repoStore, covStore)

	server := &MoraServer{}
	server.sessionManager = NewMoraSessionManager()
	server.scms = []SCM{scm}
	server.coverage = coverage
	server.repos = repoStore

	return server
}

func TestServerSCMList(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	scm := NewMockSCM(1)
	scm.id = 15
	scm.loginHandler = MockLoginMiddleware{"/login"}.Handler

	server := setupMoraServer(t, []SCM{scm})
	server.sessionManager = NewMoraSessionManager()
	handler := server.Handler()

	cookie := requireLogin(t, handler, scm.ID())

	req := httptest.NewRequest(http.MethodGet, "/api/scms", strings.NewReader(""))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var got []SCMResponse
	err = json.Unmarshal(body, &got)
	require.NoError(t, err)

	expected := []SCMResponse{
		{
			ID:      scm.ID(),
			URL:     scm.URL().String(),
			Logined: true,
		},
	}
	require.Equal(t, expected, got)
}

func TestServerRepoList(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	repo := Repository{
		SCM:       1215,
		Namespace: "owner",
		Name:      "repo",
		Link:      "https://scm.com/owner/repo"}

	scm := NewMockSCM(1215)
	scm.loginHandler = MockLoginMiddleware{"/login"}.Handler
	scm.client.Repositories = createMockRepoService(controller, []Repository{repo})

	server := setupServer(t, scm, []*Repository{&repo})

	handler := server.Handler()

	cookie := requireLogin(t, handler, scm.ID())

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
		SCM:       1,
		Namespace: "owner",
		Name:      "repo0",
		Link:      "https://scm.com/owner/repo0"}

	repo1 := Repository{
		SCM:       1,
		Namespace: "owner",
		Name:      "repo1",
		Link:      "https://scm.com/owner/repo1"}

	scm := NewMockSCM(1)
	scm.loginHandler = MockLoginMiddleware{"/login"}.Handler
	scm.client.Repositories = createMockRepoService(controller, []Repository{repo1})

	server := setupServer(t, scm, []*Repository{&repo0, &repo1})
	handler := server.Handler()

	cookie := requireLogin(t, handler, scm.ID())

	req := httptest.NewRequest(http.MethodGet, "/api/repos", strings.NewReader(""))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()

	require.Equal(t, http.StatusOK, res.StatusCode)
	requireEqualRepoList(t, []Repository{repo1}, res)
}
