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
	got string
}

func (s *MockStore) Scan() ([]ScanedCoverage, error) {
	return s.rec, nil
}

func (s *MockStore) Put(cov Coverage, contents string) error {
	s.got = contents
	return nil
}

func assertEqualProfileBlock(t *testing.T, a [][]int, b [][]int) bool {
	require.Equal(t, len(a), len(b))
	ok := true
	for i, aa := range a {
		bb := b[i]
		require.Equal(t, 3, len(aa))
		require.Equal(t, 3, len(bb))
		for j := 0; j < 3; j++ {
			ok = ok && assert.Equal(t, aa[j], bb[j])
		}
	}
	return ok
}

func assertEqualProfile(t *testing.T, a *profile.Profile, b *profile.Profile) bool {
	ok := assert.Equal(t, a.FileName, b.FileName)
	ok = ok && assert.Equal(t, a.Hits, b.Hits)
	ok = ok && assert.Equal(t, a.Lines, b.Lines)
	ok = ok && assertEqualProfileBlock(t, a.Blocks, b.Blocks)
	return ok
}

func assertEqualCoverageEntry(t *testing.T, a *CoverageEntry, b *CoverageEntry) bool {
	ok := assert.Equal(t, a.Name, b.Name)
	ok = ok && assert.Equal(t, a.Hits, b.Hits)
	ok = ok && assert.Equal(t, a.Lines, b.Lines)
	require.Equal(t, len(a.Profiles), len(b.Profiles))
	for key, pa := range a.Profiles {
		ok = ok && assertEqualProfile(t, pa, b.Profiles[key])
	}
	return ok
}

func assertEqualCoverage(t *testing.T, a *Coverage, b *Coverage) bool {
	ok := assert.Equal(t, a.RepoURL(), b.RepoURL())
	ok = ok && assert.Equal(t, a.Revision(), b.Revision())
	ok = ok && assert.Equal(t, a.Time(), b.Time())
	require.Equal(t, len(a.Entries()), len(b.Entries()))
	for i, ea := range a.Entries() {
		ok = ok && assertEqualCoverageEntry(t, ea, b.Entries()[i])
	}
	return ok
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

	assertEqualCoverageEntry(t, &expected, merged)
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

	expected := Coverage{
		url:      url,
		revision: revision,
		time:     coverage0.Time(),
		entries: []*CoverageEntry{ // alphabetical
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

	assertEqualCoverage(t, &expected, merged)
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
	require.Equal(t, "", store.got)

	err := p.AddCoverage(&cov)
	require.NoError(t, err)

	require.Equal(t, 1, len(p.Coverages()))
	assertEqualCoverage(t, &cov, p.Coverages()[0])

	require.NotEqual(t, "", store.got)
	entries, err := parseScanedCoverageContents(store.got)
	require.NoError(t, err)

	require.Equal(t, 1, len(entries))
	assertEqualCoverageEntry(t, cov.Entries()[0], entries[0])
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
	p.coverages = append(p.coverages, &existing)

	err := p.AddCoverage(&added)
	require.NoError(t, err)

	entries, err := parseScanedCoverageContents(store.got)
	require.NoError(t, err)
	require.Equal(t, 1, len(entries))

	entry := entries[0]
	assertEqualCoverageEntry(t, &expected, entry)
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
