package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/drone/go-scm/scm"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
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
	require.NotNil(t, admin.PasswordHash)
	err = bcrypt.CompareHashAndPassword([]byte(*admin.PasswordHash), []byte("admin"))
	require.NoError(t, err)
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

func TestFetchRawUser_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer testtoken", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":42,"login":"octocat","username":"","avatar_url":"https://example.com/av.jpg"}`))
	}))
	defer srv.Close()

	ru, err := fetchRawUser(srv.URL, "testtoken")
	require.NoError(t, err)
	require.Equal(t, 42, ru.ID)
	require.Equal(t, "octocat", ru.Login)
	require.Equal(t, "https://example.com/av.jpg", ru.AvatarURL)
}

func TestFetchRawUser_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`forbidden`))
	}))
	defer srv.Close()

	_, err := fetchRawUser(srv.URL, "testtoken")
	require.Error(t, err)
	require.Contains(t, err.Error(), "403")
}

func TestFetchRawUser_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid}`))
	}))
	defer srv.Close()

	_, err := fetchRawUser(srv.URL, "testtoken")
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse response")
}

func TestFetchGiteaUserInfo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/user", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"username":"giteauser","avatar_url":"https://av.com/1"}`))
	}))
	defer srv.Close()

	info, err := fetchGiteaUserInfo(srv.URL, "testtoken")
	require.NoError(t, err)
	require.Equal(t, "gitea", info.Provider)
	require.Equal(t, "giteauser", info.Username)
	require.Equal(t, "1", info.ProviderUserID)
	require.Equal(t, "https://av.com/1", info.AvatarURL)
}

func TestFetchGiteaUserInfo_FallbackToLogin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":2,"username":"","login":"gitealogin","avatar_url":""}`))
	}))
	defer srv.Close()

	info, err := fetchGiteaUserInfo(srv.URL, "testtoken")
	require.NoError(t, err)
	require.Equal(t, "gitealogin", info.Username)
}

func TestFetchGiteaUserInfo_NoUsername(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":3,"username":"","login":"","avatar_url":""}`))
	}))
	defer srv.Close()

	_, err := fetchGiteaUserInfo(srv.URL, "testtoken")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no username")
}

func TestFetchGenericUserInfo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/user", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":10,"username":"genericuser","avatar_url":"https://av.com/2"}`))
	}))
	defer srv.Close()

	info, err := fetchGenericUserInfo(srv.URL, "testtoken")
	require.NoError(t, err)
	require.Equal(t, srv.URL, info.Provider)
	require.Equal(t, "genericuser", info.Username)
	require.Equal(t, "10", info.ProviderUserID)
}

func TestFetchGenericUserInfo_FallbackToLogin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":11,"username":"","login":"genericlogin","avatar_url":""}`))
	}))
	defer srv.Close()

	info, err := fetchGenericUserInfo(srv.URL, "testtoken")
	require.NoError(t, err)
	require.Equal(t, "genericlogin", info.Username)
}

func TestFetchGenericUserInfo_NoUsername(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":12,"username":"","login":"","avatar_url":""}`))
	}))
	defer srv.Close()

	_, err := fetchGenericUserInfo(srv.URL, "testtoken")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no username")
}

func TestFetchProviderUserInfo_Generic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":100,"username":"genericroute","avatar_url":""}`))
	}))
	defer srv.Close()

	rm := NewMockRepositoryManager(1)
	rm.url, _ = url.Parse(srv.URL)
	token := &scm.Token{Token: "testtoken"}

	info, err := fetchProviderUserInfo(rm, token)
	require.NoError(t, err)
	require.Equal(t, srv.URL, info.Provider)
	require.Equal(t, "genericroute", info.Username)
	require.Equal(t, "100", info.ProviderUserID)
}

func TestFetchProviderUserInfo_Gitea(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/user", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":200,"username":"gitearoute","avatar_url":""}`))
	}))
	defer srv.Close()

	giteaURL := strings.Replace(srv.URL, "http://", "http://gitea@", 1)
	rm := NewMockRepositoryManager(1)
	rm.url, _ = url.Parse(giteaURL)
	token := &scm.Token{Token: "testtoken"}

	info, err := fetchProviderUserInfo(rm, token)
	require.NoError(t, err)
	require.Equal(t, "gitea", info.Provider)
	require.Equal(t, "gitearoute", info.Username)
}

func TestCreateUserForSession_ExistingUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":42,"username":"testuser","avatar_url":""}`))
	}))
	defer srv.Close()

	store := newTestUserStore(t)
	user, err := store.CreateUser("testuser", "")
	require.NoError(t, err)
	err = store.LinkAuth(user.ID, srv.URL, "42")
	require.NoError(t, err)

	sess := NewMoraSession()
	rm := NewMockRepositoryManager(1)
	rm.url, _ = url.Parse(srv.URL)
	token := &scm.Token{Token: "testtoken"}

	createUserForSession(sess, rm, token, store)
	require.NotNil(t, sess.UserID())
	require.Equal(t, user.ID, *sess.UserID())
}

func TestCreateUserForSession_NewUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":99,"username":"newuser","avatar_url":"https://av.com/new"}`))
	}))
	defer srv.Close()

	store := newTestUserStore(t)
	sess := NewMoraSession()
	rm := NewMockRepositoryManager(1)
	rm.url, _ = url.Parse(srv.URL)
	token := &scm.Token{Token: "testtoken"}

	createUserForSession(sess, rm, token, store)
	require.Nil(t, sess.UserID())
	require.NotNil(t, sess.PendingSignup())
	require.Equal(t, "newuser", sess.PendingSignup().username)
	require.Equal(t, "99", sess.PendingSignup().providerUserID)
	require.Equal(t, srv.URL, sess.PendingSignup().provider)
	require.Equal(t, "https://av.com/new", sess.PendingSignup().avatarURL)
}

func TestGitHubUserInfoFromRaw_Success(t *testing.T) {
	ru := &rawUserResponse{
		ID:        42,
		Login:     "octocat",
		AvatarURL: "https://avatars.com/1",
	}

	info := githubUserInfoFromRaw(ru)
	require.Equal(t, "github", info.Provider)
	require.Equal(t, "octocat", info.Username)
	require.Equal(t, "42", info.ProviderUserID)
	require.Equal(t, "https://avatars.com/1", info.AvatarURL)
}

func TestGitHubUserInfoFromRaw_FallbackToUsername(t *testing.T) {
	ru := &rawUserResponse{
		ID:       43,
		Login:    "",
		Username: "monalisa",
	}

	info := githubUserInfoFromRaw(ru)
	require.Equal(t, "monalisa", info.Username)
}

func TestFetchRawUser_NewRequestError(t *testing.T) {
	_, err := fetchRawUser("://invalid", "testtoken")
	require.Error(t, err)
	require.Contains(t, err.Error(), "create request")
}

func TestFetchGiteaUserInfo_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := fetchGiteaUserInfo(srv.URL, "testtoken")
	require.Error(t, err)
}

func TestCreateUserForSession_FetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	store := newTestUserStore(t)
	sess := NewMoraSession()
	rm := NewMockRepositoryManager(1)
	rm.url, _ = url.Parse(srv.URL)
	token := &scm.Token{Token: "testtoken"}

	createUserForSession(sess, rm, token, store)
	require.Nil(t, sess.UserID())
	require.Nil(t, sess.PendingSignup())
}

func TestCreateUserForSession_FindByProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":55,"username":"dbuser","avatar_url":""}`))
	}))
	defer srv.Close()

	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)
	store := NewUserStore(db)
	require.NoError(t, store.Init())
	_ = db.Close()

	sess := NewMoraSession()
	rm := NewMockRepositoryManager(1)
	rm.url, _ = url.Parse(srv.URL)
	token := &scm.Token{Token: "testtoken"}

	createUserForSession(sess, rm, token, store)
	require.Nil(t, sess.UserID())
	require.Nil(t, sess.PendingSignup())
}
