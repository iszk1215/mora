package server

import (
	"encoding/json"
	"fmt"
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
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCoverageStore(t *testing.T, coverages ...*Coverage) CoverageStore {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	store := NewCoverageStore(db)
	err = store.Init()
	require.NoError(t, err)

	for _, cov := range coverages {
		err = store.Put(cov)
		require.NoError(t, err)
	}

	return store
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
		RepoURL:   repo.Url,
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
		RepoID:    repo.Id,
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

func Test_injectCoverage(t *testing.T) {
	var got *Coverage = nil

	handler := func(w http.ResponseWriter, r *http.Request) {
		cov, ok := CoverageFrom(r.Context())
		require.True(t, ok)
		got = cov
	}

	want := &Coverage{
		Revision:  "revision",
		RepoID:    1215,
		Timestamp: time.Now().Round(0),
		Entries:   []*CoverageEntry{},
	}

	covStore := setupCoverageStore(t, want)
	s := NewCoverageHandler(nil, covStore)

	r := chi.NewRouter()
	r.Route("/{id}", func(r chi.Router) {
		r.Use(s.injectCoverage)
		r.Get("/", handler)
	})

	req := httptest.NewRequest(
		http.MethodGet, fmt.Sprintf("/%d", want.ID), strings.NewReader(""))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.Equal(t, want, got)
}

func Test_injectCoverage_malformed_id(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/foo", strings.NewReader(""))
	req = req.WithContext(WithRepo(req.Context(), Repository{Url: "link"}))
	w := httptest.NewRecorder()

	s := NewCoverageHandler(nil, nil)
	s.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func TestMakeCoverageResponseList(t *testing.T) {
	scm := NewMockSCM(1)
	repo := Repository{
		Id:        1215,
		Namespace: "owner",
		Name:      "repo",
		Url:       fmt.Sprintf("%s/owner/repo", scm.URL()),
	}

	cov := &Coverage{
		RepoID:    repo.Id,
		Revision:  "abcde",
		Timestamp: time.Now().Round(0),
		Entries: []*CoverageEntry{
			{
				Name:  "cc",
				Hits:  20,
				Lines: 100,
				Profiles: map[string]*profile.Profile{
					"test.cc": {
						FileName: "test.cc",
						Hits:     13,
						Lines:    17,
						Blocks:   [][]int{{1, 5, 1}, {10, 13, 0}, {13, 20, 1}},
					},
				},
			},
			{
				Name:     "py",
				Hits:     280,
				Lines:    300,
				Profiles: nil,
			},
		},
	}

	want := CoverageListResponse{
		Repo: repo,
		Coverages: []CoverageResponse{
			{
				ID:          cov.ID,
				Timestamp:   cov.Timestamp,
				Revision:    cov.Revision,
				RevisionURL: scm.RevisionURL(repo.Url, cov.Revision),
				Entries: []*CoverageEntry{
					{
						Name:  "cc",
						Hits:  20,
						Lines: 100,
					},
					{
						Name:  "py",
						Hits:  280,
						Lines: 300,
					},
				},
			},
		},
	}

	got := makeCoverageListResponse(scm, repo, []*Coverage{cov})
	require.Equal(t, want, got)
}

// API Test

func Test_CoverageHandler_CoverageList(t *testing.T) {
	scm := NewMockSCM(1)
	repo := Repository{Id: 1215, Namespace: "owner", Name: "repo", Url: "url"}

	time0 := time.Now().Round(0)
	time1 := time0.Add(-10 * time.Hour * 24)
	cov0 := &Coverage{ID: 0, RepoID: repo.Id, Timestamp: time0, Revision: "abc123"}
	cov1 := &Coverage{ID: 1, RepoID: repo.Id, Timestamp: time1, Revision: "abc124"}

	covStore := setupCoverageStore(t, cov0, cov1)

	s := NewCoverageHandler(nil, covStore)

	r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	r = r.WithContext(WithRepo(WithSCM(r.Context(), scm), repo))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	res := w.Result()

	want := makeCoverageListResponse(scm, repo, []*Coverage{cov1, cov0})

	require.Equal(t, http.StatusOK, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var data CoverageListResponse
	err = json.Unmarshal(body, &data)
	require.NoError(t, err)

	require.Equal(t, want, data)
}

func Test_CoverageHandler_FileList(t *testing.T) {
	scm := NewMockSCM(1)

	filename := "test.go"
	revision := "abcde"
	timestamp := time.Now().Round(0)

	repo := Repository{
		Id:  1215,
		Url: "http://mock.scm/org/name",
	}

	cov := &Coverage{
		ID:        -1,
		RepoID:    repo.Id,
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

	covStore := setupCoverageStore(t, cov)
	s := NewCoverageHandler(nil, covStore)

	sess := NewMoraSessionWithTokenFor(scm)

	req := httptest.NewRequest(
		http.MethodGet, fmt.Sprintf("/%d/go/files", cov.ID), strings.NewReader(""))
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
		RevisionURL: scm.RevisionURL(repo.Url, cov.Revision),
		Time:        cov.Timestamp,
		Hits:        13,
		Lines:       17,
	}

	want := FileListResponse{
		Files:    []*FileResponse{&fileRes},
		Repo:     repo,
		Metadata: metaRes,
	}

	assert.Equal(t, want, got)
}

func Test_CoverageHandler_File(t *testing.T) {
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
	contents.EXPECT().
		Find( /*ctx*/ gomock.Any(), orgName+"/"+repoName, filename, revision).
		Return(&content, nil, nil)

	scm := NewMockSCM(1)
	scm.client.Contents = contents

	repo := Repository{Id: 1215, Namespace: orgName, Name: repoName, Url: repoURL}

	cov := &Coverage{
		RepoID:    repo.Id,
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

	covStore := setupCoverageStore(t, cov)
	s := NewCoverageHandler(nil, covStore)

	sess := NewMoraSessionWithTokenFor(scm)

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/%d/%s/files/%s", cov.ID, entryName, filename),
		strings.NewReader(""))
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
		Repo:     repo,
		FileName: prof.FileName,
		Code:     code,
		Blocks:   prof.Blocks,
	}

	assert.Equal(t, want, got)
}

func TestCoverageHandlerProcessUploadRequest(t *testing.T) {
	covStore := setupCoverageStore(t)
	repo := Repository{
		Namespace: "mockowner",
		Name:      "mockrepo",
		Url:       "http://mock.scm/mockowner/mockrepo",
	}
	repoStore := setupRepositoryStore(t, &repo)
	s := NewCoverageHandler(repoStore, covStore)

	req, want := makeCoverageUploadRequest(repo)
	err := s.processUploadRequest(req)
	require.NoError(t, err)

	got, err := covStore.ListAll()
	require.NoError(t, err)
	want.ID = 1
	assert.Equal(t, []*Coverage{want}, got)
}

func TestCoverageHandler_AddCoverage(t *testing.T) {
	cov := &Coverage{
		RepoID:    1215,
		Revision:  "012345",
		Timestamp: time.Now().Round(0),
		Entries: []*CoverageEntry{
			{
				Name:  "go",
				Hits:  13,
				Lines: 20,
				Profiles: map[string]*profile.Profile{
					"test.go": {
						FileName: "test.go",
						Hits:     13,
						Lines:    17,
						Blocks:   [][]int{{1, 5, 1}, {10, 13, 0}, {13, 20, 1}},
					},
				},
			},
		},
	}

	store := setupCoverageStore(t)
	handler := NewCoverageHandler(nil, store)
	err := handler.AddCoverage(cov)

	require.NoError(t, err)
	got, err := store.ListAll()
	require.NoError(t, err)
	assert.Equal(t, []*Coverage{cov}, got)
}

func TestCoverageHandler_AddCoverageMerge(t *testing.T) {
	existing := &Coverage{
		RepoID:    1215,
		Revision:  "012345",
		Timestamp: time.Now().Round(0),
		Entries: []*CoverageEntry{
			{
				Name:  "go",
				Hits:  13,
				Lines: 17,
				Profiles: map[string]*profile.Profile{
					"test.go": {
						FileName: "test.go",
						Hits:     13,
						Lines:    17,
						Blocks:   [][]int{{1, 5, 1}, {10, 13, 0}, {13, 20, 1}},
					},
				},
			},
		},
	}

	added := Coverage{
		RepoID:    1215,
		Revision:  "012345",
		Timestamp: time.Now(),
		Entries: []*CoverageEntry{
			{
				Name:  "cc",
				Hits:  0,
				Lines: 3,
				Profiles: map[string]*profile.Profile{
					"test.cc": {
						FileName: "test.cc",
						Hits:     0,
						Lines:    3,
						Blocks:   [][]int{{1, 3, 0}},
					},
				},
			},
		},
	}

	want := &Coverage{
		RepoID:    1215,
		Revision:  "012345",
		Timestamp: existing.Timestamp,
		Entries: []*CoverageEntry{
			// sorted by Name
			{
				Name:  "cc",
				Hits:  0,
				Lines: 3,
				Profiles: map[string]*profile.Profile{
					"test.cc": {
						FileName: "test.cc",
						Hits:     0,
						Lines:    3,
						Blocks:   [][]int{{1, 3, 0}},
					},
				},
			},
			{
				Name:  "go",
				Hits:  13,
				Lines: 17,
				Profiles: map[string]*profile.Profile{
					"test.go": {
						FileName: "test.go",
						Hits:     13,
						Lines:    17,
						Blocks:   [][]int{{1, 5, 1}, {10, 13, 0}, {13, 20, 1}},
					},
				},
			},
		},
	}

	store := setupCoverageStore(t, existing)

	handler := NewCoverageHandler(nil, store)
	err := handler.AddCoverage(&added)

	require.NoError(t, err)

	got, err := store.ListAll()
	require.NoError(t, err)
	want.ID = 1
	assert.Equal(t, []*Coverage{want}, got)
}
