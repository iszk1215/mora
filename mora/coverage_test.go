package mora

import (
	"encoding/json"
	"io/ioutil"
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
		ok = ok && assert.Equal(t, a.Name(), b.Name)
		ok = ok && assert.Equal(t, a.Lines(), b.Lines)
		ok = ok && assert.Equal(t, a.Hits(), b.Hits)
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

	body, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)

	var data []CoverageResponse
	err = json.Unmarshal(body, &data)
	require.NoError(t, err)

	assertEqualCoverageList(t, expected, data)
}

type MockCoverageEntry struct {
	name  string
	lines int
	hits  int
}

func (e MockCoverageEntry) Name() string {
	return e.name
}

func (e MockCoverageEntry) Lines() int {
	return e.lines
}

func (e MockCoverageEntry) Hits() int {
	return e.hits
}

type MockCoverage struct {
	time     time.Time
	revision string
	entries  []MockCoverageEntry
}

func NewMockCoverage() *MockCoverage {
	return &MockCoverage{entries: []MockCoverageEntry{}}
}

func (c MockCoverage) Time() time.Time {
	return c.time
}

func (c MockCoverage) Revision() string {
	return c.revision
}

func (c MockCoverage) Entries() []CoverageEntry {
	ret := []CoverageEntry{}
	for _, e := range c.entries {
		ret = append(ret, e)
	}
	return ret
}

type MockCoverageProvider struct {
	coverages map[string][]Coverage
}

func NewMockCoverageProvider() *MockCoverageProvider {
	p := &MockCoverageProvider{}
	p.coverages = map[string][]Coverage{}
	return p
}

func (p *MockCoverageProvider) AddCoverage(repo string, cov Coverage) {
	p.coverages[repo] = append(p.coverages[repo], cov)
}

func (p *MockCoverageProvider) CoveragesFor(repo string) []Coverage {
	covs, ok := p.coverages[repo]
	if !ok {
		return []Coverage{}
	}
	return covs
}

func (p *MockCoverageProvider) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
}

func (p *MockCoverageProvider) HandleCoverage() http.Handler {
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

func createMockCoverage() MockCoverage {
	cc := MockCoverageEntry{"cc", 100, 20}
	py := MockCoverageEntry{"python", 300, 280}
	cov := MockCoverage{time: time.Now(), revision: "abc123"}
	cov.entries = []MockCoverageEntry{cc, py}

	return cov
}

func TestSerializeCoverage(t *testing.T) {
	// repo := MockRepo{"scm", "owner", "repo"}
	scm := NewMockSCMClient("scm")
	repo := &Repo{Namespace: "owner", Name: "repo"} // FIXME
	cov := createMockCoverage()

	data := convertCoverages(scm, repo, []Coverage{cov})

	require.Equal(t, 1, len(data))
	assertEqualCoverage(t, cov, data[0])
}

func getResultFromCovrageListHandler(handler http.Handler, repo *Repo) *http.Response {
	r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	scm := NewMockSCMClient("scm")
	r = r.WithContext(WithRepo(WithSCM(r.Context(), scm), repo))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w.Result()
}

func TestCoverageList(t *testing.T) {
	repo := &Repo{Namespace: "owner", Name: "repo"}
	p := NewMockCoverageProvider()
	expected := createMockCoverage()
	p.AddCoverage(repo.Link, expected)

	handler := handleCoverageList(p)
	res := getResultFromCovrageListHandler(handler, repo)

	testCoverageListResponse(t, []Coverage{expected}, res)
}

func TestCoverageListWithHTMLCoverageProvider(t *testing.T) {
	dir, repo, expected := createMockDataset(t)
	p := NewHTMLCoverageProvider(dir)
	err := p.Sync()
	require.NoError(t, err)

	handler := handleCoverageList(p)
	res := getResultFromCovrageListHandler(handler, repo)

	testCoverageListResponse(t, []Coverage{expected}, res)
}
