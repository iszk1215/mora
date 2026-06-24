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

// Test_relativePathFromRoot_Fallback confirms the first-match-wins heuristic:
// when root/cmd/main.go does not exist but root/main.go does, the function
// falls back to root/main.go. This is intentional behaviour for lcov paths
// that use different directory layouts between build and checkout.
func Test_relativePathFromRoot_Fallback(t *testing.T) {
	fsys := fstest.MapFS{
		"main.go":     &fstest.MapFile{},
		"src/main.go": &fstest.MapFile{},
	}

	got := relativePathFromRoot("/home/user/project/cmd/main.go", fsys)

	assert.Equal(t, "main.go", got,
		"should fall back to root/main.go when cmd/main.go does not exist")
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

func TestUpload_ReturnsErrorWhenServerEmpty(t *testing.T) {
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

	err = Upload("", "http://example.com/repo", gitDir, "go", false, false, true, []string{lcovFile})
	require.Error(t, err)
	require.Contains(t, err.Error(), "server")
}

func initGitRepo(t *testing.T, dir string) *git.Repository {
	t.Helper()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	srcFile := filepath.Join(dir, "tracked.go")
	err = os.WriteFile(srcFile, []byte("package main\n"), 0644)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("tracked.go")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)
	return repo
}

func TestIsDirty_Clean(t *testing.T) {
	repo := initGitRepo(t, t.TempDir())
	dirty, err := isDirty(repo)
	require.NoError(t, err)
	assert.False(t, dirty, "clean repo should not be dirty")
}

func TestIsDirty_ModifiedFile(t *testing.T) {
	dir := t.TempDir()
	repo := initGitRepo(t, dir)

	err := os.WriteFile(filepath.Join(dir, "tracked.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	require.NoError(t, err)

	dirty, err := isDirty(repo)
	require.NoError(t, err)
	assert.True(t, dirty, "modified file should be dirty")
}

func TestIsDirty_StagedNewFile(t *testing.T) {
	dir := t.TempDir()
	repo := initGitRepo(t, dir)

	err := os.WriteFile(filepath.Join(dir, "new.go"), []byte("package new\n"), 0644)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("new.go")
	require.NoError(t, err)

	dirty, err := isDirty(repo)
	require.NoError(t, err)
	assert.True(t, dirty, "staged new file should be dirty")
}

func TestIsDirty_DeletedFile(t *testing.T) {
	dir := t.TempDir()
	repo := initGitRepo(t, dir)

	err := os.Remove(filepath.Join(dir, "tracked.go"))
	require.NoError(t, err)

	dirty, err := isDirty(repo)
	require.NoError(t, err)
	assert.True(t, dirty, "deleted file should be dirty")
}
