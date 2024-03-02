package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/iszk1215/mora/mora/base"
	"github.com/iszk1215/mora/mora/mockscm"
	"github.com/iszk1215/mora/mora/profile"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockRepositoryClient struct {
	url    *url.URL
	client *scm.Client
}

func (m *MockRepositoryClient) Client() *scm.Client {
	return m.client
}

func NewMockRepositoryClient() *MockRepositoryClient {
	m := &MockRepositoryClient{}
	m.url, _ = url.Parse(strings.Join([]string{"https://mock.scm"}, ""))
	m.client = &scm.Client{}
	return m
}

func (m *MockRepositoryClient) RevisionURL(baseURL string, revision string) string {
	joined, _ := url.JoinPath(baseURL, "revision", revision)
	return joined
}

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

/*
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
*/

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

	store := setupCoverageStore(t, want)
	s := newCoverageHandler(store)

	r := chi.NewRouter()
	r.Route("/{id}", func(r chi.Router) {
		r.Use(s.injectCoverage)
		r.Get("/", handler)
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%d", want.ID), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.Equal(t, want, got)
}

func Test_injectCoverage_malformed_id(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	req = req.WithContext(base.WithRepo(req.Context(), Repository{Url: "link"}))
	w := httptest.NewRecorder()

	s := newCoverageHandler(nil)
	s.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func TestMakeCoverageResponseList(t *testing.T) {
	rm := NewMockRepositoryClient()
	repo := Repository{
		Id:        1215,
		Namespace: "owner",
		Name:      "repo",
		Url:       "repoUrl",
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
				RevisionURL: rm.RevisionURL(repo.Url, cov.Revision),
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

	got := makeCoverageListResponse(rm, repo, []*Coverage{cov})
	require.Equal(t, want, got)
}

// API Test

func Test_CoverageHandler_CoverageList(t *testing.T) {
	rm := NewMockRepositoryClient()
	repo := Repository{Id: 1215, Namespace: "owner", Name: "repo", Url: "url"}

	time0 := time.Now().Round(0)
	time1 := time0.Add(-10 * time.Hour * 24)
	cov0 := &Coverage{ID: 0, RepoID: repo.Id, Timestamp: time0, Revision: "abc123"}
	cov1 := &Coverage{ID: 1, RepoID: repo.Id, Timestamp: time1, Revision: "abc124"}

	store := setupCoverageStore(t, cov0, cov1)
	s := newCoverageHandler(store)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(base.WithRepo(base.WithRepositoryClient(r.Context(), rm), repo))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	res := w.Result()

	want := makeCoverageListResponse(rm, repo, []*Coverage{cov1, cov0})

	require.Equal(t, http.StatusOK, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	var data CoverageListResponse
	err = json.Unmarshal(body, &data)
	require.NoError(t, err)

	require.Equal(t, want, data)
}

func Test_CoverageHandler_FileList(t *testing.T) {
	rm := NewMockRepositoryClient()

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

	store := setupCoverageStore(t, cov)
	s := newCoverageHandler(store)

	req := httptest.NewRequest(
		http.MethodGet, fmt.Sprintf("/%d/go/files", cov.ID), nil)
	ctx := req.Context()
	ctx = base.WithRepositoryClient(ctx, rm)
	ctx = base.WithRepo(ctx, repo)
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
		RevisionURL: rm.RevisionURL(repo.Url, cov.Revision),
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
	//	contents.EXPECT().
	//		Find( /*ctx*/ gomock.Any(), orgName+"/"+repoName, filename, revision).
	//		Return(&content, nil, nil)
	contents.EXPECT().
		Find( /*ctx*/ gomock.Any(), orgName+"/"+repoName, filename, revision).
		DoAndReturn(func(ctx context.Context, repo, filename, revision string) (*scm.Content, *scm.Response, error) {
			token, ok := ctx.Value(scm.TokenKey{}).(*scm.Token)
			if !ok || token.Token != "valid_token" {
				return nil, nil, errors.New("no token or invalid token")
			}
			return &content, nil, nil
		}).AnyTimes()

	rm := NewMockRepositoryClient()
	rm.client.Contents = contents

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

	store := setupCoverageStore(t, cov)
	s := newCoverageHandler(store)

	path := fmt.Sprintf("/%d/%s/files/%s", cov.ID, entryName, filename)

	t.Run("with valid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		ctx := req.Context()
		ctx = scm.WithContext(ctx, &scm.Token{Token: "valid_token"})
		ctx = base.WithRepositoryClient(ctx, rm)
		ctx = base.WithRepo(ctx, repo)
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
	})

	t.Run("with invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		ctx := req.Context()
		ctx = scm.WithContext(ctx, &scm.Token{Token: "invalid_token"})
		ctx = base.WithRepositoryClient(ctx, rm)
		ctx = base.WithRepo(ctx, repo)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		s.Handler().ServeHTTP(w, req)

		result := w.Result()
		require.Equal(t, http.StatusNotFound, result.StatusCode)
	})

	t.Run("without token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		ctx := req.Context()
		ctx = base.WithRepositoryClient(ctx, rm)
		ctx = base.WithRepo(ctx, repo)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		s.Handler().ServeHTTP(w, req)

		result := w.Result()
		require.Equal(t, http.StatusNotFound, result.StatusCode)
	})
}

/*
func TestCoverageHandlerProcessUploadRequest(t *testing.T) {
	covStore := setupCoverageStore(t)
	repo := Repository{
		Namespace: "mockowner",
		Name:      "mockrepo",
		Url:       "http://mock.scm/mockowner/mockrepo",
	}
	repoStore := setupRepositoryStore(t, &repo)
	s := newCoverageHandler(repoStore, covStore)

	req, want := makeCoverageUploadRequest(repo)
	err := s.processUploadRequest(req)
	require.NoError(t, err)

	got, err := covStore.ListAll()
	require.NoError(t, err)
	want.ID = 1
	assert.Equal(t, []*Coverage{want}, got)
}
*/

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
	handler := newCoverageHandler(store)
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

	handler := newCoverageHandler(store)
	err := handler.AddCoverage(&added)

	require.NoError(t, err)

	got, err := store.ListAll()
	require.NoError(t, err)
	want.ID = 1
	assert.Equal(t, []*Coverage{want}, got)
}

func TestCoverageHandler_HandleUpload(t *testing.T) {
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

	request := &CoverageUploadRequest{
		RepoURL:   "hoge", // FIXME
		Revision:  cov.Revision,
		Timestamp: cov.Timestamp,
		Entries: []*CoverageEntryUploadRequest{
			{
				Name:     cov.Entries[0].Name,
				Profiles: []*profile.Profile{cov.Entries[0].Profiles["test.go"]},
				Hits:     cov.Entries[0].Hits,
				Lines:    cov.Entries[0].Lines,
			},
		},
	}
	body, err := json.Marshal(request)
	require.NoError(t, err)

	store := setupCoverageStore(t)
	s := newCoverageHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(body))
	req = req.WithContext(base.WithRepo(req.Context(), Repository{Id: cov.RepoID}))
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Result().StatusCode)
	got, err := store.ListAll()
	require.NoError(t, err)
	cov.ID = 1 // 1 will be assigned by the server
	assert.Equal(t, []*Coverage{cov}, got)
}
