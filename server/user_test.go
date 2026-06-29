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

func TestUserStore_CreateUser(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUser("octocat", "https://avatars.example.com/42")
	require.NoError(t, err)
	require.NotZero(t, user.ID)
	require.Equal(t, "octocat", user.Username)
	require.Equal(t, "https://avatars.example.com/42", user.AvatarURL)
}

func TestUserStore_LinkAuthAndFindByProvider(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUser("octocat", "")
	require.NoError(t, err)

	err = store.LinkAuth(user.ID, "github", "42")
	require.NoError(t, err)

	found, err := store.FindByProvider("github", "42")
	require.NoError(t, err)
	require.Equal(t, user.ID, found.ID)
	require.Equal(t, "octocat", found.Username)
}

func TestUserStore_FindByProvider_NotFound(t *testing.T) {
	store := newTestUserStore(t)

	_, err := store.FindByProvider("github", "999")
	require.Error(t, err)
}

func TestUserStore_FindByProvider_WrongProvider(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUser("octocat", "")
	require.NoError(t, err)
	err = store.LinkAuth(user.ID, "github", "42")
	require.NoError(t, err)

	_, err = store.FindByProvider("gitlab", "42")
	require.Error(t, err)
}

func TestUserStore_LinkAuth_Duplicate(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUser("user1", "")
	require.NoError(t, err)

	err = store.LinkAuth(user.ID, "github", "42")
	require.NoError(t, err)

	err = store.LinkAuth(user.ID, "github", "42")
	require.Error(t, err)
}

func TestUserStore_FindByID(t *testing.T) {
	store := newTestUserStore(t)

	created, err := store.CreateUser("testuser", "")
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

func TestSessionUserID(t *testing.T) {
	sess := NewMoraSession()
	require.Nil(t, sess.UserID())

	sess.SetUserID(42)
	require.NotNil(t, sess.UserID())
	require.Equal(t, int64(42), *sess.UserID())

	sess.ClearUserID()
	require.Nil(t, sess.UserID())
}

func TestPendingSignup(t *testing.T) {
	sess := NewMoraSession()
	require.Nil(t, sess.PendingSignup())

	p := &pendingSignup{
		rmID:           1,
		provider:       "github",
		providerUserID: "42",
		username:       "octocat",
		avatarURL:      "https://example.com/avatar.jpg",
	}
	sess.SetPendingSignup(p)
	require.NotNil(t, sess.PendingSignup())
	require.Equal(t, "octocat", sess.PendingSignup().username)

	sess.ClearPendingSignup()
	require.Nil(t, sess.PendingSignup())
}

func TestUserStore_SuperuserSeed(t *testing.T) {
	store := newTestUserStore(t)

	admin, err := store.FindByID(1)
	require.NoError(t, err)
	require.Equal(t, "admin", admin.Username)
}

func TestUserStore_CreateUser_EmptyAvatar(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUser("noname", "")
	require.NoError(t, err)
	require.Equal(t, "", user.AvatarURL)
}
