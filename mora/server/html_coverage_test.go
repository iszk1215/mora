package server

import (
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func requireEqualHtmlCoverage(t *testing.T, e, g *htmlCoverage) {
	require.Equal(t, e.Time_, g.Time_)
	require.Equal(t, e.Revision_, g.Revision_)
	require.Equal(t, e.Directory, g.Directory)
	require.Equal(t, len(e.Entries_), len(g.Entries_))

	for i, ea := range e.Entries_ {
		eb := g.Entries_[i]
		require.Equal(t, *ea, *eb)
	}
}

func createMockDataset(t *testing.T) (fs.FS, *Repo, *htmlCoverage) {
	repo := &Repo{Namespace: "mockowner", Name: "mockrepo"} // FIXME
	ts, _ := time.Parse(time.RFC3339, "2022-05-06T10:46:53+09:00")
	cov := &htmlCoverage{
		RepoURL_:  repo.Link,
		Time_:     ts,
		Revision_: "130351ab1f695620cb6db0c068e4a849812d0a48",
		Directory: "",
		Entries_:  []*htmlCoverageEntry{{"go", 511, 73, "index.html"}},
	}

	data, err := yaml.Marshal(&cov)
	require.NoError(t, err)

	file := fstest.MapFile{Data: data}
	fsys := fstest.MapFS{
		"01/mora.yaml": &file,
	}

	cov.Directory = filepath.Join("01", cov.Directory)

	return fsys, repo, cov
}

func TestLoad(t *testing.T) {
	fsys, _, expected := createMockDataset(t)

	covs, err := loadDirectory(fsys, "", "mora.yaml")
	assert.NoError(t, err)
	require.Equal(t, 1, len(covs))

	requireEqualHtmlCoverage(t, expected, covs[0])
}
