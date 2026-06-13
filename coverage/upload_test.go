package coverage

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_relativePathFromRoot(t *testing.T) {
	fsys := fstest.MapFS{
		"src/test.cc": &fstest.MapFile{},
	}

	got := relativePathFromRoot("/home/mora/test/src/test.cc", fsys)

	assert.Equal(t, "src/test.cc", got)
}

func TestUpload_ReturnsErrorWhenDirty(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "repo")
	err := os.MkdirAll(gitDir, 0755)
	require.NoError(t, err)

	repo, err := git.PlainInit(gitDir, false)
	require.NoError(t, err)

	srcFile := filepath.Join(gitDir, "test.go")
	err = os.WriteFile(srcFile, []byte("package main\n"), 0644)
	require.NoError(t, err)

	lcovFile := filepath.Join(gitDir, "coverage.lcov")
	lcovContent := "SF:test.go\nDA:1,1\nDA:2,1\nend_of_record\n"
	err = os.WriteFile(lcovFile, []byte(lcovContent), 0644)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("test.go")
	require.NoError(t, err)
	_, err = wt.Add("coverage.lcov")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	err = os.WriteFile(srcFile, []byte("package main\n\nfunc main() {}\n"), 0644)
	require.NoError(t, err)

	err = Upload("", "http://example.com/repo", gitDir, "go", true, false, true, []string{lcovFile})
	require.Error(t, err)
	require.Contains(t, err.Error(), "dirty")
}
