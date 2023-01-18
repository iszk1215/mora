package server

import (
	"testing"
	"time"

	"github.com/iszk1215/mora/mora/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockStore struct {
	rec ScanedCoverage
	got string
}

func (s *MockStore) Scan() ([]ScanedCoverage, error) {
	return []ScanedCoverage{s.rec}, nil
}

func (s *MockStore) Put(cov Coverage, contents string) error {
	s.got = contents
	return nil
}

func TestMergeEntry(t *testing.T) {
	entry0 := CoverageEntry{
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
	}

	entry1 := CoverageEntry{
		Name:  "go",
		Hits:  2,
		Lines: 4,
		Profiles: map[string]*profile.Profile{
			"test2.go": {
				FileName: "test2.go",
				Hits:     2,
				Lines:    4,
				Blocks:   [][]int{{1, 2, 1}, {3, 4, 0}},
			},
		},
	}

	merged := mergeEntry(&entry0, &entry1)

	assert.Equal(t, "go", merged.Name)
	assert.Equal(t, 15, merged.Hits)
	assert.Equal(t, 21, merged.Lines)
	assert.Equal(t, 2, len(merged.Profiles))
	assert.Contains(t, merged.Profiles, "test.go")
	assert.Contains(t, merged.Profiles, "test2.go")
}

func TestMergeCoverage(t *testing.T) {
	url := "http://mockscm.com/mockowner/mockrepo"
	revision := "012345"

	coverage0 := Coverage{
		url:      url,
		revision: revision,
		time:     time.Now(),
		entries: []*CoverageEntry{
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

	coverage1 := Coverage{
		url:      url,
		revision: revision,
		time:     time.Now(),
		entries: []*CoverageEntry{
			{
				Name:  "cc",
				Hits:  13,
				Lines: 17,
				Profiles: map[string]*profile.Profile{
					"test.cc": {
						FileName: "test.cc",
						Hits:     13,
						Lines:    17,
						Blocks:   [][]int{{1, 5, 1}, {10, 13, 0}, {13, 20, 1}},
					},
				},
			},
		},
	}

	merged, err := mergeCoverage(&coverage0, &coverage1)
	require.NoError(t, err)
	assert.Equal(t, url, merged.RepoURL())
	assert.Equal(t, revision, merged.Revision())
	require.Equal(t, 2, len(merged.Entries()))
	assert.Contains(t, merged.Entries()[0].Name, "cc")
	assert.Contains(t, merged.Entries()[1].Name, "go")
}

func TestMergeCoverageErrorUrl(t *testing.T) {
	url := "http://mockscm.com/mockowner/mockrepo"
	revision := "012345"

	coverage0 := Coverage{
		url:      url,
		revision: revision,
		time:     time.Now(),
		entries:  nil,
	}

	coverage1 := Coverage{
		url:      "http://foo.com/bar",
		revision: revision,
		time:     time.Now(),
	}

	_, err := mergeCoverage(&coverage0, &coverage1)
	require.Error(t, err)
}

func TestMergeCoverageErrorRevision(t *testing.T) {
	url := "http://mockscm.com/mockowner/mockrepo"
	revision := "012345"

	coverage0 := Coverage{
		url:      url,
		revision: revision,
		time:     time.Now(),
		entries:  nil,
	}

	coverage1 := Coverage{
		url:      url,
		revision: "3456",
		time:     time.Now(),
	}

	_, err := mergeCoverage(&coverage0, &coverage1)
	require.Error(t, err)
}

func TestMoraCoverageProviderAddCoverage(t *testing.T) {
	// TODO: more simple coverage
	cov := Coverage{
		url:      "http://mockscm.com/mockowner/mockrepo",
		revision: "012345",
		time:     time.Now(),
		entries: []*CoverageEntry{
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
					"test2.go": {
						FileName: "test2.go",
						Hits:     0,
						Lines:    3,
						Blocks:   [][]int{{1, 3, 0}},
					},
				},
			},
		},
	}

	// body, err := json.Marshal(req)
	// require.NoError(t, err)

	store := MockStore{}
	p := NewMoraCoverageProvider(&store)

	// w := httptest.NewRecorder()
	// r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(body))
	err := p.AddCoverage(&cov)
	require.NoError(t, err)

	/*
		res := w.Result()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, 1, len(p.coverages))
	*/

	/*
		got := p.coverages[0]
		assert.Equal(t, cov.Revision(), got.Revision())
		require.Equal(t, 1, len(got.entries))

		entry := got.entries[0]
		assert.Equal(t, 13, entry.Hits)
		assert.Equal(t, 20, entry.Lines)
		assert.Equal(t, 2, len(entry.files))
	*/

	exp := `[{"entry":"go","profiles":[{"filename":"test.go","hits":13,"lines":17,"blocks":[[1,5,1],[10,13,0],[13,20,1]]},{"filename":"test2.go","hits":0,"lines":3,"blocks":[[1,3,0]]}],"hits":13,"lines":20}]`

	assert.Equal(t, exp, store.got)
	t.Log(store.got)

	// require.Equal(t, http.StatusOK, res.StatusCode)
}

func TestHandlerUploadMerge(t *testing.T) {
	coverage0 := Coverage{
		url:      "http://mockscm.com/mockowner/mockrepo",
		revision: "012345",
		time:     time.Now(),
		entries: []*CoverageEntry{
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

	store := MockStore{}
	p := NewMoraCoverageProvider(&store)
	p.coverages = append(p.coverages, &coverage0)

	req := CoverageUploadRequest{
		RepoURL:  "http://mockscm.com/mockowner/mockrepo",
		Revision: "012345",
		Time:     time.Now(),
		Entries: []*CoverageEntryUploadRequest{
			{
				EntryName: "go",
				Hits:      0,
				Lines:     3,
				Profiles: []*profile.Profile{
					{
						FileName: "test2.go",
						Hits:     0,
						Lines:    3,
						Blocks:   [][]int{{1, 3, 0}},
					},
				},
			},
		},
	}

	cov, err := parseCoverage(&req)
	require.NoError(t, err)

	contents, err := p.makeContents(cov)
	require.NoError(t, err)
	t.Log(string(contents))

	entries, err := parseScanedCoverageContents(string(contents))
	require.NoError(t, err)
	require.Equal(t, 1, len(entries))

	entry := entries[0]
	assert.Equal(t, "go", entry.Name)
	assert.Equal(t, 13, entry.Hits)
	assert.Equal(t, 20, entry.Lines)

	require.Equal(t, 2, len(entry.Profiles))

	require.Contains(t, entry.Profiles, "test.go")
	require.Contains(t, entry.Profiles, "test2.go")

	exp := `[{"entry":"go","profiles":[{"filename":"test.go","hits":13,"lines":17,"blocks":[[1,5,1],[10,13,0],[13,20,1]]},{"filename":"test2.go","hits":0,"lines":3,"blocks":[[1,3,0]]}],"hits":13,"lines":20}]`
	assert.Equal(t, exp, string(contents))
}

func TestMoraCoverageProviderNew(t *testing.T) {
	rec := ScanedCoverage{
		RepoURL:  "url",
		Revision: "0123",
		Time:     time.Now(),
		Contents: `[{"entry":"go","hits":1,"lines":2}]`,
	}

	store := MockStore{rec: rec}

	provider := NewMoraCoverageProvider(&store)
	coverages := provider.Coverages()
	require.Equal(t, 1, len(coverages))

	cov := coverages[0]
	assert.Equal(t, rec.RepoURL, cov.RepoURL())
	assert.Equal(t, rec.Revision, cov.Revision())
	assert.Equal(t, rec.Time, cov.Time())
}
