package coverage

import (
	"testing"
	"time"

	"github.com/iszk1215/mora/mora/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeCoverage(t *testing.T) {
	revision := "012345"

	coverage0 := Coverage{
		RepoID:    1215,
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
		RepoID:    1215,
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
		RepoID:    1215,
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
	revision := "012345"

	coverage0 := Coverage{
		RepoID:    1215,
		Revision:  revision,
		Timestamp: time.Now(),
		Entries:   nil,
	}

	coverage1 := Coverage{
		RepoID:    1976,
		Revision:  revision,
		Timestamp: time.Now(),
	}

	_, err := mergeCoverage(&coverage0, &coverage1)
	require.Error(t, err)
}

func TestMergeCoverageErrorRevision(t *testing.T) {
	revision := "012345"

	coverage0 := Coverage{
		RepoID:    1215,
		Revision:  revision,
		Timestamp: time.Now(),
		Entries:   nil,
	}

	coverage1 := Coverage{
		RepoID:    1215,
		Revision:  "3456",
		Timestamp: time.Now(),
	}

	_, err := mergeCoverage(&coverage0, &coverage1)
	require.Error(t, err)
}
