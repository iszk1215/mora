package server

import (
	"testing"
	"time"

	"github.com/iszk1215/mora/mora/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockCoverageStore struct {
	rec []*Coverage
	got *Coverage
}

func (s *MockCoverageStore) Scan() ([]*Coverage, error) {
	return s.rec, nil
}

func (s *MockCoverageStore) Put(cov *Coverage) error {
	s.got = cov
	return nil
}

func TestMoraCoverageProviderAddCoverage(t *testing.T) {
	cov := Coverage{
		RepoID:    1215,
		Revision:  "012345",
		Timestamp: time.Now(),
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

	store := MockCoverageStore{}
	p := NewMoraCoverageProvider(&store)
	require.Nil(t, store.got)

	err := p.AddCoverage(&cov)
	require.NoError(t, err)
	require.NotNil(t, store.got)

	assert.Equal(t, &cov, store.got)
}

func TestHandlerAddCoveragedMerge(t *testing.T) {
	existing := Coverage{
		RepoID:    1215,
		Revision:  "012345",
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

	added := Coverage{
		RepoID:    1215,
		Revision:  "012345",
		Timestamp: time.Now(),
		Entries: []*CoverageEntry{
			{
				Name:  "go",
				Hits:  0,
				Lines: 3,
				Profiles: map[string]*profile.Profile{
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

	want := CoverageEntry{
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
	}

	store := MockCoverageStore{}
	p := NewMoraCoverageProvider(&store)
	require.Nil(t, store.got)
	p.coverages = append(p.coverages, &existing)

	err := p.AddCoverage(&added)
	require.NoError(t, err)
	require.NotNil(t, store.got)

	// cov, err := parseScanedCoverage(*store.got)
	// require.NoError(t, err)

	assert.Equal(t, []*CoverageEntry{&want}, store.got.Entries)
}
