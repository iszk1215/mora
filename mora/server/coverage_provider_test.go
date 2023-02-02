package server

import (
	"testing"
	"time"

	"github.com/elliotchance/pie/v2"
	"github.com/iszk1215/mora/mora/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockCoverageStore struct {
	coverages []*Coverage
}

func (s *MockCoverageStore) findOne(f func(cov *Coverage) bool) (*Coverage, error) {
	filtered := pie.Filter(s.coverages, f)

	if len(filtered) == 0 {
		return nil, nil
	}

	return filtered[0], nil
}

func (s *MockCoverageStore) Find(id int64) (*Coverage, error) {
	return s.findOne(func(cov *Coverage) bool { return cov.ID == id })
}

func (s *MockCoverageStore) FindRevision(repoID int64, revision string) (*Coverage, error) {
	return s.findOne(
		func(cov *Coverage) bool {
			return cov.RepoID == repoID && cov.Revision == revision
		})
}

func (s *MockCoverageStore) List(repo_id int64) ([]*Coverage, error) {
	return pie.Filter(s.coverages, func(cov *Coverage) bool { return cov.RepoID == repo_id }), nil
}

func (s *MockCoverageStore) ListAll() ([]*Coverage, error) {
	return s.coverages, nil
}

func (s *MockCoverageStore) Put(cov *Coverage) error {
	found, _ := s.Find(cov.ID)
	if found != nil {
		added := []*Coverage{}
		for _, c := range s.coverages {
			if c.ID == found.ID {
				added = append(added, cov)
			} else {
				added = append(added, c)
			}
		}
		s.coverages = added
	} else {
		s.coverages = append(s.coverages, cov)
	}
	return nil
}

func TestMoraCoverageProviderAddCoverage(t *testing.T) {
	cov := &Coverage{
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

	store := &MockCoverageStore{}
	handler := NewCoverageHandler(nil, store)
	err := handler.AddCoverage(cov)

	require.NoError(t, err)
	assert.Equal(t, []*Coverage{cov}, store.coverages)
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

	want := Coverage{
		RepoID:    1215,
		Revision:  "012345",
		Timestamp: existing.Timestamp,
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

	store := &MockCoverageStore{}
	store.Put(&existing)

	handler := NewCoverageHandler(nil, store)
	err := handler.AddCoverage(&added)

	require.NoError(t, err)
	assert.Equal(t, []*Coverage{&want}, store.coverages)
}
