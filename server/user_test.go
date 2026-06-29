package server

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func newTestUserStore(t *testing.T) UserStore {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)
	store := NewUserStore(db)
	require.NoError(t, store.Init())
	return store
}

func TestUserStore_FindOrCreate_CreatesNewUser(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.FindOrCreate("github", "42", "octocat", "https://avatars.example.com/42")
	require.NoError(t, err)
	require.NotZero(t, user.ID)
	require.Equal(t, "github", user.Provider)
	require.Equal(t, "42", user.ProviderUserID)
	require.Equal(t, "octocat", user.Username)
	require.Equal(t, "https://avatars.example.com/42", user.AvatarURL)
}

func TestUserStore_FindOrCreate_ReturnsExisting(t *testing.T) {
	store := newTestUserStore(t)

	user1, err := store.FindOrCreate("github", "42", "octocat", "https://avatars.example.com/42")
	require.NoError(t, err)

	user2, err := store.FindOrCreate("github", "42", "octocat", "https://avatars.example.com/42")
	require.NoError(t, err)
	require.Equal(t, user1.ID, user2.ID)
}

func TestUserStore_FindOrCreate_UpdatesUsernameAndAvatar(t *testing.T) {
	store := newTestUserStore(t)

	_, err := store.FindOrCreate("github", "99", "oldname", "https://avatars.example.com/old")
	require.NoError(t, err)

	user, err := store.FindOrCreate("github", "99", "newname", "https://avatars.example.com/new")
	require.NoError(t, err)
	require.Equal(t, "newname", user.Username)
	require.Equal(t, "https://avatars.example.com/new", user.AvatarURL)
}

func TestUserStore_FindByID(t *testing.T) {
	store := newTestUserStore(t)

	created, err := store.FindOrCreate("github", "100", "testuser", "")
	require.NoError(t, err)

	found, err := store.FindByID(created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)
	require.Equal(t, "testuser", found.Username)
}

func TestUserStore_FindByID_NotFound(t *testing.T) {
	store := newTestUserStore(t)

	_, err := store.FindByID(999)
	require.Error(t, err)
}

func TestUserStore_UniqueProviderUserID(t *testing.T) {
	store := newTestUserStore(t)

	user1, err := store.FindOrCreate("github", "1", "user1", "")
	require.NoError(t, err)

	_, err = store.FindOrCreate("gitlab", "1", "user1", "")
	require.NoError(t, err)

	require.Equal(t, int64(2), user1.ID)

	githubUsers, err := store.FindByID(2)
	require.NoError(t, err)
	require.Equal(t, "github", githubUsers.Provider)
}

func TestSessionUserID(t *testing.T) {
	sess := NewMoraSession()
	require.Nil(t, sess.UserID())

	sess.SetUserID(42)
	require.NotNil(t, sess.UserID())
	require.Equal(t, int64(42), *sess.UserID())

	sess.ClearUserID()
	require.Nil(t, sess.UserID())
}
