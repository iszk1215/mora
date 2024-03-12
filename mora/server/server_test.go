package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/iszk1215/mora/mora/core"
	"github.com/iszk1215/mora/mora/mockscm"
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

func requireEqualRepoList(t *testing.T, want []Repository, res *http.Response) {
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var got []Repository
	err = json.Unmarshal(body, &got)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func createMockRepoService(
	controller *gomock.Controller,
	repos ...Repository,
) scm.RepositoryService {
	mockRepoService := mockscm.NewMockRepositoryService(controller)
	mockRepoService.EXPECT().Find(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, repo string) (
			*scm.Repository, *scm.Response, error) {
			for _, r := range repos {
				if r.Namespace+"/"+r.Name == repo {
					ret := scm.Repository{
						Name:      r.Name,
						Namespace: r.Namespace,
						Link:      r.Url,
					}
					return &ret, &scm.Response{}, nil
				}
			}
			return nil, &scm.Response{}, fmt.Errorf("no repository found")
		}).AnyTimes()
	return mockRepoService
}

func Test_checkRepoAccess(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	rm := NewMockRepositoryManager(1)
	repo := Repository{Id: 3, Namespace: "owner", Name: "repo0"}
	rm.client.Repositories = createMockRepoService(controller, repo)
	sess := NewMoraSessionWithTokenFor(rm)

	cache := sess.getReposCache(rm.ID())
	require.Equal(t, 0, len(cache))

	err := checkRepoAccess(sess, rm, repo)
	require.NoError(t, err)

	// cache has the repo
	cache = sess.getReposCache(rm.ID())
	require.NotNil(t, cache)
	require.Equal(t, map[int64]bool{repo.Id: true}, cache)
}

func Test_checkRepoAccess_NoAccess(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	rm := NewMockRepositoryManager(1)
	repo0 := Repository{Id: 12, Namespace: "owner", Name: "repo0"}
	repo1 := Repository{Id: 13, Namespace: "owner", Name: "repo1"}
	rm.client.Repositories = createMockRepoService(controller, repo0)
	sess := NewMoraSessionWithTokenFor(rm)

	cache := sess.getReposCache(rm.ID())
	require.Equal(t, 0, len(cache))

	err := checkRepoAccess(sess, rm, repo1)
	require.Error(t, err)

	cache = sess.getReposCache(rm.ID())
	_, ok := cache[repo1.Id]
	require.False(t, ok)
}

type MoraServerBuilder struct {
	t      *testing.T
	Server *MoraServer
}

func NewMoraServerBuilder(t *testing.T) *MoraServerBuilder {
	return &MoraServerBuilder{t: t, Server: &MoraServer{}}
}

func (b *MoraServerBuilder) WithAPIKey(key string) *MoraServerBuilder {
	b.Server.apiKey = key
	return b
}

func (b *MoraServerBuilder) WithRepositoryManager(rm ...RepositoryManager) *MoraServerBuilder {
	b.Server.repositoryManagers = append(b.Server.repositoryManagers, rm...)
	return b
}

func (b *MoraServerBuilder) WithRepo(repos ...*Repository) *MoraServerBuilder {
	b.Server.repos = setupRepositoryStore(b.t, repos...)
	return b
}

func (b *MoraServerBuilder) WithSessionManager() *MoraServerBuilder {
	b.Server.sessionManager = NewMoraSessionManager()
	return b
}

func (b *MoraServerBuilder) Finish() *MoraServer {
	return b.Server
}

func Test_injectRepo(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	repo := Repository{
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "repo",
		Url:               "http://mock.com/owner/repo",
	}

	rm := NewMockRepositoryManager(1)
	rm.client.Repositories = createMockRepoService(controller, repo)

	server := NewMoraServerBuilder(t).WithRepositoryManager(rm).WithRepo(&repo).Finish()

	valid_path := fmt.Sprintf("/%d", repo.Id)

	callInjectRepo := func(req *http.Request) (int, Repository) {
		var repo Repository

		r := chi.NewRouter()
		r.Route("/{repo_id}", func(r chi.Router) {
			r.Use(server.injectRepo)
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				repo, _ = core.RepoFrom(r.Context())
			})
		})

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		return w.Result().StatusCode, repo
	}

	t.Run("login", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, valid_path, nil)
		sess := NewMoraSessionWithTokenFor(rm)
		req = req.WithContext(WithMoraSession(req.Context(), sess))

		status, got := callInjectRepo(req)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, repo, got)
	})

	t.Run("invalid repo id", func(t *testing.T) {
		path := fmt.Sprintf("/%d", repo.Id+1)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		sess := NewMoraSessionWithTokenFor(rm)
		req = req.WithContext(WithMoraSession(req.Context(), sess))

		status, _ := callInjectRepo(req)
		require.Equal(t, http.StatusBadRequest, status)
	})

	t.Run("nologin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, valid_path, nil)
		sess := NewMoraSession()
		req = req.WithContext(WithMoraSession(req.Context(), sess))

		status, _ := callInjectRepo(req)
		require.Equal(t, http.StatusForbidden, status)
	})

	t.Run("invalid path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		sess := NewMoraSessionWithTokenFor(rm)
		req = req.WithContext(WithMoraSession(req.Context(), sess))

		status, _ := callInjectRepo(req)
		require.Equal(t, http.StatusBadRequest, status)
	})

	t.Run("unsupported api key", func(t *testing.T) {
		sess := NewMoraSession()
		req := httptest.NewRequest(http.MethodGet, valid_path, nil)
		req = req.WithContext(WithMoraSession(req.Context(), sess))
		req.Header.Set("Authorization", "Bearer valid key")

		status, _ := callInjectRepo(req)
		require.Equal(t, http.StatusForbidden, status)
	})

	server.apiKey = "valid key"

	t.Run("api key", func(t *testing.T) {
		sess := NewMoraSession()
		req := httptest.NewRequest(http.MethodGet, valid_path, nil)
		req = req.WithContext(WithMoraSession(req.Context(), sess))
		req.Header.Set("Authorization", "Bearer valid key")

		status, got := callInjectRepo(req)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, repo, got)
	})
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

func TestServerRepositoryManagerList(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	rm := NewMockRepositoryManager(1)
	rm.id = 15
	rm.loginHandler = MockLoginMiddleware{"/login"}.Handler

	server := NewMoraServerBuilder(t).WithRepositoryManager(rm).WithSessionManager().Finish()
	handler := server.Handler()

	cookie := requireLogin(t, handler, rm.ID())

	req := httptest.NewRequest(http.MethodGet, "/api/scms", strings.NewReader(""))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var got []RepositoryManagerResponse
	err = json.Unmarshal(body, &got)
	require.NoError(t, err)

	expected := []RepositoryManagerResponse{
		{
			ID:      rm.ID(),
			URL:     rm.URL().String(),
			Logined: true,
		},
	}
	require.Equal(t, expected, got)
}

func TestServerRepoList(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	repo := Repository{
		RepositoryManager: 1215,
		Namespace:         "owner",
		Name:              "repo",
		Url:               "https://scm.com/owner/repo"}

	rm := NewMockRepositoryManager(1215)
	rm.loginHandler = MockLoginMiddleware{"/login"}.Handler
	rm.client.Repositories = createMockRepoService(controller, repo)

	key := "valid_api_key"

	server := NewMoraServerBuilder(t).WithRepositoryManager(rm).WithRepo(&repo).
		WithSessionManager().WithAPIKey(key).Finish()

	handler := server.Handler()

	url := "/api/repos"

	t.Run("repo list after login", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		cookie := requireLogin(t, handler, rm.ID())
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		res := w.Result()

		require.Equal(t, http.StatusOK, res.StatusCode)
		requireEqualRepoList(t, []Repository{repo}, res)
	})

	t.Run("repo list without login", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		res := w.Result()
		require.Equal(t, http.StatusOK, res.StatusCode)
		requireEqualRepoList(t, []Repository{}, res)
	})

	t.Run("repo list with api key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		res := w.Result()
		require.Equal(t, http.StatusOK, res.StatusCode)
		requireEqualRepoList(t, []Repository{repo}, res)
	})

	t.Run("repo list with invalid api key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer key")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		res := w.Result()
		require.Equal(t, http.StatusOK, res.StatusCode)
		requireEqualRepoList(t, []Repository{}, res)
	})
}

func TestServerRepoList2(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	repo0 := Repository{
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "repo0",
		Url:               "https://scm.com/owner/repo0"}

	repo1 := Repository{
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "repo1",
		Url:               "https://scm.com/owner/repo1"}

	rm := NewMockRepositoryManager(1)
	rm.loginHandler = MockLoginMiddleware{"/login"}.Handler
	rm.client.Repositories = createMockRepoService(controller, repo1)

	server := NewMoraServerBuilder(t).WithRepositoryManager(rm).WithRepo(&repo0, &repo1).
		WithSessionManager().Finish()
	handler := server.Handler()

	cookie := requireLogin(t, handler, rm.ID())

	req := httptest.NewRequest(http.MethodGet, "/api/repos", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()

	require.Equal(t, http.StatusOK, res.StatusCode)
	requireEqualRepoList(t, []Repository{repo1}, res)
}

func Test_NewMoraServerFromConfig_NoRepositoryManagerError(t *testing.T) {
	config := MoraConfig{}
	_, err := NewMoraServerFromConfig(config)
	require.Error(t, err)
}

func Test_NewMoraServerFromConfig_EmptySecret(t *testing.T) {
	config := MoraConfig{}
	config.RepositoryManagers = []RepositoryManagerConfig{
		{
			Driver: "github",
		},
	}
	_, err := NewMoraServerFromConfig(config)
	require.Error(t, err)
}

func Test_NewMoraServerFromConfig_Github(t *testing.T) {
	tmp, err := os.CreateTemp("", "github.conf")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())

	_, err = tmp.Write([]byte("ClientID = \"id\"\nClientSecret = \"secret\""))
	require.NoError(t, err)

	config := MoraConfig{}
	config.RepositoryManagers = []RepositoryManagerConfig{
		{
			Driver:         "github",
			SecretFilename: tmp.Name(),
		},
	}

	server, err := NewMoraServerFromConfig(config)
	require.NoError(t, err)

	// want, err := NewGithubFromFile(1, tmp.Name())
	// require.NoError(t, err)
	require.Equal(t, 1, len(server.repositoryManagers))

	got := server.repositoryManagers[0]
	assert.Equal(t, int64(1), got.ID())
	assert.Equal(t, "https://github.com", got.URL().String())
}

func Test_NewMoraServerFromConfig_Gitea(t *testing.T) {
	tmp, err := os.CreateTemp("", "gitea.conf")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())

	_, err = tmp.Write([]byte("ClientID = \"id\"\nClientSecret = \"secret\""))
	require.NoError(t, err)

	config := MoraConfig{}
	config.Server.URL = "http://localhost:4000"
	config.RepositoryManagers = []RepositoryManagerConfig{
		{
			Driver:         "gitea",
			URL:            "https://gitea.dayo/",
			SecretFilename: tmp.Name(),
		},
	}

	server, err := NewMoraServerFromConfig(config)
	require.NoError(t, err)

	_, err = NewGiteaFromFile(
		1, tmp.Name(), config.RepositoryManagers[0].URL, config.Server.URL+"/login")
	require.NoError(t, err)
	got := server.repositoryManagers[0]
	assert.Equal(t, int64(1), got.ID())
	assert.Equal(t, config.RepositoryManagers[0].URL, got.URL().String())
}
