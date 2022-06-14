package mora

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireLocation(t *testing.T, expected string, r *http.Response) {
	loc, err := r.Location()
	require.NoError(t, err)
	require.Equal(t, expected, loc.String())
}

func assertEqualRepoResponse(t *testing.T, expected Repo, got RepoResponse) bool {
	// FIXME: check SCM
	ok := assert.Equal(t, expected.Namespace, got.Namespace)
	ok = ok && assert.Equal(t, expected.Name, got.Name)
	ok = ok && assert.Equal(t, expected.Link, got.Link)
	return ok
}

func requireEqualRepoList(t *testing.T, expected []Repo, res *http.Response) {
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

func TestRepos(t *testing.T) {
	client := NewMockSCMClient("mock")
	// FIXME
	// repo0 := MockRepo{"mock", "owner", "repo0"}
	// repo1 := MockRepo{"mock", "owner", "repo1"}
	repo0 := &Repo{Namespace: "owner", Name: "repo0"}
	repo1 := &Repo{Namespace: "owner", Name: "repo1"}
	client.AddRepo(repo0, repo1)

	sess := NewMoraSessionWithEmptyTokenFor(client.Name())
	repos, err := getReposWithCache(client, sess)

	require.NoError(t, err)
	require.Equal(t, client.repos, repos)

	// from cache
	repos, ok := sess.getReposCache(client.Name())
	require.True(t, ok)
	require.Equal(t, client.repos, repos)
}

func getResultWithHandler(sess *MoraSession, clients []Client, path string, handler http.Handler) *http.Response {

	got := httptest.NewRecorder()
	r := chi.NewRouter()
	r.Route("/{scm}/{owner}/{repo}", func(r chi.Router) {
		r.Use(injectRepo(clients))
		r.Get("/", handler.ServeHTTP)
	})

	req := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
	req = req.WithContext(WithMoraSession(req.Context(), sess))
	r.ServeHTTP(got, req)

	return got.Result()
}

func getResult(sess *MoraSession, clients []Client, path string) *http.Response {
	null := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	return getResultWithHandler(sess, clients, path, null)
}

func newRepo(scm, namespace, name string) *Repo {
	return &Repo{Namespace: namespace, Name: name,
		Link: scm + "/" + namespace + "/" + name}

}

func createMockSCMClient(name string) (Client, *Repo) {
	client := NewMockSCMClient(name)
	repo := newRepo("https://"+name+".com", "owner", "repo")
	client.AddRepo(repo)
	return client, repo
}

func TestRepoCheckerSuccess(t *testing.T) {
	client, repo := createMockSCMClient("mock")
	sess := NewMoraSessionWithEmptyTokenFor(client.Name())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := RepoFrom(r.Context())
		require.True(t, ok)
		require.Equal(t, repo, got)
	})

	res := getResultWithHandler(sess, []Client{client}, "/mock/owner/repo", handler)
	require.Equal(t, http.StatusOK, res.StatusCode)
}

func TestRepoCheckerNoLogin(t *testing.T) {
	client, _ := createMockSCMClient("mock")

	sess := NewMoraSession() // without token
	res := getResult(sess, []Client{client}, "/mock/owner/repo")

	require.Equal(t, http.StatusSeeOther, res.StatusCode)
	requireLocation(t, "/scms", res)
}

func testRepoCheckerError(t *testing.T, path string, expectedCode int) {
	client, _ := createMockSCMClient("mock")
	sess := NewMoraSessionWithEmptyTokenFor(client.Name())

	res := getResult(sess, []Client{client}, path)
	require.Equal(t, expectedCode, res.StatusCode)
}

func TestRepoCheckerUnknownSCM(t *testing.T) {
	testRepoCheckerError(t, "/err/owner/repo", http.StatusNotFound)
}

func TestRepoCheckerUnknownOwner(t *testing.T) {
	testRepoCheckerError(t, "/mock/error/repo", http.StatusNotFound)
}

func TestRepoCheckerUnknownRepo(t *testing.T) {
	testRepoCheckerError(t, "/mock/owner/unknown", http.StatusNotFound)
}

func TestSCMList(t *testing.T) {
	client0, _ := createMockSCMClient("mock0")
	client1, _ := createMockSCMClient("mock1")
	r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	sess := NewMoraSessionWithEmptyTokenFor(client0.Name())

	r = r.WithContext(WithMoraSession(r.Context(), sess))
	got := httptest.NewRecorder()
	handler := HandleSCMList([]Client{client0, client1})
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

func setupServer(t *testing.T) (http.Handler, Client, Repo) {
	scm := NewMockSCMClient("scm")
	scm.loginHandler = MockLoginMiddleware{"/login/" + scm.Name()}.Handler
	repo := Repo{Namespace: "owner", Name: "repo"}
	scm.AddRepo(&repo)

	provider := NewMockCoverageProvider()
	cov := NewMockCoverage()
	provider.AddCoverage(repo.Link, cov)

	server, err := NewMoraServer([]Client{scm}, provider, false)
	require.NoError(t, err)
	handler := server.Handler()

	return handler, scm, repo
}

func TestServerSCMList(t *testing.T) {
	handler, scm, _ := setupServer(t)
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
	handler, scm, repo := setupServer(t)
	cookie := requireLogin(t, handler, scm.Name())

	req := httptest.NewRequest(http.MethodGet, "/api/repos", strings.NewReader(""))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	res := w.Result()

	require.Equal(t, http.StatusOK, res.StatusCode)
	requireEqualRepoList(t, []Repo{repo}, res)
}
