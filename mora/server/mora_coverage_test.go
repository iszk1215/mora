package server

import (
	"testing"
	"time"

	"github.com/iszk1215/mora/mora/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeEntry(t *testing.T) {
	entry0 := CoverageEntry{
		Name:  "go",
		Hits:  13,
		Lines: 17,
		profiles: map[string]*profile.Profile{
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
		profiles: map[string]*profile.Profile{
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
	assert.Equal(t, 2, len(merged.profiles))
	assert.Contains(t, merged.profiles, "test.go")
	assert.Contains(t, merged.profiles, "test2.go")
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
				profiles: map[string]*profile.Profile{
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
				profiles: map[string]*profile.Profile{
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
	assert.Equal(t, 2, len(merged.Entries()))
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

func TestHandleUpload(t *testing.T) {
	profile0 := &profile.Profile{
		FileName: "test.go",
		Hits:     13,
		Lines:    17,
		Blocks:   [][]int{{1, 5, 1}, {10, 13, 0}, {13, 20, 1}},
	}
	profile1 := &profile.Profile{
		FileName: "test2.go",
		Hits:     0,
		Lines:    3,
		Blocks:   [][]int{{1, 3, 0}},
	}
	profiles := []*profile.Profile{profile0, profile1}

	e := &CoverageEntryUploadRequest{
		EntryName: "go",
		Profiles:  profiles,
		Hits:      13,
		Lines:     20,
	}
	entries := []*CoverageEntryUploadRequest{e}

	req := CoverageUploadRequest{
		RepoURL:  "http://mockscm.com/mockowner/mockrepo",
		Revision: "012345",
		Time:     time.Now(),
		Entries:  entries,
	}

	// body, err := json.Marshal(req)
	// require.NoError(t, err)

	p := NewMoraCoverageProvider(nil)

	// w := httptest.NewRecorder()
	// r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(body))
	err := p.HandleUploadRequest(&req)
	require.NoError(t, err)

	/*
		res := w.Result()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, 1, len(p.coverages))
	*/

	cov := p.coverages[0]
	assert.Equal(t, cov.Revision(), req.Revision)
	require.Equal(t, 1, len(cov.entries))

	entry := cov.entries[0]
	assert.Equal(t, 13, entry.Hits)
	assert.Equal(t, 20, entry.Lines)
	assert.Equal(t, 2, len(entry.profiles))

	// require.Equal(t, http.StatusOK, res.StatusCode)
}

func TestAddOrMergeCoverage(t *testing.T) {
	coverage0 := Coverage{
		url:      "http://mockscm.com/mockowner/mockrepo",
		revision: "012345",
		time:     time.Now(),
		entries: []*CoverageEntry{
			{
				Name:  "go",
				Hits:  13,
				Lines: 17,
				profiles: map[string]*profile.Profile{
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
		url:      "http://mockscm.com/mockowner/mockrepo",
		revision: "012345",
		time:     time.Now(),
		entries: []*CoverageEntry{
			{
				Name:  "go",
				Hits:  13,
				Lines: 17,
				profiles: map[string]*profile.Profile{
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

	p := NewMoraCoverageProvider(nil)
	merged0 := p.addOrMergeCoverage(&coverage0)
	require.Nil(t, merged0)
	merged1 := p.addOrMergeCoverage(&coverage1)
	require.NotNil(t, merged1)
}
