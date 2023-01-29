package server

import (
	"testing"
	"time"

	"github.com/iszk1215/mora/mora/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockStore struct {
	rec []ScanedCoverage
	got *ScanedCoverage
}

func (s *MockStore) Scan() ([]ScanedCoverage, error) {
	return s.rec, nil
}

func (s *MockStore) Put(cov *ScanedCoverage) error {
	s.got = cov
	return nil
}

func TestMoraCoverageProviderAddCoverage(t *testing.T) {
	cov := Coverage{
		RepoURL:   "http://mockscm.com/mockowner/mockrepo",
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

	store := MockStore{}
	p := NewMoraCoverageProvider(&store)
	require.Nil(t, store.got)

	err := p.AddCoverage(&cov)
	require.NoError(t, err)

	require.Equal(t, []*Coverage{&cov}, p.Coverages())

	require.NotNil(t, store.got)
	got, err := parseScanedCoverage(*store.got)
	require.NoError(t, err)

	assert.Equal(t, &cov, got)
}

func TestHandlerAddCoveragedMerge(t *testing.T) {
	existing := Coverage{
		RepoURL:   "http://mockscm.com/mockowner/mockrepo",
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
		RepoURL:   "http://mockscm.com/mockowner/mockrepo",
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

	store := MockStore{}
	p := NewMoraCoverageProvider(&store)
	require.Nil(t, store.got)
	p.coverages = append(p.coverages, &existing)

	err := p.AddCoverage(&added)
	require.NoError(t, err)
	require.NotNil(t, store.got)

	cov, err := parseScanedCoverage(*store.got)
	require.NoError(t, err)

	assert.Equal(t, []*CoverageEntry{&want}, cov.Entries)
}

func TestMoraCoverageProviderNew(t *testing.T) {
	want := Coverage{
		ID:        123,
		RepoURL:   "url",
		Revision:  "0123",
		Timestamp: time.Now().Round(0),
		Entries: []*CoverageEntry{
			{
				Name:     "go",
				Hits:     1,
				Lines:    2,
				Profiles: map[string]*profile.Profile{}},
		},
	}

	rec := ScanedCoverage{
		ID:       want.ID,
		RepoURL:  want.RepoURL,
		Revision: want.Revision,
		Time:     want.Timestamp,
		Contents: `[{"entry":"go","hits":1,"lines":2}]`,
	}

	store := MockStore{rec: []ScanedCoverage{rec}}

	provider := NewMoraCoverageProvider(&store)
	got := provider.Coverages()
	assert.Equal(t, []*Coverage{&want}, got)
}
