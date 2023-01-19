package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iszk1215/mora/mora/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertEqualCoverageAndResponse(t *testing.T, expected Coverage, got CoverageResponse) bool {
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
		ok = ok && assertEqualCoverageAndResponse(t, expected[i], got[i])
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

func createMockCoverage() Coverage {
	cc := CoverageEntry{"cc", 100, 20, nil}
	py := CoverageEntry{"python", 300, 280, nil}
	cov := Coverage{time: time.Now(), revision: "abc123"}
	cov.entries = []*CoverageEntry{&cc, &py}

	return cov
}

func TestParseCoverageUploadRequest(t *testing.T) {
	url := "http://mockscm.com/mockowner/mockrepo"
	revision := "12345"
	now := time.Now()

	prof := profile.Profile{
		FileName: "test2.go",
		Hits:     0,
		Lines:    3,
		Blocks:   [][]int{{1, 3, 0}},
	}

	req := CoverageUploadRequest{
		RepoURL:  url,
		Revision: revision,
		Time:     now,
		Entries: []*CoverageEntryUploadRequest{
			{
				EntryName: "go",
				Hits:      0,
				Lines:     3,
				Profiles:  []*profile.Profile{&prof},
			},
		},
	}

	got, err := parseCoverageUploadRequest(&req)
	require.NoError(t, err)

	expected := Coverage{
		url:      url,
		revision: revision,
		time:     now,
		entries: []*CoverageEntry{
			{
				Name:  "go",
				Hits:  0,
				Lines: 3,
				Profiles: map[string]*profile.Profile{
					"test2.go": &prof,
				},
			},
		},
	}

	assertEqualCoverage(t, &expected, got)
}

func TestSerializeCoverage(t *testing.T) {
	scm := NewMockSCM("scm")
	repo := &Repo{Namespace: "owner", Name: "repo"} // FIXME
	cov := createMockCoverage()

	data := makeCoverageResponseList(scm, repo, []*Coverage{&cov})

	require.Equal(t, 1, len(data))
	assertEqualCoverageAndResponse(t, cov, data[0])
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
	p := NewMoraCoverageProvider(nil)

	time0 := time.Now()
	time1 := time0.Add(-10 * time.Hour * 24)
	cov0 := Coverage{url: "url", time: time0, revision: "abc123"}
	cov1 := Coverage{url: "url", time: time1, revision: "abc124"}
	cov0.url = repo.Link
	cov1.url = repo.Link
	p.AddCoverage(&cov0)
	p.AddCoverage(&cov1)
	//p.addCoverage(repo.Link, &cov0)
	//p.addCoverage(repo.Link, &cov1)

	s := NewCoverageService()
	s.AddProvider(p)
	s.Sync()

	handler := http.HandlerFunc(s.handleCoverageList)
	res := getResultFromCovrageListHandler(handler, repo)

	testCoverageListResponse(t, []Coverage{cov1, cov0}, res)
}
