package server

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func setupRepoStore(t *testing.T) RepositoryStore {
	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	 db.MustExec("PRAGMA foreign_keys = OFF")

	s := NewRepositoryStore(db)
	err = s.Init()
	require.NoError(t, err)

	return s
}

func TestRepoStore_New(t *testing.T) {
	setupRepoStore(t)
}

func TestRepoStore_Put(t *testing.T) {
	s := setupRepoStore(t)

	repo := &Repository{
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "repo",
		Url:               "https://scm.com/owner/repo",
	}

	err := s.Put(repo)
	require.NoError(t, err)
	require.Equal(t, int64(1), repo.Id)
}

func TestRepoStore_Put_DuplicateURL(t *testing.T) {
	s := setupRepoStore(t)

	repo1 := &Repository{
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "repo1",
		Url:               "https://scm.com/owner/repo",
	}
	err := s.Put(repo1)
	require.NoError(t, err)

	repo2 := &Repository{
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "repo2",
		Url:               "https://scm.com/owner/repo",
	}
	err = s.Put(repo2)
	require.Error(t, err)
}

func TestRepoStore_Find(t *testing.T) {
	s := setupRepoStore(t)

	repo := &Repository{
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "repo",
		Url:               "https://scm.com/owner/repo",
	}
	err := s.Put(repo)
	require.NoError(t, err)

	got, err := s.Find(repo.Id)
	require.NoError(t, err)
	require.Equal(t, *repo, got)
}

func TestRepoStore_Find_NotFound(t *testing.T) {
	s := setupRepoStore(t)

	_, err := s.Find(999)
	require.Error(t, err)
}

func TestRepoStore_FindURL(t *testing.T) {
	s := setupRepoStore(t)

	repo := &Repository{
		RepositoryManager: 1,
		Namespace:         "owner",
		Name:              "repo",
		Url:               "https://scm.com/owner/repo",
	}
	err := s.Put(repo)
	require.NoError(t, err)

	got, err := s.FindURL("https://scm.com/owner/repo")
	require.NoError(t, err)
	require.Equal(t, *repo, got)
}

func TestRepoStore_FindURL_NotFound(t *testing.T) {
	s := setupRepoStore(t)

	_, err := s.FindURL("https://scm.com/nonexistent")
	require.Error(t, err)
}

func TestRepoStore_ListAll(t *testing.T) {
	s := setupRepoStore(t)

	repo1 := &Repository{
		RepositoryManager: 1,
		Namespace:         "owner1",
		Name:              "repo1",
		Url:               "https://scm.com/owner1/repo1",
	}
	repo2 := &Repository{
		RepositoryManager: 1,
		Namespace:         "owner2",
		Name:              "repo2",
		Url:               "https://scm.com/owner2/repo2",
	}
	err := s.Put(repo1)
	require.NoError(t, err)
	err = s.Put(repo2)
	require.NoError(t, err)

	got, err := s.ListAll()
	require.NoError(t, err)
	require.Len(t, got, 2)
}

func TestRepoStore_ListAll_Empty(t *testing.T) {
	s := setupRepoStore(t)

	got, err := s.ListAll()
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestRepoStore_Init_DBError(t *testing.T) {
	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	 db.MustExec("PRAGMA foreign_keys = OFF")
	require.NoError(t, db.Close())

	s := NewRepositoryStore(db)
	err = s.Init()
	require.Error(t, err)
}
