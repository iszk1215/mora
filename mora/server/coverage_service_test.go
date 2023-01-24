package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/iszk1215/mora/mora/mockscm"
	"github.com/iszk1215/mora/mora/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertEqualCoverageAndResponse(t *testing.T, want Coverage, got CoverageResponse) bool {
	ok := assert.True(t, want.Timestamp.Equal(got.Time))
	ok = ok && assert.Equal(t, want.Revision, got.Revision)

	ok = ok && assert.Equal(t, len(want.Entries), len(got.Entries))
	if len(want.Entries) != len(got.Entries) {
		return false
	}
	for i, a := range want.Entries {
		b := got.Entries[i]
		ok = ok && assert.Equal(t, a.Name, b.Name)
		ok = ok && assert.Equal(t, a.Lines, b.Lines)
		ok = ok && assert.Equal(t, a.Hits, b.Hits)
	}

	return ok
}

func assertEqualCoverageList(t *testing.T, want []Coverage, got []CoverageResponse) bool {
	ok := assert.Equal(t, len(want), len(got))
	if !ok {
		return false
	}

	for i := range want {
		ok = ok && assertEqualCoverageAndResponse(t, want[i], got[i])
	}

	return ok
}

// Test Data

func makeCoverageUploadRequest() (*CoverageUploadRequest, *Coverage) {
	url := "http://mockscm.com/mockowner/mockrepo"
	revision := "12345"
	now := time.Now().Round(0)

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

	want := Coverage{
		URL:       url,
		Revision:  revision,
		Timestamp: now,
		Entries: []*CoverageEntry{
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

	return &req, &want
}

// Test Cases

func testCoverageListResponse(t *testing.T, want []Coverage, res *http.Response) {
	require.Equal(t, http.StatusOK, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var data []CoverageResponse
	err = json.Unmarshal(body, &data)
	require.NoError(t, err)

	assertEqualCoverageList(t, want, data)
}

func Test_injectCoverage(t *testing.T) {
	var got *Coverage = nil
	handler := func(w http.ResponseWriter, r *http.Request) {
		cov, ok := CoverageFrom(r.Context())
		require.True(t, ok)
		got = cov
	}

	repo := &Repo{Link: "link"}

	want := Coverage{
		URL:       repo.Link,
		Revision:  "revision",
		Timestamp: time.Now().Round(0),
		Entries:   nil,
	}

	s := NewCoverageService(nil)
	s.coverages = map[string][]*Coverage{
		repo.Link: {&want},
	}

	r := chi.NewRouter()
	r.Route("/{index}", func(r chi.Router) {
		r.Use(s.injectCoverage)
		r.Get("/", handler)
	})

	req := httptest.NewRequest(http.MethodGet, "/0", strings.NewReader(""))
	req = req.WithContext(WithRepo(req.Context(), repo))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.Equal(t, &want, got)
}

func Test_injectCoverage_no_repo_in_context(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/0", strings.NewReader(""))
	w := httptest.NewRecorder()

	s := NewCoverageService(nil)
	s.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func Test_injectCoverage_malformed_index(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/foo", strings.NewReader(""))
	req = req.WithContext(WithRepo(req.Context(), &Repo{Link: "link"}))
	w := httptest.NewRecorder()

	s := NewCoverageService(nil)
	s.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func Test_injectCoverage_no_repo_in_service(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/0", strings.NewReader(""))
	req = req.WithContext(WithRepo(req.Context(), &Repo{Link: "link"}))
	w := httptest.NewRecorder()

	s := NewCoverageService(nil)
	s.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func TestParseCoverageUploadRequest(t *testing.T) {
	req, want := makeCoverageUploadRequest()
	got, err := parseCoverageUploadRequest(req)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMakeCoverageResponseList(t *testing.T) {
	scm := NewMockSCM("scm")
	repo := &Repo{Namespace: "owner", Name: "repo"} // FIXME

	cov := Coverage{
		URL:       "dummyURL",
		Revision:  "abcde",
		Timestamp: time.Now().Round(0),
		Entries: []*CoverageEntry{
			{
				Name:     "cc",
				Hits:     20,
				Lines:    100,
				Profiles: nil,
			},
			{
				Name:     "py",
				Hits:     280,
				Lines:    300,
				Profiles: nil,
			},
		},
	}

	data := makeCoverageResponseList(scm, repo, []*Coverage{&cov})

	require.Equal(t, 1, len(data))
	assertEqualCoverageAndResponse(t, cov, data[0])
}

func getResultFromCoverageListHandler(handler http.Handler, repo *Repo) *http.Response {
	r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	scm := NewMockSCM("scm")
	r = r.WithContext(WithRepo(WithSCM(r.Context(), scm), repo))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w.Result()
}

// API Test

func Test_CoverageService_CoverageList(t *testing.T) {
	repo := &Repo{Namespace: "owner", Name: "repo", Link: "url"}
	p := NewMoraCoverageProvider(nil)

	time0 := time.Now().Round(0)
	time1 := time0.Add(-10 * time.Hour * 24)
	cov0 := Coverage{URL: repo.Link, Timestamp: time0, Revision: "abc123"}
	cov1 := Coverage{URL: repo.Link, Timestamp: time1, Revision: "abc124"}
	p.AddCoverage(&cov0)
	p.AddCoverage(&cov1)

	s := NewCoverageService(p)

	res := getResultFromCoverageListHandler(s.Handler(), repo)

	testCoverageListResponse(t, []Coverage{cov1, cov0}, res)
}

func Test_CoverageService_FileList(t *testing.T) {
	filename := "test.go"
	revision := "revision"

	scm := NewMockSCM("mock")
	repo := &Repo{Link: "link"}

	cov := Coverage{
		URL:       repo.Link,
		Revision:  revision,
		Timestamp: time.Now().Round(0),
		Entries: []*CoverageEntry{
			{
				Name:  "go",
				Hits:  13,
				Lines: 17,
				Profiles: map[string]*profile.Profile{
					filename: {
						FileName: filename,
						Hits:     13,
						Lines:    17,
						Blocks:   [][]int{{1, 5, 1}, {10, 13, 0}, {13, 20, 1}},
					},
				},
			},
		},
	}

	p := NewMoraCoverageProvider(nil)
	p.coverages = []*Coverage{&cov}

	s := NewCoverageService(p)

	sess := NewMoraSessionWithTokenFor(scm.Name())

	req := httptest.NewRequest(http.MethodGet, "/0/go/files", strings.NewReader(""))
	ctx := req.Context()
	ctx = WithMoraSession(ctx, sess)
	ctx = WithSCM(ctx, scm)
	ctx = WithRepo(ctx, repo)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	result := w.Result()
	require.Equal(t, http.StatusOK, result.StatusCode)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	var got FileListResponse
	err = json.Unmarshal(body, &got)
	require.NoError(t, err)

	fileRes := FileResponse{
		FileName: filename,
		Hits:     13,
		Lines:    17,
	}

	metaRes := MetaResonse{
		Revision:    revision,
		RevisionURL: repo.Link + "/revision/" + revision, // MockSCM
		Time:        cov.Timestamp,
		Hits:        13,
		Lines:       17,
	}

	want := FileListResponse{
		Files: []*FileResponse{&fileRes},
		Meta:  metaRes,
	}

	assert.Equal(t, want, got)
}

func Test_CoverageService_File(t *testing.T) {
	scmName := "mockscm"
	repoName := "repo"
	orgName := "org"
	repoURL := "link"
	entryName := "go"
	filename := "go/test.go"
	revision := "revision"
	code := "hello, world"

	prof := profile.Profile{
		FileName: filename,
		Hits:     13,
		Lines:    17,
		Blocks:   [][]int{{1, 5, 1}, {10, 13, 0}, {13, 20, 1}},
	}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	contents := mockscm.NewMockContentService(mockCtrl)
	content := scm.Content{Data: []byte(code)}
	contents.EXPECT().Find(gomock.Any(), orgName+"/"+repoName, filename, revision).Return(&content, nil, nil)

	scm := NewMockSCM(scmName)
	scm.client.Contents = contents

	repo := &Repo{Namespace: orgName, Name: repoName, Link: repoURL}

	cov := Coverage{
		URL:       repo.Link,
		Revision:  revision,
		Timestamp: time.Now().Round(0),
		Entries: []*CoverageEntry{
			{
				Name:  entryName,
				Hits:  13,
				Lines: 17,
				Profiles: map[string]*profile.Profile{
					prof.FileName: &prof,
				},
			},
		},
	}

	p := NewMoraCoverageProvider(nil)
	p.coverages = []*Coverage{&cov}

	s := NewCoverageService(p)

	sess := NewMoraSessionWithTokenFor(scm.Name())

	req := httptest.NewRequest(http.MethodGet, "/0/"+entryName+"/files/"+filename, strings.NewReader(""))
	ctx := req.Context()
	ctx = WithMoraSession(ctx, sess)
	ctx = WithSCM(ctx, scm)
	ctx = WithRepo(ctx, repo)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	result := w.Result()
	require.Equal(t, http.StatusOK, result.StatusCode)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	var got CodeResponse
	err = json.Unmarshal(body, &got)
	require.NoError(t, err)

	want := CodeResponse{
		FileName: prof.FileName,
		Code:     code,
		Blocks:   prof.Blocks,
	}

	assert.Equal(t, want, got)
}

func TestCoverageServiceProcessUploadRequest(t *testing.T) {
	p := NewMoraCoverageProvider(nil)
	s := NewCoverageService(p)

	req, want := makeCoverageUploadRequest()
	err := s.processUploadRequest(req)
	require.NoError(t, err)

	assert.Equal(t, []*Coverage{want}, p.Coverages())
}
