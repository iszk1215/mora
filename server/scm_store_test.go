package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupScmStore(t *testing.T) RepositoryManagerStore {
	db := openTestDB(t)
	s := NewRepositoryManagerStore(db)
	err := s.Init()
	require.NoError(t, err)

	return s
}

func TestScmStore_New(t *testing.T) {
	setupScmStore(t)
}

func TestScmStore_FindURL_NotFound(t *testing.T) {
	s := setupScmStore(t)

	id, externalID, driver, err := s.FindURL("https://scm.example.com")
	require.NoError(t, err)
	require.Equal(t, int64(-1), id)
	require.Empty(t, externalID)
	require.Empty(t, driver)
}

func TestScmStore_Insert(t *testing.T) {
	s := setupScmStore(t)

	id, externalID, err := s.Insert("github", "https://github.com")
	require.NoError(t, err)
	require.Equal(t, int64(1), id)
	require.True(t, strings.HasPrefix(externalID, scmExternalIDPrefix))
	_, err = parseSCMExternalID(externalID)
	require.NoError(t, err)
}

func TestScmStore_Insert_Duplicate(t *testing.T) {
	s := setupScmStore(t)

	_, _, err := s.Insert("github", "https://github.com")
	require.NoError(t, err)

	_, _, err = s.Insert("gitlab", "https://github.com")
	require.Error(t, err)
}

func TestScmStore_FindURL_AfterInsert(t *testing.T) {
	s := setupScmStore(t)

	_, externalID, err := s.Insert("gitea", "https://gitea.example.com")
	require.NoError(t, err)

	id, gotExternalID, driver, err := s.FindURL("https://gitea.example.com")
	require.NoError(t, err)
	require.Equal(t, int64(1), id)
	require.Equal(t, externalID, gotExternalID)
	require.Equal(t, "gitea", driver)
}

func TestScmStore_Init_MigratesExternalID(t *testing.T) {
	db := openTestDB(t)
	db.MustExec(`CREATE TABLE scm (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		driver TEXT NOT NULL,
		url TEXT NOT NULL UNIQUE
	)`)
	db.MustExec(`INSERT INTO scm (driver, url)
		VALUES ('gitea', 'https://gitea.example.com')`)

	s := NewRepositoryManagerStore(db)
	require.NoError(t, s.Init())

	id, externalID, driver, err := s.FindURL("https://gitea.example.com")
	require.NoError(t, err)
	require.Equal(t, int64(1), id)
	require.Equal(t, "gitea", driver)
	require.True(t, strings.HasPrefix(externalID, scmExternalIDPrefix))
	_, err = parseSCMExternalID(externalID)
	require.NoError(t, err)

	require.NoError(t, s.Init())
}

func TestScmStore_Init_DBError(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Close())

	s := NewRepositoryManagerStore(db)
	err := s.Init()
	require.Error(t, err)
}

func TestScmStore_FindURL_MultipleEntries(t *testing.T) {
	s := setupScmStore(t)

	_, _, err := s.Insert("github", "https://github.com")
	require.NoError(t, err)
	_, _, err = s.Insert("gitlab", "https://gitlab.com")
	require.NoError(t, err)
	_, _, err = s.Insert("gitea", "https://gitea.example.com")
	require.NoError(t, err)

	id, externalID, driver, err := s.FindURL("https://gitlab.com")
	require.NoError(t, err)
	require.Equal(t, int64(2), id)
	require.NotEmpty(t, externalID)
	require.Equal(t, "gitlab", driver)
}
