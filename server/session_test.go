package server

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/drone/go-scm/scm"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

type failReader struct{}

func (f *failReader) Read(p []byte) (int, error) {
	return 0, errors.New("mock read error")
}

// recordingSessionStore counts session store operations so tests can assert
// that read-only requests do not hit the database.
type recordingSessionStore struct {
	sessionStore
	gets    int
	puts    int
	touches int
}

func (s *recordingSessionStore) Get(sid string) (*MoraSession, bool) {
	s.gets++
	return s.sessionStore.Get(sid)
}

func (s *recordingSessionStore) Put(sid string, sess *MoraSession) error {
	s.puts++
	return s.sessionStore.Put(sid, sess)
}

func (s *recordingSessionStore) Touch(sid string, at time.Time) error {
	s.touches++
	return s.sessionStore.Touch(sid, at)
}

func newTestSessionManager() *MoraSessionManager {
	return &MoraSessionManager{
		cookiename:    "morasessionid",
		store:         newMemSessionStore(),
		lifetime:      24 * time.Hour,
		stopCh:        make(chan struct{}),
		touchInterval: time.Hour,
	}
}

// newRecordingDBSessionStore returns a database-backed recording session store
// whose Get/Put/Touch operations are counted.
func newRecordingDBSessionStore(t *testing.T) *recordingSessionStore {
	t.Helper()
	db := openTestDB(t)
	store := newDBSessionStore(db)
	require.NoError(t, store.Init())
	return &recordingSessionStore{sessionStore: store}
}

func TestSessionID_ReturnsErrorOnReadFailure(t *testing.T) {
	old := rand.Reader
	rand.Reader = &failReader{}
	defer func() { rand.Reader = old }()

	id, err := sessionID()
	require.Error(t, err)
	require.Empty(t, id)
}

func TestSessionMiddleware_Returns500OnSessionIDError(t *testing.T) {
	old := rand.Reader
	rand.Reader = &failReader{}
	defer func() { rand.Reader = old }()

	m := newTestSessionManager()
	next := func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}
	handler := m.SessionMiddleware(http.HandlerFunc(next))
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)

	require.Equal(t, http.StatusInternalServerError, got.Code)
}

func TestSessionManager(t *testing.T) {
	m := newTestSessionManager()
	next := func(w http.ResponseWriter, r *http.Request) {
		_, ok := MoraSessionFrom(r.Context())
		require.True(t, ok)
	}
	handler := m.SessionMiddleware(http.HandlerFunc(next))
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)
}

func TestSessionManager_SetsCookie(t *testing.T) {
	m := newTestSessionManager()
	next := func(w http.ResponseWriter, r *http.Request) {}
	handler := m.SessionMiddleware(http.HandlerFunc(next))
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)

	res := got.Result()
	cookies := res.Cookies()
	require.Len(t, cookies, 1)

	cookie := cookies[0]
	require.Equal(t, "morasessionid", cookie.Name)
	require.NotEmpty(t, cookie.Value)
	require.Equal(t, "/", cookie.Path)
	require.True(t, cookie.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	// Secure is enabled by default even when the server runs behind plain HTTP.
	require.True(t, cookie.Secure)
}

func TestSessionManager_InsecureCookie(t *testing.T) {
	m := newTestSessionManager()
	m.insecureCookie = true
	next := func(w http.ResponseWriter, r *http.Request) {}
	handler := m.SessionMiddleware(http.HandlerFunc(next))

	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)

	cookies := got.Result().Cookies()
	require.Len(t, cookies, 1)
	require.False(t, cookies[0].Secure,
		"Secure attribute should be omitted with insecure_cookie over HTTP")
}

func TestSessionManager_InsecureCookieStaysSecureOverHTTPS(t *testing.T) {
	tests := []struct {
		name string
		tls  bool
		xfp  string
	}{
		{name: "direct TLS connection", tls: true},
		{name: "X-Forwarded-Proto https", xfp: "https"},
		{name: "X-Forwarded-Proto HTTPS uppercase", xfp: "HTTPS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestSessionManager()
			m.insecureCookie = true
			handler := m.SessionMiddleware(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {}))

			req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			if tt.xfp != "" {
				req.Header.Set("X-Forwarded-Proto", tt.xfp)
			}
			got := httptest.NewRecorder()
			handler.ServeHTTP(got, req)

			cookies := got.Result().Cookies()
			require.Len(t, cookies, 1)
			require.True(t, cookies[0].Secure,
				"Secure should stay enabled when the request itself is over HTTPS")
		})
	}
}

func TestSessionManager_ReusesExistingSession(t *testing.T) {
	m := newTestSessionManager()

	// First request: no cookie -> middleware creates new session
	var firstSid string
	first := func(w http.ResponseWriter, r *http.Request) {
		sess, ok := MoraSessionFrom(r.Context())
		require.True(t, ok)
		sess.setToken(1, scm.Token{Token: "test-token"})
	}
	handler := m.SessionMiddleware(http.HandlerFunc(first))
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)

	res := got.Result()
	cookies := res.Cookies()
	require.Len(t, cookies, 1)
	firstSid = cookies[0].Value

	// Second request: include the cookie -> middleware reuses existing session
	var gotToken string
	second := func(w http.ResponseWriter, r *http.Request) {
		sess, ok := MoraSessionFrom(r.Context())
		require.True(t, ok)
		tok, ok := sess.getToken(1)
		require.True(t, ok, "token from first request should persist")
		gotToken = tok.Token
	}
	handler2 := m.SessionMiddleware(http.HandlerFunc(second))
	req2 := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	req2.AddCookie(&http.Cookie{Name: "morasessionid", Value: firstSid})
	got2 := httptest.NewRecorder()
	handler2.ServeHTTP(got2, req2)

	require.Equal(t, "test-token", gotToken)
}

func TestMoraSession_Remove(t *testing.T) {
	sess := NewMoraSession()
	sess.setToken(1, scm.Token{Token: "token1"})
	sess.setToken(2, scm.Token{Token: "token2"})
	sess.setReposCache(1, map[int64]bool{42: true})

	_, ok := sess.getToken(1)
	require.True(t, ok)
	_, ok = sess.getToken(2)
	require.True(t, ok)

	sess.Remove(1)

	_, ok = sess.getToken(1)
	require.False(t, ok, "token for rm 1 should be removed")
	_, ok = sess.getToken(2)
	require.True(t, ok, "token for rm 2 should remain")
	cache := sess.getReposCache(1)
	require.Nil(t, cache, "repos cache for rm 1 should be removed")
}

func TestMoraSessionDirectRace(t *testing.T) {
	t.Parallel()
	sess := NewMoraSession()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rmID := int64(i % 5)
			sess.setToken(rmID, scm.Token{Token: "token"})
			sess.getToken(rmID)
			sess.setReposCache(rmID, map[int64]bool{int64(i): true})
			sess.getReposCache(rmID)
		}(i)
	}
	wg.Wait()
}

func TestMoraSessionManager_GC(t *testing.T) {
	m := newTestSessionManager()
	m.lifetime = time.Hour

	now := time.Now()
	sess1 := NewMoraSession()
	sess1.timestamp = now.Add(-2 * time.Hour) // expired
	require.NoError(t, m.store.Put("sid1", sess1))

	sess2 := NewMoraSession()
	sess2.timestamp = now.Add(-30 * time.Minute) // still valid
	require.NoError(t, m.store.Put("sid2", sess2))

	m.GC()

	require.Equal(t, 1, m.store.Len())
	_, ok := m.store.Get("sid1")
	require.False(t, ok, "expired session should be removed")
	_, ok = m.store.Get("sid2")
	require.True(t, ok, "valid session should remain")
}

func TestMoraSessionConcurrentHTTPRace(t *testing.T) {
	t.Parallel()
	m := newTestSessionManager()
	handler := m.SessionMiddleware(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			sess, _ := MoraSessionFrom(r.Context())
			sess.setToken(1, scm.Token{Token: "test", Refresh: "refresh"})
			sess.getToken(1)
			sess.setReposCache(1, map[int64]bool{42: true})
			sess.getReposCache(1)
		},
	))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: "morasessionid", Value: "shared-sid"})
			handler.ServeHTTP(httptest.NewRecorder(), req)
		}()
	}
	wg.Wait()
}

func TestSessionManager_ConcurrentReadsDoNotSerialize(t *testing.T) {
	t.Parallel()
	m := newTestSessionManager()
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	next := func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
	}
	handler := m.SessionMiddleware(http.HandlerFunc(next))
	cookie := &http.Cookie{Name: "morasessionid", Value: "shared-sid"}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(cookie)
			handler.ServeHTTP(httptest.NewRecorder(), req)
		}()
	}

	// Both handlers must be running at the same time. If the per-session lock
	// were still held across the whole handler, the second request could not
	// enter until the first (blocked on release) had finished.
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("a request never entered the handler: same-sid requests are still serialized")
		}
	}
	close(release)
	wg.Wait()
}

func TestSessionManager_ConcurrentMutators_NoLostUpdate(t *testing.T) {
	t.Parallel()
	m := newTestSessionManager()
	const requests = 8
	handler := m.SessionMiddleware(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			rmID := int64(strings.Count(r.URL.RawQuery, "x")) + 1
			sess, _ := MoraSessionFrom(r.Context())
			sess.setToken(rmID, scm.Token{Token: r.URL.RawQuery})
		},
	))
	cookie := &http.Cookie{Name: "morasessionid", Value: "shared-mut-sid"}

	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(query string) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/?"+query, nil)
			req.AddCookie(cookie)
			handler.ServeHTTP(httptest.NewRecorder(), req)
		}(strings.Repeat("x", i))
	}
	wg.Wait()

	// Concurrent mutators share one session object, so every write-back
	// snapshot (serialized per session) already contains all changes made so
	// far; no update may be lost.
	sess, ok := m.store.Get("shared-mut-sid")
	require.True(t, ok)
	require.Len(t, sess.tokenMap, requests)
	for i := 1; i <= requests; i++ {
		tok, ok := sess.getToken(int64(i))
		require.True(t, ok, "token for rm %d was lost", i)
		require.Equal(t, strings.Repeat("x", i-1), tok.Token)
	}
}

func TestMoraSessionMarshalUnmarshal_RoundTrip(t *testing.T) {
	sess := NewMoraSession()
	sess.SetUserID(42)
	sess.setToken(1, scm.Token{Token: "access", Refresh: "refresh", Expires: time.Now().Add(time.Hour)})
	sess.setReposCache(1, map[int64]bool{7: true, 8: false})
	sess.SetPendingSignup(&pendingSignup{
		rmID:           2,
		provider:       "gitea",
		providerUserID: "user-123",
		username:       "alice",
		avatarURL:      "https://example.com/a.png",
	})
	sess.loggingInto = 3

	data, err := json.Marshal(sess)
	require.NoError(t, err)

	got := NewMoraSession()
	require.NoError(t, json.Unmarshal(data, got))

	require.Equal(t, int64(42), *got.UserID())
	tok, ok := got.getToken(1)
	require.True(t, ok)
	require.Equal(t, "access", tok.Token)
	require.Equal(t, "refresh", tok.Refresh)
	require.False(t, tok.Expires.IsZero())

	cache := got.getReposCache(1)
	require.NotNil(t, cache)
	require.Equal(t, map[int64]bool{7: true, 8: false}, cache)

	p := got.PendingSignup()
	require.NotNil(t, p)
	require.Equal(t, int64(2), p.rmID)
	require.Equal(t, "alice", p.username)

	require.Equal(t, int64(3), got.loggingInto)
	require.False(t, got.timestamp.IsZero())
}

func TestMoraSessionMarshalUnmarshal_EmptyMaps(t *testing.T) {
	sess := NewMoraSession()
	data, err := json.Marshal(sess)
	require.NoError(t, err)

	got := NewMoraSession()
	require.NoError(t, json.Unmarshal(data, got))

	// Unmarshaled sessions must have usable maps so later mutations do not panic.
	require.NotNil(t, got.reposMap)
	require.NotNil(t, got.tokenMap)
	got.setToken(1, scm.Token{Token: "t"})
	_, ok := got.getToken(1)
	require.True(t, ok)
}

func TestDBSessionStore_CRUD(t *testing.T) {
	db := openTestDB(t)
	store := newDBSessionStore(db)
	require.NoError(t, store.Init())
	require.Equal(t, 0, store.Len())

	sess := NewMoraSession()
	sess.SetUserID(7)
	require.NoError(t, store.Put("sid-1", sess))

	require.Equal(t, 1, store.Len())
	require.True(t, store.Has("sid-1"))

	got, ok := store.Get("sid-1")
	require.True(t, ok)
	require.Equal(t, int64(7), *got.UserID())

	require.NoError(t, store.Delete("sid-1"))
	require.False(t, store.Has("sid-1"))
	require.Equal(t, 0, store.Len())
	_, ok = store.Get("sid-1")
	require.False(t, ok)
}

func TestDBSessionStore_Expiry(t *testing.T) {
	db := openTestDB(t)
	store := newDBSessionStore(db)
	require.NoError(t, store.Init())

	fresh := NewMoraSession()
	fresh.timestamp = time.Now().Add(-30 * time.Minute)
	require.NoError(t, store.Put("fresh", fresh))

	stale := NewMoraSession()
	now := time.Now()
	stale.timestamp = now.Add(-2 * time.Hour)
	require.NoError(t, store.Put("stale", stale))

	require.NoError(t, store.DeleteExpired(now.Add(-time.Hour)))
	require.True(t, store.Has("fresh"))
	require.False(t, store.Has("stale"))
}

func TestIsHranaStreamError(t *testing.T) {
	streamErr := errors.New("Error preparing statement: Hrana: `api error: `status=404 Not Found, body={\"error\":\"stream not found: bd5cec4e:968958c\"}`")

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "stream not found", err: streamErr, want: true},
		{name: "stream has expired", err: errors.New("the stream has expired due to inactivity"), want: true},
		{name: "HRANA_CLOSED", err: errors.New("Hrana: HRANA_CLOSED"), want: true},
		{name: "unrelated", err: errors.New("sql: no such table: session"), want: false},
		{name: "nil", err: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isHranaStreamError(tt.err))
		})
	}
}

func TestDBSessionStore_DeleteExpiredRetriesOnHranaStreamError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer func() { _ = sqlDB.Close() }()

	resetCalled := false
	orig := resetIdlePool
	resetIdlePool = func(db *sqlx.DB) { resetCalled = true }
	defer func() { resetIdlePool = orig }()

	store := newDBSessionStore(sqlx.NewDb(sqlDB, "sqlmock"))

	cutoff := int64(1234567890)
	streamErr := errors.New("Error preparing statement: Hrana: `api error: `status=404 Not Found, body={\"error\":\"stream not found: bd5cec4e:968958c\"}`")

	mock.ExpectExec("DELETE FROM session WHERE last_seen <= ?").
		WithArgs(cutoff).
		WillReturnError(streamErr)
	mock.ExpectExec("DELETE FROM session WHERE last_seen <= ?").
		WithArgs(cutoff).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, store.DeleteExpired(time.Unix(0, cutoff)))
	require.True(t, resetCalled, "pool reset must run before the retry")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDBSessionStore_DeleteExpiredErrorAfterRetry(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer func() { _ = sqlDB.Close() }()

	orig := resetIdlePool
	resetIdlePool = func(db *sqlx.DB) {}
	defer func() { resetIdlePool = orig }()

	store := newDBSessionStore(sqlx.NewDb(sqlDB, "sqlmock"))

	cutoff := int64(1234567890)
	streamErr := errors.New("Hrana: `api error: `status=404 Not Found, body={\"error\":\"stream not found: bd5cec4e:968958c\"}`")

	mock.ExpectExec("DELETE FROM session WHERE last_seen <= ?").
		WithArgs(cutoff).
		WillReturnError(streamErr)
	mock.ExpectExec("DELETE FROM session WHERE last_seen <= ?").
		WithArgs(cutoff).
		WillReturnError(streamErr)

	err = store.DeleteExpired(time.Unix(0, cutoff))
	require.Error(t, err)
	require.Contains(t, err.Error(), "session DeleteExpired")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDBSessionStore_DeleteExpiredNonRetryableError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer func() { _ = sqlDB.Close() }()

	store := newDBSessionStore(sqlx.NewDb(sqlDB, "sqlmock"))

	cutoff := int64(1234567890)
	mock.ExpectExec("DELETE FROM session WHERE last_seen <= ?").
		WithArgs(cutoff).
		WillReturnError(errors.New("sql: no such table: session"))

	err = store.DeleteExpired(time.Unix(0, cutoff))
	require.Error(t, err)
	require.Contains(t, err.Error(), "session DeleteExpired")
	require.NoError(t, mock.ExpectationsWereMet(), "non-retryable errors must not trigger a retry")
}

func TestDBSessionStore_SharedAcrossStores(t *testing.T) {
	db := openTestDB(t)
	store1 := newDBSessionStore(db)
	require.NoError(t, store1.Init())

	sess := NewMoraSession()
	sess.SetUserID(99)
	require.NoError(t, store1.Put("shared-sid", sess))

	// Another store instance on the same database must see the session,
	// mimicking a separate Cloud Run instance.
	store2 := newDBSessionStore(db)
	require.NoError(t, store2.Init())
	got, ok := store2.Get("shared-sid")
	require.True(t, ok)
	require.Equal(t, int64(99), *got.UserID())
}

func TestMoraSessionManager_DBPersistenceThroughMiddleware(t *testing.T) {
	db := openTestDB(t)
	store := newDBSessionStore(db)
	require.NoError(t, store.Init())
	m := NewMoraSessionManagerWithStore(false, store)
	defer func() { _ = m.Close() }()

	// First request creates and persists a logged-in session.
	handler := m.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := MoraSessionFrom(r.Context())
		sess.SetUserID(11)
	}))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	sid := cookies[0].Value

	require.True(t, store.Has(sid))
	got, ok := store.Get(sid)
	require.True(t, ok)
	require.Equal(t, int64(11), *got.UserID())

	// A second request on a fresh manager (same DB) reuses the session,
	// like another Cloud Run instance seeing the cookie.
	m2 := NewMoraSessionManagerWithStore(false, newDBSessionStore(db))
	defer func() { _ = m2.Close() }()
	handler2 := m2.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := MoraSessionFrom(r.Context())
		require.Equal(t, int64(11), *sess.UserID())
	}))
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(&http.Cookie{Name: "morasessionid", Value: sid})
	handler2.ServeHTTP(w2, req2)
}

func TestSessionManager_SkipsGetWithoutCookie(t *testing.T) {
	store := &recordingSessionStore{sessionStore: newMemSessionStore()}
	m := &MoraSessionManager{
		cookiename:    "morasessionid",
		store:         store,
		lifetime:      24 * time.Hour,
		stopCh:        make(chan struct{}),
		touchInterval: time.Hour,
	}
	defer func() { _ = m.Close() }()

	handler := m.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.Equal(t, 0, store.gets, "a request without a session cookie must not read the store")
	require.Equal(t, 0, store.puts, "an anonymous request must not persist a session")
	require.Equal(t, 0, store.touches)
}

func TestSessionManager_PutsOnlyWhenDirty(t *testing.T) {
	store := &recordingSessionStore{sessionStore: newMemSessionStore()}
	m := &MoraSessionManager{
		cookiename:    "morasessionid",
		store:         store,
		lifetime:      24 * time.Hour,
		stopCh:        make(chan struct{}),
		touchInterval: time.Hour,
	}
	defer func() { _ = m.Close() }()

	// Mutating request persists the session.
	handler := m.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := MoraSessionFrom(r.Context())
		sess.setToken(1, scm.Token{Token: "t"})
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	sid := w.Result().Cookies()[0].Value
	require.Equal(t, 1, store.puts)

	// Read-only request with cookie must not rewrite the session.
	handler2 := m.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(&http.Cookie{Name: "morasessionid", Value: sid})
	handler2.ServeHTTP(httptest.NewRecorder(), req2)

	require.Equal(t, 1, store.gets)
	require.Equal(t, 1, store.puts, "a read-only request must not persist the session")
}

func TestSessionManager_TouchesAtMostOncePerInterval(t *testing.T) {
	store := &recordingSessionStore{sessionStore: newMemSessionStore()}
	m := &MoraSessionManager{
		cookiename:    "morasessionid",
		store:         store,
		lifetime:      24 * time.Hour,
		stopCh:        make(chan struct{}),
		touchInterval: time.Hour,
	}
	defer func() { _ = m.Close() }()

	// Seed a session through the store directly, as if another instance had
	// created it (this instance's lastWritten map is empty).
	sid := "pre-existing-sid"
	require.NoError(t, store.Put(sid, NewMoraSession()))

	// Two consecutive read-only requests within the interval: one touch.
	handler := m.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "morasessionid", Value: sid})
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	require.Equal(t, 1, store.touches, "last_seen must only be refreshed once per interval")

	// After the interval elapses, the next read-only request touches again.
	m.lastWritten.Store(sid, time.Now().Add(-2*m.touchInterval))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "morasessionid", Value: sid})
	handler.ServeHTTP(httptest.NewRecorder(), req)
	require.Equal(t, 2, store.touches)
}

// ----------------------------------------------------------------------
// cachingSessionStore (process-local read cache for the DB store)

func TestCachingSessionStore_PositiveCacheSkipsRepeatedReads(t *testing.T) {
	inner := newRecordingDBSessionStore(t)
	store := newCachingSessionStore(inner, time.Hour)

	// Seed the database directly (bypassing the cache) as another instance
	// would have done.
	sess := NewMoraSession()
	sess.SetUserID(7)
	require.NoError(t, inner.Put("sid-1", sess))

	got, ok := store.Get("sid-1")
	require.True(t, ok)
	require.Equal(t, int64(7), *got.UserID())
	require.Equal(t, 1, inner.gets)

	// A second read within the TTL is served from the process cache without
	// querying the database again.
	got, ok = store.Get("sid-1")
	require.True(t, ok)
	require.Equal(t, int64(7), *got.UserID())
	require.Equal(t, 1, inner.gets, "reads within the TTL must not reach the store")
}

func TestCachingSessionStore_NegativeCacheSkipsRepeatedReads(t *testing.T) {
	inner := newRecordingDBSessionStore(t)
	store := newCachingSessionStore(inner, time.Hour)

	// The first read of an unknown sid queries the database and records a
	// negative result; repeat reads are served from the cache.
	sess, ok := store.Get("sid-absent")
	require.False(t, ok)
	require.Nil(t, sess)
	sess, ok = store.Get("sid-absent")
	require.False(t, ok)
	require.Nil(t, sess)
	require.Equal(t, 1, inner.gets, "absent sids must be cached as negatives")

	// After the TTL elapses the next read re-queries the database.
	short := newCachingSessionStore(inner, time.Millisecond)
	sess, ok = short.Get("sid-absent")
	require.False(t, ok)
	require.Nil(t, sess)
	require.Equal(t, 2, inner.gets)
	time.Sleep(5 * time.Millisecond)
	sess, ok = short.Get("sid-absent")
	require.False(t, ok)
	require.Nil(t, sess)
	require.Equal(t, 3, inner.gets, "expired entries must trigger a fresh database read")
}

func TestCachingSessionStore_PutPromotesNegativeToPositive(t *testing.T) {
	inner := newRecordingDBSessionStore(t)
	store := newCachingSessionStore(inner, time.Hour)

	_, ok := store.Get("sid-1")
	require.False(t, ok)
	require.Equal(t, 1, inner.gets)

	// Persisting the session flips the negative entry into a positive one, so
	// the next read does not touch the database.
	sess := NewMoraSession()
	sess.SetUserID(3)
	require.NoError(t, store.Put("sid-1", sess))

	got, ok := store.Get("sid-1")
	require.True(t, ok)
	require.Equal(t, int64(3), *got.UserID())
	require.Equal(t, 1, inner.gets, "Put must upgrade the cache without another read")
}

func TestCachingSessionStore_DeleteInvalidatesCache(t *testing.T) {
	inner := newRecordingDBSessionStore(t)
	store := newCachingSessionStore(inner, time.Hour)

	sess := NewMoraSession()
	sess.SetUserID(5)
	require.NoError(t, inner.Put("sid-1", sess))
	_, ok := store.Get("sid-1")
	require.True(t, ok)
	require.Equal(t, 1, inner.gets)

	require.NoError(t, store.Delete("sid-1"))
	_, ok = store.Get("sid-1")
	require.False(t, ok)
	require.Equal(t, 2, inner.gets, "Delete must invalidate the cache entry")
}

func TestCachingSessionStore_DeleteExpiredSweepsCache(t *testing.T) {
	inner := newRecordingDBSessionStore(t)
	store := newCachingSessionStore(inner, time.Millisecond)

	_, ok := store.Get("sid-absent")
	require.False(t, ok)
	require.Equal(t, 1, inner.gets)

	time.Sleep(5 * time.Millisecond)
	require.NoError(t, store.DeleteExpired(time.Now()))

	_, ok = store.Get("sid-absent")
	require.False(t, ok)
	require.Equal(t, 2, inner.gets, "DeleteExpired must sweep expired cache entries")
}

func TestSessionManager_ReusesCachedSessionWithoutRepeatedGet(t *testing.T) {
	inner := newRecordingDBSessionStore(t)
	cacheStore := newCachingSessionStore(inner, time.Hour)
	m := NewMoraSessionManagerWithStore(false, cacheStore)
	defer func() { _ = m.Close() }()

	// First request (no cookie) signs the user in; the write-back persists
	// the session and warms the process cache.
	mutate := m.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := MoraSessionFrom(r.Context())
		sess.SetUserID(11)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	mutate.ServeHTTP(w, req)
	sid := w.Result().Cookies()[0].Value

	// Subsequent read-only requests with the cookie reuse the cached session
	// without reading the database again.
	read := m.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := MoraSessionFrom(r.Context())
		require.Equal(t, int64(11), *sess.UserID())
	}))
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "morasessionid", Value: sid})
		read.ServeHTTP(httptest.NewRecorder(), req)
	}

	require.Equal(t, 0, inner.gets, "session reads must be served from the process cache")
	require.Equal(t, 1, inner.puts, "only the mutating request persists the session")
}

func TestSessionManager_SkipsRepeatedGetsForAnonymousSession(t *testing.T) {
	inner := newRecordingDBSessionStore(t)
	cacheStore := newCachingSessionStore(inner, time.Hour)
	m := NewMoraSessionManagerWithStore(false, cacheStore)
	defer func() { _ = m.Close() }()

	// An anonymous visitor reloading the top page: only the first request
	// queries the database (and misses); the rest are negative cache hits.
	handler := m.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "morasessionid", Value: "anon-sid"})
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	require.Equal(t, 1, inner.gets, "only the first anonymous request reads the database")
	require.Equal(t, 0, inner.puts, "anonymous sessions are never persisted")
	require.Equal(t, 0, inner.touches, "absent sessions must not be touched")
}
