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

func (s *MockStore) Put(cov ScanedCoverage) error {
	s.got = &cov
	return nil
}

func TestMoraCoverageProviderAddCoverage(t *testing.T) {
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
				},
			},
		},
	}

	store := MockStore{}
	p := NewMoraCoverageProvider(&store)
	require.Nil(t, store.got)

	err := p.AddCoverage(&cov)
	require.NoError(t, err)

	require.Equal(t, 1, len(p.Coverages()))
	assertEqualCoverage(t, &cov, p.Coverages()[0])

	require.NotNil(t, store.got)
	got, err := parseScanedCoverage(*store.got)
	require.NoError(t, err)

	assertEqualCoverage(t, &cov, got)
}

func TestParseCoverage(t *testing.T) {
	now := time.Now()
	prof := profile.Profile{
		FileName: "test2.go",
		Hits:     0,
		Lines:    3,
		Blocks:   [][]int{{1, 3, 0}},
	}

	req := CoverageUploadRequest{
		RepoURL:  "http://mockscm.com/mockowner/mockrepo",
		Revision: "012345",
		Time:     now,
		Entries: []*CoverageEntryUploadRequest{
			{
				EntryName: "go",
				Hits:      0,
				Lines:     3,
				Profiles:  []*profile.Profile{&prof},
			},
		},
	}

	got, err := parseCoverage(&req)
	require.NoError(t, err)

	expected := Coverage{
		url:      "http://mockscm.com/mockowner/mockrepo",
		revision: "012345",
		time:     now,
		entries: []*CoverageEntry{
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

	assertEqualCoverage(t, &expected, got)
}

func TestHandlerAddCoveragedMerge(t *testing.T) {
	existing := Coverage{
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

	added := Coverage{
		url:      "http://mockscm.com/mockowner/mockrepo",
		revision: "012345",
		time:     time.Now(),
		entries: []*CoverageEntry{
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

	expected := CoverageEntry{
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

	require.Equal(t, 1, len(cov.Entries()))
	assertEqualCoverageEntry(t, &expected, cov.Entries()[0])
}

func TestMoraCoverageProviderNew(t *testing.T) {
	rec := ScanedCoverage{
		RepoURL:  "url",
		Revision: "0123",
		Time:     time.Now(),
		Contents: `[{"entry":"go","hits":1,"lines":2}]`,
	}

	store := MockStore{rec: []ScanedCoverage{rec}}

	provider := NewMoraCoverageProvider(&store)
	coverages := provider.Coverages()
	require.Equal(t, 1, len(coverages))

	cov := coverages[0]
	assert.Equal(t, rec.RepoURL, cov.RepoURL())
	assert.Equal(t, rec.Revision, cov.Revision())
	assert.Equal(t, rec.Time, cov.Time())
}
