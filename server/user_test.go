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

func TestUserStore_CreateAPIKey(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUser("apikeyuser", "")
	require.NoError(t, err)

	plaintext, err := store.CreateAPIKey(user.ID, "test key")
	require.NoError(t, err)
	require.Contains(t, plaintext, "mora_")
	require.Len(t, plaintext, 69) // "mora_" (5) + 64 hex chars
}

func TestUserStore_CreateAPIKey_InvalidName(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUser("apikeyuser2", "")
	require.NoError(t, err)

	_, err = store.CreateAPIKey(user.ID, "")
	require.NoError(t, err) // empty name is allowed at DB level
}

func TestUserStore_ListAPIKeys(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUser("listkeyuser", "")
	require.NoError(t, err)

	keys, err := store.ListAPIKeys(user.ID)
	require.NoError(t, err)
	require.Empty(t, keys)

	_, err = store.CreateAPIKey(user.ID, "key1")
	require.NoError(t, err)
	_, err = store.CreateAPIKey(user.ID, "key2")
	require.NoError(t, err)

	keys, err = store.ListAPIKeys(user.ID)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	names := map[string]bool{}
	for _, k := range keys {
		names[k.Name] = true
	}
	require.True(t, names["key1"])
	require.True(t, names["key2"])
}

func TestUserStore_ListAPIKeys_OtherUser(t *testing.T) {
	store := newTestUserStore(t)

	user1, err := store.CreateUser("user1", "")
	require.NoError(t, err)
	user2, err := store.CreateUser("user2", "")
	require.NoError(t, err)

	_, err = store.CreateAPIKey(user1.ID, "user1key")
	require.NoError(t, err)

	keys, err := store.ListAPIKeys(user2.ID)
	require.NoError(t, err)
	require.Empty(t, keys)
}

func TestUserStore_RevokeAPIKey(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUser("revokekeyuser", "")
	require.NoError(t, err)

	_, err = store.CreateAPIKey(user.ID, "torevoke")
	require.NoError(t, err)

	keys, err := store.ListAPIKeys(user.ID)
	require.NoError(t, err)
	require.Len(t, keys, 1)

	err = store.RevokeAPIKey(user.ID, keys[0].ID)
	require.NoError(t, err)

	keys, err = store.ListAPIKeys(user.ID)
	require.NoError(t, err)
	require.Empty(t, keys)
}

func TestUserStore_RevokeAPIKey_NotFound(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUser("revokenotfound", "")
	require.NoError(t, err)

	err = store.RevokeAPIKey(user.ID, 999)
	require.Error(t, err)
}

func TestUserStore_RevokeAPIKey_WrongUser(t *testing.T) {
	store := newTestUserStore(t)

	user1, err := store.CreateUser("user1", "")
	require.NoError(t, err)
	user2, err := store.CreateUser("user2", "")
	require.NoError(t, err)

	_, err = store.CreateAPIKey(user1.ID, "key1")
	require.NoError(t, err)

	keys, err := store.ListAPIKeys(user1.ID)
	require.NoError(t, err)
	require.Len(t, keys, 1)

	err = store.RevokeAPIKey(user2.ID, keys[0].ID)
	require.Error(t, err)
}

func TestUserStore_FindUserByAPIKey(t *testing.T) {
	store := newTestUserStore(t)

	user, err := store.CreateUser("findkeyuser", "")
	require.NoError(t, err)

	plaintext, err := store.CreateAPIKey(user.ID, "findme")
	require.NoError(t, err)

	found, err := store.FindUserByAPIKey(plaintext)
	require.NoError(t, err)
	require.Equal(t, user.ID, found.ID)
	require.Equal(t, user.Username, found.Username)
}

func TestUserStore_FindUserByAPIKey_NotFound(t *testing.T) {
	store := newTestUserStore(t)

	_, err := store.FindUserByAPIKey("mora_nonexistentkey0000000000000000000000000000000000000000000000000000")
	require.Error(t, err)
}

func TestUserStore_FindUserByAPIKey_InvalidFormat(t *testing.T) {
	store := newTestUserStore(t)

	_, err := store.FindUserByAPIKey("invalid-key")
	require.Error(t, err)
}

func TestUserStore_FindUserByAPIKey_WrongUserKey(t *testing.T) {
	store := newTestUserStore(t)

	user1, err := store.CreateUser("user1", "")
	require.NoError(t, err)
	_, err = store.CreateUser("user2", "")
	require.NoError(t, err)

	pk, err := store.CreateAPIKey(user1.ID, "key1")
	require.NoError(t, err)

	found, err := store.FindUserByAPIKey(pk)
	require.NoError(t, err)
	require.Equal(t, user1.ID, found.ID)
}
