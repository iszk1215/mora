package mora

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/drone/drone/mock/mockscm"
	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireLocation(t *testing.T, expected string, r *http.Response) {
	loc, err := r.Location()
	require.NoError(t, err)
	require.Equal(t, expected, loc.String())
}

func assertEqualRepoResponse(t *testing.T, expected *Repo, got RepoResponse) bool {
	// FIXME: check SCM
	ok := assert.Equal(t, expected.Namespace, got.Namespace)
	ok = ok && assert.Equal(t, expected.Name, got.Name)
	ok = ok && assert.Equal(t, expected.Link, got.Link)
	return ok
}

func requireEqualRepoList(t *testing.T, expected []*Repo, res *http.Response) {
	body, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)

	var data []RepoResponse
	err = json.Unmarshal(body, &data)
	require.NoError(t, err)

	require.Equal(t, len(expected), len(data))
	for i, exp := range expected {
		assertEqualRepoResponse(t, exp, data[i])
	}
}

func Test_getReposWithCache(t *testing.T) {
	scm := NewMockSCM("mock")

	repo0 := &Repo{Namespace: "owner", Name: "repo0"}
	repo1 := &Repo{Namespace: "owner", Name: "repo1"}
	mockRepos := []*Repo{repo0, repo1}

	controller := gomock.NewController(t)
	defer controller.Finish()
	enableRepoService(controller, scm, mockRepos)

	sess := NewMoraSessionWithEmptyTokenFor(scm.Name())
	repos, err := getReposWithCache(scm, sess)

	require.NoError(t, err)
	require.Equal(t, mockRepos, repos)

	// from cache
	repos, ok := sess.getReposCache(scm.Name())
	require.True(t, ok)
	require.Equal(t, mockRepos, repos)
}

func getResultWithHandler(sess *MoraSession, scms []SCM, path string, handler http.Handler) *http.Response {

	got := httptest.NewRecorder()
	r := chi.NewRouter()
	r.Route("/{scm}/{owner}/{repo}", func(r chi.Router) {
		r.Use(injectRepo(scms))
		r.Get("/", handler.ServeHTTP)
	})

	req := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
	req = req.WithContext(WithMoraSession(req.Context(), sess))
	r.ServeHTTP(got, req)

	return got.Result()
}

func getResult(sess *MoraSession, scms []SCM, path string) *http.Response {
	null := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	return getResultWithHandler(sess, scms, path, null)
}

func newRepo(scm, namespace, name string) *Repo {
	return &Repo{Namespace: namespace, Name: name,
		Link: scm + "/" + namespace + "/" + name}
}

func enableRepoService(controller *gomock.Controller, mock *MockSCM, repos []*Repo) {
	mockRepoService := mockscm.NewMockRepositoryService(controller)
	mockRepoService.EXPECT().List(gomock.Any(), gomock.Any()).Return(repos, &scm.Response{}, nil)
	mock.client.Repositories = mockRepoService
}

func Test_injectRepo(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()
	scm := NewMockSCM("mock")
	repo := newRepo("http://mock.com", "owner", "repo")
	enableRepoService(controller, scm, []*Repo{repo})
	sess := NewMoraSessionWithEmptyTokenFor(scm.Name())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := RepoFrom(r.Context())
		require.True(t, ok)
		require.Equal(t, repo, got)
	})

	res := getResultWithHandler(sess, []SCM{scm}, "/mock/owner/repo", handler)
	require.Equal(t, http.StatusOK, res.StatusCode)
}

func Test_injectRepo_NoLogin(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()
	scm := NewMockSCM("mock")

	sess := NewMoraSession() // without token
	res := getResult(sess, []SCM{scm}, "/mock/owner/repo")

	require.Equal(t, http.StatusSeeOther, res.StatusCode)
	requireLocation(t, "/scms", res)
}

func test_injectRepo_Error(t *testing.T, path string, useRepo bool, expectedCode int) {
	controller := gomock.NewController(t)
	defer controller.Finish()
	scm := NewMockSCM("mock")
	if useRepo {
		enableRepoService(controller, scm, []*Repo{})
	}
	sess := NewMoraSessionWithEmptyTokenFor(scm.Name())

	res := getResult(sess, []SCM{scm}, path)
	require.Equal(t, expectedCode, res.StatusCode)
}

func Test_injectRepo_UnknownSCM(t *testing.T) {
	test_injectRepo_Error(t, "/err/owner/repo", false, http.StatusNotFound)
}

func TestRepoCheckerUnknownOwner(t *testing.T) {
	test_injectRepo_Error(t, "/mock/error/repo", true, http.StatusNotFound)
}

func TestRepoCheckerUnknownRepo(t *testing.T) {
	test_injectRepo_Error(t, "/mock/owner/unknown", true, http.StatusNotFound)
}

func TestSCMList(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()
	scm0 := NewMockSCM("mock0")
	scm1 := NewMockSCM("mock1")
	r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	sess := NewMoraSessionWithEmptyTokenFor(scm0.Name())

	r = r.WithContext(WithMoraSession(r.Context(), sess))
	got := httptest.NewRecorder()
	handler := HandleSCMList([]SCM{scm0, scm1})
	handler.ServeHTTP(got, r)
	res := got.Result()
	t.Log(res)
	body, _ := ioutil.ReadAll(res.Body)
	// t.Log(string(body))

	expected := `[{"url":"https://mock0.com","name":"mock0","logined":true},{"url":"https://mock1.com","name":"mock1","logined":false}]
`

	require.Equal(t, expected, string(body))
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

func setupServer(t *testing.T, controller *gomock.Controller, useRepo bool) (http.Handler, SCM, *Repo) {
	scm := NewMockSCM("scm")
	scm.loginHandler = MockLoginMiddleware{"/login/" + scm.Name()}.Handler

	repo := &Repo{Namespace: "owner", Name: "repo"}
	if useRepo {
		enableRepoService(controller, scm, []*Repo{repo})
	}

	provider := NewMockCoverageProvider()
	cov := NewMockCoverage()
	provider.AddCoverage(repo.Link, cov)

	coverage := NewCoverageService()
	coverage.AddProvider(provider)
	coverage.Sync()

	server, err := NewMoraServer([]SCM{scm}, false)
	server.coverage = coverage
	require.NoError(t, err)
	handler := server.Handler()

	return handler, scm, repo
}

func TestServerSCMList(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()
	handler, scm, _ := setupServer(t, controller, false)
	cookie := requireLogin(t, handler, scm.Name())

	req := httptest.NewRequest(http.MethodGet, "/api/scms", strings.NewReader(""))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()

	body, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	//t.Log(string(body))

	var scms []SCMResponse
	err = json.Unmarshal(body, &scms)
	require.NoError(t, err)
	//t.Log(scms)

	require.Equal(t, 1, len(scms))
	expected := SCMResponse{URL: scm.URL().String(), Name: scm.Name(), Logined: true}
	require.Equal(t, expected, scms[0])
}

func TestServerRepoList(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()
	handler, scm, repo := setupServer(t, controller, true)
	cookie := requireLogin(t, handler, scm.Name())

	req := httptest.NewRequest(http.MethodGet, "/api/repos", strings.NewReader(""))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()

	require.Equal(t, http.StatusOK, res.StatusCode)
	requireEqualRepoList(t, []*Repo{repo}, res)
}
