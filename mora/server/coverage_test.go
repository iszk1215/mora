package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertEqualCoverage(t *testing.T, expected Coverage, got CoverageResponse) bool {
	ok := assert.True(t, expected.Time().Equal(got.Time))
	ok = ok && assert.Equal(t, expected.Revision(), got.Revision)

	ok = ok && assert.Equal(t, len(expected.Entries()), len(got.Entries))
	if len(expected.Entries()) != len(got.Entries) {
		return false
	}
	for i, a := range expected.Entries() {
		b := got.Entries[i]
		ok = ok && assert.Equal(t, a.Name, b.Name)
		ok = ok && assert.Equal(t, a.Lines, b.Lines)
		ok = ok && assert.Equal(t, a.Hits, b.Hits)
	}

	return ok
}

func assertEqualCoverageList(t *testing.T, expected []Coverage, got []CoverageResponse) bool {
	ok := assert.Equal(t, len(expected), len(got))
	if !ok {
		return false
	}

	for i := range expected {
		ok = ok && assertEqualCoverage(t, expected[i], got[i])
	}

	return ok
}

func testCoverageListResponse(t *testing.T, expected []Coverage, res *http.Response) {
	require.Equal(t, http.StatusOK, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var data []CoverageResponse
	err = json.Unmarshal(body, &data)
	require.NoError(t, err)

	assertEqualCoverageList(t, expected, data)
}

/*
type MockCoverage struct {
	url      string
	time     time.Time
	revision string
	entries  []CoverageEntry
}

func (c MockCoverage) RepoURL() string {
	return c.url
}

func (c MockCoverage) Time() time.Time {
	return c.time
}

func (c MockCoverage) Revision() string {
	return c.revision
}

func (c MockCoverage) Entries() []CoverageEntry {
	return c.entries
}
*/

type MockCoverageProvider struct {
	coverages map[string][]*Coverage
}

func NewMockCoverageProvider() *MockCoverageProvider {
	p := &MockCoverageProvider{}
	p.coverages = map[string][]*Coverage{}
	return p
}

func (p *MockCoverageProvider) HandleUploadRequest(req *CoverageUploadRequest) error {
	return nil
}

func (p *MockCoverageProvider) Coverages() []*Coverage {
	list := []*Coverage{}
	for _, c := range p.coverages {
		list = append(list, c...)
	}
	return list
}

func (p *MockCoverageProvider) AddCoverage(repo string, cov *Coverage) {
	p.coverages[repo] = append(p.coverages[repo], cov)
}

func (p *MockCoverageProvider) WebHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
}

func (p *MockCoverageProvider) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
}

func (p *MockCoverageProvider) Sync() error { return nil }

func (p *MockCoverageProvider) Repos() []string {
	repos := []string{}
	for k := range p.coverages {
		repos = append(repos, k)
	}
	return repos
}

func createMockCoverage() Coverage {
	cc := CoverageEntry{"cc", 100, 20, nil}
	py := CoverageEntry{"python", 300, 280, nil}
	cov := Coverage{time: time.Now(), revision: "abc123"}
	cov.entries = []*CoverageEntry{&cc, &py}

	return cov
}

func TestSerializeCoverage(t *testing.T) {
	scm := NewMockSCM("scm")
	repo := &Repo{Namespace: "owner", Name: "repo"} // FIXME
	cov := createMockCoverage()

	data := makeCoverageResponseList(scm, repo, []*Coverage{&cov})

	require.Equal(t, 1, len(data))
	assertEqualCoverage(t, cov, data[0])
}

func getResultFromCovrageListHandler(handler http.Handler, repo *Repo) *http.Response {
	r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	scm := NewMockSCM("scm")
	r = r.WithContext(WithRepo(WithSCM(r.Context(), scm), repo))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w.Result()
}

func TestCoverageList(t *testing.T) {
	repo := &Repo{Namespace: "owner", Name: "repo"}
	p := NewMockCoverageProvider()

	time0 := time.Now()
	time1 := time0.Add(-10 * time.Hour * 24)
	cov0 := Coverage{time: time0, revision: "abc123"}
	cov1 := Coverage{time: time1, revision: "abc123"}
	cov0.url = repo.Link
	cov1.url = repo.Link
	p.AddCoverage(repo.Link, &cov0)
	p.AddCoverage(repo.Link, &cov1)

	s := NewCoverageService()
	s.AddProvider(p)
	s.Sync()

	handler := http.HandlerFunc(s.handleCoverageList)
	res := getResultFromCovrageListHandler(handler, repo)

	testCoverageListResponse(t, []Coverage{cov1, cov0}, res)
}
