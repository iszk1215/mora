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

	expected := CoverageEntry{
		Name:  "go",
		Hits:  15,
		Lines: 21,
		Profiles: map[string]*profile.Profile{
			"test.go": {
				FileName: "test.go",
				Hits:     13,
				Lines:    17,
				Blocks:   [][]int{{1, 5, 1}, {10, 13, 0}, {13, 20, 1}},
			},
			"test2.go": {
				FileName: "test2.go",
				Hits:     2,
				Lines:    4,
				Blocks:   [][]int{{1, 2, 1}, {3, 4, 0}},
			},
		},
	}

	assert.Equal(t, expected, *merged)
}

func TestMergeCoverage(t *testing.T) {
	url := "http://mockscm.com/mockowner/mockrepo"
	revision := "012345"

	coverage0 := Coverage{
		RepoURL:   url,
		Revision:  revision,
		Timestamp: time.Now(),
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

	coverage1 := Coverage{
		RepoURL:   url,
		Revision:  revision,
		Timestamp: time.Now(),
		Entries: []*CoverageEntry{
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

	expected := Coverage{
		RepoURL:   url,
		Revision:  revision,
		Timestamp: coverage0.Timestamp,
		Entries: []*CoverageEntry{ // alphabetical
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

	assert.Equal(t, &expected, merged)
}

func TestMergeCoverageErrorUrl(t *testing.T) {
	url := "http://mockscm.com/mockowner/mockrepo"
	revision := "012345"

	coverage0 := Coverage{
		RepoURL:   url,
		Revision:  revision,
		Timestamp: time.Now(),
		Entries:   nil,
	}

	coverage1 := Coverage{
		RepoURL:   "http://foo.com/bar",
		Revision:  revision,
		Timestamp: time.Now(),
	}

	_, err := mergeCoverage(&coverage0, &coverage1)
	require.Error(t, err)
}

func TestMergeCoverageErrorRevision(t *testing.T) {
	url := "http://mockscm.com/mockowner/mockrepo"
	revision := "012345"

	coverage0 := Coverage{
		RepoURL:   url,
		Revision:  revision,
		Timestamp: time.Now(),
		Entries:   nil,
	}

	coverage1 := Coverage{
		RepoURL:   url,
		Revision:  "3456",
		Timestamp: time.Now(),
	}

	_, err := mergeCoverage(&coverage0, &coverage1)
	require.Error(t, err)
}
