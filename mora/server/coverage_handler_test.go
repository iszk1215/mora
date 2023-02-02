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
	ok := assert.True(t, want.Timestamp.Equal(got.Timestamp))
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

func makeCoverageUploadRequest(repo Repository) (*CoverageUploadRequest, *Coverage) {
	revision := "12345"
	now := time.Now().Round(0)

	prof := profile.Profile{
		FileName: "test2.go",
		Hits:     0,
		Lines:    3,
		Blocks:   [][]int{{1, 3, 0}},
	}

	req := CoverageUploadRequest{
		RepoURL:   repo.Link,
		Revision:  revision,
		Timestamp: now,
		Entries: []*CoverageEntryUploadRequest{
			{
				Name:     "go",
				Hits:     0,
				Lines:    3,
				Profiles: []*profile.Profile{&prof},
			},
		},
	}

	want := Coverage{
		RepoID:    1215,
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

	repo := Repository{Link: "link"}

	want := Coverage{
		ID:        123,
		Revision:  "revision",
		Timestamp: time.Now().Round(0),
		Entries:   nil,
	}

	covStore := &MockCoverageStore{}

	p := NewMoraCoverageProvider(covStore)
	p.AddCoverage(&want)
	s := NewCoverageHandler(p, nil, covStore)

	r := chi.NewRouter()
	r.Route("/{index}", func(r chi.Router) {
		r.Use(s.injectCoverage)
		r.Get("/", handler)
	})

	req := httptest.NewRequest(http.MethodGet, "/123", strings.NewReader(""))
	req = req.WithContext(WithRepo(req.Context(), repo))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.Equal(t, &want, got)
}

func Test_injectCoverage_no_repo_in_context(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/0", strings.NewReader(""))
	w := httptest.NewRecorder()

	s := NewCoverageHandler(nil, nil, nil)
	s.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func Test_injectCoverage_malformed_index(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/foo", strings.NewReader(""))
	req = req.WithContext(WithRepo(req.Context(), Repository{Link: "link"}))
	w := httptest.NewRecorder()

	s := NewCoverageHandler(nil, nil, nil)
	s.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func Test_injectCoverage_no_repo_in_service(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/0", strings.NewReader(""))
	req = req.WithContext(WithRepo(req.Context(), Repository{Link: "link"}))
	w := httptest.NewRecorder()

	s := NewCoverageHandler(nil, nil, nil)
	s.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

/*
func TestParseCoverageUploadRequest(t *testing.T) {
	req, want := makeCoverageUploadRequest()
	got, err := parseCoverageUploadRequest(req)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
*/

func TestMakeCoverageResponseList(t *testing.T) {
	scm := NewMockSCM("scm")
	repo := Repository{Namespace: "owner", Name: "repo"} // FIXME

	cov := Coverage{
		RepoID:    1215,
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

func getResultFromCoverageListHandler(handler http.Handler, repo Repository) *http.Response {
	r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	scm := NewMockSCM("scm")
	r = r.WithContext(WithRepo(WithSCM(r.Context(), scm), repo))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w.Result()
}

// API Test

func Test_CoverageHandler_CoverageList(t *testing.T) {
	repo := Repository{ID: 1215, Namespace: "owner", Name: "repo", Link: "url"}

	covStore := MockCoverageStore{}

	p := NewMoraCoverageProvider(&covStore)

	time0 := time.Now().Round(0)
	time1 := time0.Add(-10 * time.Hour * 24)
	cov0 := Coverage{ID: 0, RepoID: repo.ID, Timestamp: time0, Revision: "abc123"}
	cov1 := Coverage{ID: 1, RepoID: repo.ID, Timestamp: time1, Revision: "abc124"}
	p.AddCoverage(&cov0)
	p.AddCoverage(&cov1)

	s := NewCoverageHandler(p, nil, &covStore)

	res := getResultFromCoverageListHandler(s.Handler(), repo)

	testCoverageListResponse(t, []Coverage{cov1, cov0}, res)
}

func Test_CoverageHandler_FileList(t *testing.T) {
	scm := NewMockSCM("mock")
	covStore := &MockCoverageStore{}
	p := NewMoraCoverageProvider(covStore)
	s := NewCoverageHandler(p, nil, covStore)

	repoURL := "http://mock.scm/org/name"
	filename := "test.go"
	revision := "abcde"
	timestamp := time.Now().Round(0)

	repo := Repository{Link: repoURL}

	cov := Coverage{
		ID:        123,
		Revision:  revision,
		Timestamp: timestamp,
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

	p.AddCoverage(&cov)

	sess := NewMoraSessionWithTokenFor(scm.Name())

	req := httptest.NewRequest(http.MethodGet, "/123/go/files", strings.NewReader(""))
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
		RevisionURL: repoURL + "/revision/" + revision, // MockSCM
		Time:        cov.Timestamp,
		Hits:        13,
		Lines:       17,
	}

	want := FileListResponse{
		Files:    []*FileResponse{&fileRes},
		Metadata: metaRes,
	}

	assert.Equal(t, want, got)
}

func Test_CoverageHandler_File(t *testing.T) {
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
	contents.EXPECT().Find( /*ctx*/ gomock.Any(), orgName+"/"+repoName, filename, revision).Return(&content, nil, nil)

	scm := NewMockSCM(scmName)
	scm.client.Contents = contents

	repo := Repository{ID: 1215, Namespace: orgName, Name: repoName, Link: repoURL}

	cov := Coverage{
		RepoID:    repo.ID,
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

	covStore := &MockCoverageStore{}
	p := NewMoraCoverageProvider(covStore)
	p.AddCoverage(&cov)

	s := NewCoverageHandler(p, nil, covStore)

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

func TestCoverageHandlerProcessUploadRequest(t *testing.T) {
	covStore := MockCoverageStore{}
	p := NewMoraCoverageProvider(&covStore)
	m := MockRepoStore{}
	repo := Repository{
		ID:        1215,
		Namespace: "mockowner",
		Name:      "mockrepo",
		Link:      "http://mock.scm/mockowner/mockrepo",
	}
	m.repos = []Repository{repo}
	s := NewCoverageHandler(p, m, &covStore)

	req, want := makeCoverageUploadRequest(repo)
	err := s.processUploadRequest(req)
	require.NoError(t, err)

	assert.Equal(t, []*Coverage{want}, covStore.coverages)
}
