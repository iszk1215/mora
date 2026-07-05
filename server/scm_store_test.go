package server

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func setupScmStore(t *testing.T) RepositoryManagerStore {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	s := NewRepositoryManagerStore(db)
	err = s.Init()
	require.NoError(t, err)

	return s
}

func TestScmStore_New(t *testing.T) {
	setupScmStore(t)
}

func TestScmStore_FindURL_NotFound(t *testing.T) {
	s := setupScmStore(t)

	id, driver, err := s.FindURL("https://scm.example.com")
	require.NoError(t, err)
	require.Equal(t, int64(-1), id)
	require.Empty(t, driver)
}

func TestScmStore_Insert(t *testing.T) {
	s := setupScmStore(t)

	id, err := s.Insert("github", "https://github.com")
	require.NoError(t, err)
	require.Equal(t, int64(1), id)
}

func TestScmStore_Insert_Duplicate(t *testing.T) {
	s := setupScmStore(t)

	_, err := s.Insert("github", "https://github.com")
	require.NoError(t, err)

	_, err = s.Insert("gitlab", "https://github.com")
	require.Error(t, err)
}

func TestScmStore_FindURL_AfterInsert(t *testing.T) {
	s := setupScmStore(t)

	_, err := s.Insert("gitea", "https://gitea.example.com")
	require.NoError(t, err)

	id, driver, err := s.FindURL("https://gitea.example.com")
	require.NoError(t, err)
	require.Equal(t, int64(1), id)
	require.Equal(t, "gitea", driver)
}

func TestScmStore_Init_DBError(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s := NewRepositoryManagerStore(db)
	err = s.Init()
	require.Error(t, err)
}

func TestScmStore_FindURL_MultipleEntries(t *testing.T) {
	s := setupScmStore(t)

	_, err := s.Insert("github", "https://github.com")
	require.NoError(t, err)
	_, err = s.Insert("gitlab", "https://gitlab.com")
	require.NoError(t, err)
	_, err = s.Insert("gitea", "https://gitea.example.com")
	require.NoError(t, err)

	id, driver, err := s.FindURL("https://gitlab.com")
	require.NoError(t, err)
	require.Equal(t, int64(2), id)
	require.Equal(t, "gitlab", driver)
}
