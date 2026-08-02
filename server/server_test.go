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
	"time"

	"github.com/drone/go-scm/scm"
	"github.com/drone/go-scm/scm/transport/oauth2"
	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/config"
	"github.com/iszk1215/mora/core"
	"github.com/iszk1215/mora/coverage"
	"github.com/iszk1215/mora/mockscm"
	"github.com/iszk1215/mora/tracker"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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
	defer func() { _ = res.Body.Close() }()
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
	b.Server.sessionManager = newTestSessionManager()
	return b
}

func (b *MoraServerBuilder) WithTracker(trackerService *tracker.Service) *MoraServerBuilder {
	b.Server.tracker = trackerService
	return b
}

func (b *MoraServerBuilder) WithCoverage(coverageService *coverage.CoverageService) *MoraServerBuilder {
	b.Server.coverage = coverageService
	return b
}

func (b *MoraServerBuilder) WithUserStore(u UserStore) *MoraServerBuilder {
	b.Server.userStore = u
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
		require.Equal(t, http.StatusNotFound, status)
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

	t.Run("rm not found", func(t *testing.T) {
		server2 := NewMoraServerBuilder(t).WithRepositoryManager(rm).WithRepo(&repo).Finish()

		strayRepo := Repository{
			RepositoryManager: 999,
			Namespace:         "stray",
			Name:              "stray",
			Url:               "http://mock.com/stray/stray",
		}
		err := server2.repos.Put(&strayRepo)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%d", strayRepo.Id), nil)
		sess := NewMoraSessionWithTokenFor(rm)
		req = req.WithContext(WithMoraSession(req.Context(), sess))

		r := chi.NewRouter()
		r.Route("/{repo_id}", func(r chi.Router) {
			r.Use(server2.injectRepo)
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {})
		})

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		status := w.Result().StatusCode
		require.Equal(t, http.StatusInternalServerError, status)
	})
}

func Test_initRepositoryManager_EmptyURL(t *testing.T) {
	cfg := config.RepositoryManagerConfig{
		Driver: "gitea",
		URL:    "",
	}
	_, err := initRepositoryManager(cfg, "http://localhost:4000", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "rm.url is empty")
}

func Test_initRepositoryManager_EmptySecretFile(t *testing.T) {
	cfg := config.RepositoryManagerConfig{
		Driver:         "gitea",
		URL:            "https://example.com",
		SecretFilename: "",
	}
	_, err := initRepositoryManager(cfg, "http://localhost:4000", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "rm.secret_file is empty")
}

func Test_initRepositoryManager_UnknownDriver(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)
	store := NewRepositoryManagerStore(db)
	err = store.Init()
	require.NoError(t, err)

	cfg := config.RepositoryManagerConfig{
		Driver:         "unknown",
		URL:            "https://example.com",
		SecretFilename: "/tmp/secret.conf",
	}
	_, err = initRepositoryManager(cfg, "http://localhost:4000", store)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown repository manager")
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

	req := httptest.NewRequest(http.MethodGet, "/api/providers", strings.NewReader(""))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var got []RepositoryManagerResponse
	err = json.Unmarshal(body, &got)
	require.NoError(t, err)

	expected := []RepositoryManagerResponse{
		{
			ID:       rm.ID(),
			URL:      rm.URL().String(),
			Name:     "mock",
			LoggedIn: true,
		},
	}
	require.Equal(t, expected, got)
}

func TestServerAPIConfig(t *testing.T) {
	rm := NewMockRepositoryManager(1)

	server := NewMoraServerBuilder(t).WithRepositoryManager(rm).WithSessionManager().Finish()
	server.siteName = "My Mora"
	handler := server.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var got ConfigResponse
	err = json.Unmarshal(body, &got)
	require.NoError(t, err)
	require.Equal(t, "My Mora", got.SiteName)
}

func TestServerAPIConfig_Default(t *testing.T) {
	rm := NewMockRepositoryManager(1)

	server := NewMoraServerBuilder(t).WithRepositoryManager(rm).WithSessionManager().Finish()
	handler := server.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var got ConfigResponse
	err = json.Unmarshal(body, &got)
	require.NoError(t, err)
	require.Equal(t, "Mora", got.SiteName)
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

func TestServerRepoList_RMNotFound(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	repo0 := Repository{
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "repo0",
		Url:               "https://scm.com/owner/repo0"}

	repo1 := Repository{
		RepositoryManager: 999,
		Namespace:         "owner",
		Name:              "repo1",
		Url:               "https://scm.com/owner/repo1"}

	rm := NewMockRepositoryManager(1)
	rm.loginHandler = MockLoginMiddleware{"/login"}.Handler
	rm.client.Repositories = createMockRepoService(controller, repo0)

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
	requireEqualRepoList(t, []Repository{repo0}, res)
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
	cfg := config.MoraConfig{}
	_, err := NewMoraServerFromConfig(cfg)
	require.Error(t, err)
}

func Test_NewMoraServerFromConfig_EmptySecret(t *testing.T) {
	cfg := config.MoraConfig{}
	cfg.RepositoryManagers = []config.RepositoryManagerConfig{
		{
			Driver: "github",
		},
	}
	_, err := NewMoraServerFromConfig(cfg)
	require.Error(t, err)
}

func Test_NewMoraServerFromConfig_Github(t *testing.T) {
	tmp, err := os.CreateTemp("", "github.conf")
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(tmp.Name())
	}()

	_, err = tmp.Write([]byte("ClientID = \"id\"\nClientSecret = \"secret\""))
	require.NoError(t, err)

	cfg := config.MoraConfig{}
	cfg.RepositoryManagers = []config.RepositoryManagerConfig{
		{
			Driver:         "github",
			SecretFilename: tmp.Name(),
		},
	}

	server, err := NewMoraServerFromConfig(cfg)
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
	defer func() {
		_ = os.Remove(tmp.Name())
	}()

	_, err = tmp.Write([]byte("ClientID = \"id\"\nClientSecret = \"secret\""))
	require.NoError(t, err)

	cfg := config.MoraConfig{}
	cfg.Server.URL = "http://localhost:4000"
	cfg.RepositoryManagers = []config.RepositoryManagerConfig{
		{
			Driver:         "gitea",
			URL:            "https://gitea.dayo/",
			SecretFilename: tmp.Name(),
		},
	}

	server, err := NewMoraServerFromConfig(cfg)
	require.NoError(t, err)

	_, err = NewGiteaFromFile(
		1, tmp.Name(), cfg.RepositoryManagers[0].URL, cfg.Server.URL+"/login", false)
	require.NoError(t, err)
	got := server.repositoryManagers[0]
	assert.Equal(t, int64(1), got.ID())
	assert.Equal(t, cfg.RepositoryManagers[0].URL, got.URL().String())
}

func TestNewGitea_InsecureSkipVerify(t *testing.T) {
	tests := []struct {
		name     string
		skip     bool
		expected bool
	}{
		{name: "enabled", skip: true, expected: true},
		{name: "disabled", skip: false, expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitea, err := NewGitea(1, "https://gitea.example.com", "id", "secret",
				"http://localhost:4000/login", tt.skip)
			require.NoError(t, err)

			oauthTransport, ok := gitea.Client().Client.Transport.(*oauth2.Transport)
			require.True(t, ok, "Transport should be *oauth2.Transport")

			baseTransport, ok := oauthTransport.Base.(*http.Transport)
			require.True(t, ok, "Base transport should be *http.Transport")

			require.NotNil(t, baseTransport.TLSClientConfig,
				"TLSClientConfig should not be nil")
			assert.Equal(t, tt.expected, baseTransport.TLSClientConfig.InsecureSkipVerify)
		})
	}
}

func TestNewMoraServerFromConfig_Gitea_InsecureSkipVerify(t *testing.T) {
	secretFile, err := os.CreateTemp("", "gitea-*.conf")
	require.NoError(t, err)
	defer func() { _ = os.Remove(secretFile.Name()) }()

	_, err = secretFile.Write([]byte("ClientID = \"id\"\nClientSecret = \"secret\""))
	require.NoError(t, err)

	tests := []struct {
		name   string
		toml   string
		want   bool
	}{
		{
			name: "enabled",
			toml: fmt.Sprintf(`
DatabaseFilename = ":memory:"

[server]
url = "http://localhost:4000"

[[scm]]
scm = "gitea"
url = "https://gitea.example.com"
secret_file = "%s"
insecure_skip_verify = true
`, secretFile.Name()),
			want: true,
		},
		{
			name: "disabled",
			toml: fmt.Sprintf(`
DatabaseFilename = ":memory:"

[server]
url = "http://localhost:4000"

[[scm]]
scm = "gitea"
url = "https://gitea.example.com"
secret_file = "%s"
`, secretFile.Name()),
			want: false,
		},
		{
			name: "explicitly_false",
			toml: fmt.Sprintf(`
DatabaseFilename = ":memory:"

[server]
url = "http://localhost:4000"

[[scm]]
scm = "gitea"
url = "https://gitea.example.com"
secret_file = "%s"
insecure_skip_verify = false
`, secretFile.Name()),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile, err := os.CreateTemp("", "mora-*.conf")
			require.NoError(t, err)
			defer func() { _ = os.Remove(configFile.Name()) }()

			_, err = configFile.Write([]byte(tt.toml))
			require.NoError(t, err)

			cfg, err := config.ReadMoraConfig(configFile.Name())
			require.NoError(t, err)
			require.Equal(t, tt.want, cfg.RepositoryManagers[0].InsecureSkipVerify)

			srv, err := NewMoraServerFromConfig(cfg)
			require.NoError(t, err)
			defer func() { _ = srv.Close() }()

			rm := srv.repositoryManagers[0]
			oauthTransport, ok := rm.Client().Client.Transport.(*oauth2.Transport)
			require.True(t, ok)

			baseTransport, ok := oauthTransport.Base.(*http.Transport)
			require.True(t, ok)
			require.NotNil(t, baseTransport.TLSClientConfig)
			assert.Equal(t, tt.want, baseTransport.TLSClientConfig.InsecureSkipVerify)
		})
	}
}

func TestHandleMe_Anonymous(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)
	userStore := NewUserStore(db)
	require.NoError(t, userStore.Init())

	server := &MoraServer{userStore: userStore}

	sess := NewMoraSession()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r = r.WithContext(WithMoraSession(r.Context(), sess))
	server.handleMe(w, r)

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusNoContent, res.StatusCode)
}

func TestHandleMe_NoSession(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)
	userStore := NewUserStore(db)
	require.NoError(t, userStore.Init())

	server := &MoraServer{userStore: userStore}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	server.handleMe(w, r)

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusNoContent, res.StatusCode)
}

func TestHandleMe_LoggedIn(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)
	userStore := NewUserStore(db)
	require.NoError(t, userStore.Init())

	user, err := userStore.CreateUser("testuser", "https://example.com/avatar.jpg")
	require.NoError(t, err)
	err = userStore.LinkAuth(user.ID, "github", "12345")
	require.NoError(t, err)

	server := &MoraServer{userStore: userStore}

	sess := NewMoraSession()
	sess.SetUserID(user.ID)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r = r.WithContext(WithMoraSession(r.Context(), sess))
	server.handleMe(w, r)

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)

	var got User
	err = json.NewDecoder(res.Body).Decode(&got)
	require.NoError(t, err)
	require.Equal(t, user.ID, got.ID)
	require.Equal(t, "testuser", got.Username)
	require.Equal(t, "https://example.com/avatar.jpg", got.AvatarURL)
}

func TestTrackerEndpointIsMounted(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	trackerService, err := tracker.NewService(db)
	require.NoError(t, err)

	server := NewMoraServerBuilder(t).
		WithSessionManager().
		WithTracker(trackerService).
		Finish()

	handler := server.Handler()

	t.Run("GET /api/trackers returns 200", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/trackers", nil)
		handler.ServeHTTP(w, r)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)

		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		var resp tracker.ListTrackersResponse
		err = json.Unmarshal(body, &resp)
		require.NoError(t, err)
		require.Empty(t, resp.Trackers)
	})

	t.Run("POST /api/trackers requires auth", func(t *testing.T) {
		body := `{"name":"integration_test","visibility":"private"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/trackers", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(w, r)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		// anonymous users cannot create trackers
		require.Equal(t, http.StatusForbidden, res.StatusCode)
	})
}

// ----------------------------------------------------------------------
// requireTrackerAuth

func TestRequireTrackerAuth_SessionLoggedIn(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	trackerService, err := tracker.NewService(db)
	require.NoError(t, err)

	server := NewMoraServerBuilder(t).
		WithSessionManager().
		WithTracker(trackerService).
		Finish()

	// Pre-populate a session in the store
	sess := NewMoraSession()
	sess.SetUserID(42)
	sid := "test-session-id"
	server.sessionManager.store[sid] = sess

	handler := server.Handler()

	body := `{"name":"test","visibility":"private"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/trackers", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(&http.Cookie{Name: "morasessionid", Value: sid})
	handler.ServeHTTP(w, r)

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	require.NotEqual(t, http.StatusForbidden, res.StatusCode)
}

func TestRequireTrackerAuth_APIKey(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	userStore := newTestUserStore(t)
	_, err = userStore.CreateUser("apiuser", "")
	require.NoError(t, err)

	key, err := userStore.CreateAPIKey(1, "test-key")
	require.NoError(t, err)

	trackerService, err := tracker.NewService(db)
	require.NoError(t, err)

	server := NewMoraServerBuilder(t).
		WithSessionManager().
		WithTracker(trackerService).
		WithUserStore(userStore).
		Finish()

	handler := server.Handler()

	body := `{"name":"api_test","visibility":"private"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/trackers", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+key)
	handler.ServeHTTP(w, r)

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	require.NotEqual(t, http.StatusForbidden, res.StatusCode)
}

func TestRequireTrackerAuth_AnonymousFallback(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	trackerService, err := tracker.NewService(db)
	require.NoError(t, err)

	server := NewMoraServerBuilder(t).
		WithSessionManager().
		WithTracker(trackerService).
		Finish()

	handler := server.Handler()

	body := `{"name":"anon_test","visibility":"private"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/trackers", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, r)

	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusForbidden, res.StatusCode)
}

// ----------------------------------------------------------------------
// handleMe

func TestHandleMe_UserNotFound(t *testing.T) {
	userStore := newTestUserStore(t)
	server := NewMoraServerBuilder(t).
		WithSessionManager().
		Finish()
	server.userStore = userStore

	sess := NewMoraSession()
	sess.SetUserID(999) // non-existing user

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r = r.WithContext(WithMoraSession(r.Context(), sess))
	server.handleMe(w, r)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusNoContent, res.StatusCode)
}

// ----------------------------------------------------------------------
// handleCoverageListPublic

func TestHandleCoverageListPublic(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	repo := Repository{
		Id:                1,
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "repo",
		Url:               "http://mock.scm/owner/repo",
	}

	rm := NewMockRepositoryManager(1)
	rm.client.Repositories = createMockRepoService(controller, repo)

	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	coverageService, err := coverage.NewCoverageService(db)
	require.NoError(t, err)

	cov := &coverage.Coverage{
		RepoID:    repo.Id,
		Revision:  "abc123",
		Timestamp: time.Now().Round(0),
	}
	_, err = coverageService.Store().Put(cov)
	require.NoError(t, err)

	trackerService, err := tracker.NewService(db)
	require.NoError(t, err)

	// Create a tracker linked to the repo
	repoID := repo.Id
	trk, err := coverageService.CreateCoverageTracker(trackerService, "test coverage", "", "public", 1, repoID)
	require.NoError(t, err)

	privateRepo := Repository{
		Id:                2,
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "private-repo",
		Url:               "http://mock.scm/owner/private-repo",
	}
	rm.client.Repositories = createMockRepoService(controller, repo, privateRepo)

	server := NewMoraServerBuilder(t).
		WithRepositoryManager(rm).
		WithRepo(&repo, &privateRepo).
		WithSessionManager().
		WithTracker(trackerService).
		WithCoverage(coverageService).
		Finish()

	handler := server.Handler()

	t.Run("logged in with valid token", func(t *testing.T) {
		sess := NewMoraSessionWithTokenFor(rm)
		sess.SetUserID(1)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/coverages/%d", trk.Id), nil)
		r.AddCookie(&http.Cookie{Name: "morasessionid", Value: "test-sess-logged-in"})
		server.sessionManager.store["test-sess-logged-in"] = sess
		handler.ServeHTTP(w, r)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)

		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		var data coverage.CoverageListResponse
		err = json.Unmarshal(body, &data)
		require.NoError(t, err)

		require.Len(t, data.Coverages, 1)
		assert.NotEmpty(t, data.Coverages[0].RevisionURL)
	})

	t.Run("not logged in", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/coverages/%d", trk.Id), nil)
		handler.ServeHTTP(w, r)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)

		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		var data coverage.CoverageListResponse
		err = json.Unmarshal(body, &data)
		require.NoError(t, err)

		require.Len(t, data.Coverages, 1)
		assert.Empty(t, data.Coverages[0].RevisionURL)
	})

	// Create a private tracker for access control tests
	_, err = coverageService.Store().Put(&coverage.Coverage{
		RepoID:    privateRepo.Id,
		Revision:  "def456",
		Timestamp: time.Now().Round(0),
	})
	require.NoError(t, err)
	privateTrk, err := coverageService.CreateCoverageTracker(trackerService, "private coverage", "", "private", 1, privateRepo.Id)
	require.NoError(t, err)

	t.Run("private tracker - anonymous returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/coverages/%d", privateTrk.Id), nil)
		handler.ServeHTTP(w, r)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("private tracker - superuser returns 200", func(t *testing.T) {
		sess := NewMoraSessionWithTokenFor(rm)
		sess.SetUserID(1)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/coverages/%d", privateTrk.Id), nil)
		r.AddCookie(&http.Cookie{Name: "morasessionid", Value: "test-sess-superuser"})
		server.sessionManager.store["test-sess-superuser"] = sess
		handler.ServeHTTP(w, r)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("private tracker - non-member returns 404", func(t *testing.T) {
		// Create a second user who is not a member of the private tracker
		userStore := NewUserStore(db)
		require.NoError(t, userStore.Init())
		user2, err := userStore.CreateUser("nonmember", "")
		require.NoError(t, err)

		sess := NewMoraSessionWithTokenFor(rm)
		sess.SetUserID(user2.ID)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/coverages/%d", privateTrk.Id), nil)
		r.AddCookie(&http.Cookie{Name: "morasessionid", Value: "test-sess-nonmember"})
		server.sessionManager.store["test-sess-nonmember"] = sess
		handler.ServeHTTP(w, r)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})
}

// ----------------------------------------------------------------------
// handleCreateCoverageTracker

func TestHandleCreateCoverageTracker(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	repo := Repository{
		Id:                1,
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "repo",
		Url:               "http://mock.scm/owner/repo",
	}

	rm := NewMockRepositoryManager(1)
	rm.client.Repositories = createMockRepoService(controller, repo)

	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	coverageService, err := coverage.NewCoverageService(db)
	require.NoError(t, err)

	trackerService, err := tracker.NewService(db)
	require.NoError(t, err)

	server := NewMoraServerBuilder(t).
		WithRepositoryManager(rm).
		WithRepo(&repo).
		WithSessionManager().
		WithTracker(trackerService).
		WithCoverage(coverageService).
		Finish()

	handler := server.Handler()

	authedRequest := func(body string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/coverages", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: "morasessionid", Value: "test-sess-create-coverage"})
		sess := NewMoraSessionWithTokenFor(rm)
		sess.SetUserID(1)
		server.sessionManager.store["test-sess-create-coverage"] = sess
		return r
	}

	t.Run("requires auth", func(t *testing.T) {
		body := `{"name":"cov","visibility":"public","repo_id":1}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/coverages", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(w, r)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusForbidden, res.StatusCode)
	})

	t.Run("validation errors", func(t *testing.T) {
		cases := []struct {
			name string
			body string
		}{
			{name: "missing name", body: `{"visibility":"public","repo_id":1}`},
			{name: "missing repo_id", body: `{"name":"cov","visibility":"public"}`},
			{name: "invalid visibility", body: `{"name":"cov","visibility":"secret","repo_id":1}`},
			{name: "malformed body", body: `not-json`},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, authedRequest(tc.body))
				res := w.Result()
				defer func() { _ = res.Body.Close() }()
				require.Equal(t, http.StatusBadRequest, res.StatusCode)
			})
		}
	})

	t.Run("repository not found", func(t *testing.T) {
		body := `{"name":"cov","visibility":"public","repo_id":999}`
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, authedRequest(body))
		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("creates and links coverage tracker", func(t *testing.T) {
		body := `{"name":"my cov tracker","visibility":"public","repo_id":1}`
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, authedRequest(body))
		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusCreated, res.StatusCode)

		raw, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		var trk tracker.TrackerModel
		require.NoError(t, json.Unmarshal(raw, &trk))
		require.Equal(t, "my cov tracker", trk.Name)
		require.Equal(t, "coverage", trk.Type)
		require.Equal(t, "public", trk.Visibility)

		repoID, err := coverageService.FindRepoIDByTrackerID(trk.Id)
		require.NoError(t, err)
		require.NotNil(t, repoID)
		require.Equal(t, int64(1), *repoID)
	})

	t.Run("already linked returns conflict", func(t *testing.T) {
		body := `{"name":"duplicate","visibility":"public","repo_id":1}`
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, authedRequest(body))
		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusConflict, res.StatusCode)
	})
}

// ----------------------------------------------------------------------
// handleCoveragePreview

func TestHandleCoveragePreview(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	repo := Repository{
		Id:                1,
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "repo",
		Url:               "http://mock.scm/owner/repo",
	}

	rm := NewMockRepositoryManager(1)
	rm.client.Repositories = createMockRepoService(controller, repo)

	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	coverageService, err := coverage.NewCoverageService(db)
	require.NoError(t, err)

	cov := &coverage.Coverage{
		RepoID:    repo.Id,
		Revision:  "abc123",
		Timestamp: time.Now().Round(0),
		Entries: []*coverage.CoverageEntry{
			{Name: "overall", Hits: 70, Lines: 100},
		},
	}
	_, err = coverageService.Store().Put(cov)
	require.NoError(t, err)

	trackerService, err := tracker.NewService(db)
	require.NoError(t, err)

	repoID := repo.Id
	trk, err := coverageService.CreateCoverageTracker(trackerService, "test coverage", "", "public", 1, repoID)
	require.NoError(t, err)

	server := NewMoraServerBuilder(t).
		WithRepositoryManager(rm).
		WithRepo(&repo).
		WithSessionManager().
		WithTracker(trackerService).
		WithCoverage(coverageService).
		Finish()

	handler := server.Handler()

	t.Run("public coverage tracker returns preview series", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/coverages/%d/preview", trk.Id), nil)
		handler.ServeHTTP(w, r)

		res := w.Result()
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)

		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		var data tracker.PreviewResponse
		err = json.Unmarshal(body, &data)
		require.NoError(t, err)

		require.Equal(t, "coverage", data.Tracker.Type)
		require.Len(t, data.Series, 1)
		require.Equal(t, "overall", data.Series[0].Series.Name)
		require.Len(t, data.Series[0].Values, 1)
		require.Equal(t, 70.0, data.Series[0].Values[0].Value)
	})
}
