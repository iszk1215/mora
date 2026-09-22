package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

const schema_session = `
CREATE TABLE IF NOT EXISTS session (
    sid       TEXT    PRIMARY KEY,
    data      TEXT    NOT NULL,
    last_seen INTEGER NOT NULL
)`

// sessionCacheTTL bounds how long a session snapshot may be served from the
// process-local cache without re-reading the shared database. The TTL is fixed
// (not refreshed by reads), so cross-instance staleness stays within one TTL
// and reload bursts are collapsed to a single database read.
const sessionCacheTTL = 5 * time.Second

// sessionStore persists MoraSession objects keyed by session ID.
//
// The in-memory implementation backs unit tests and single-process demo mode.
// The database implementation keeps sessions across Cloud Run instances that
// share the same (Turso/libSQL) database.
type sessionStore interface {
	Get(sid string) (*MoraSession, bool)
	Put(sid string, sess *MoraSession) error
	Delete(sid string) error
	Has(sid string) bool
	DeleteExpired(before time.Time) error
	// Touch refreshes only the last_seen timestamp of an existing session,
	// without rewriting the stored session data (used to keep active sessions
	// alive for GC on read-only requests).
	Touch(sid string, at time.Time) error
	Len() int
}

// memSessionStore keeps sessions in a process-local map. Sessions do not
// survive process restarts, matching the historical single-instance behavior.
type memSessionStore struct {
	lock  sync.Mutex
	store map[string]*MoraSession
}

func newMemSessionStore() *memSessionStore {
	return &memSessionStore{store: map[string]*MoraSession{}}
}

func (s *memSessionStore) Get(sid string) (*MoraSession, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	sess, ok := s.store[sid]
	return sess, ok
}

func (s *memSessionStore) Put(sid string, sess *MoraSession) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.store[sid] = sess
	return nil
}

func (s *memSessionStore) Delete(sid string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.store, sid)
	return nil
}

func (s *memSessionStore) Has(sid string) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, ok := s.store[sid]
	return ok
}

func (s *memSessionStore) DeleteExpired(before time.Time) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	for sid, sess := range s.store {
		sess.lock.Lock()
		expired := sess.timestamp.Before(before)
		sess.lock.Unlock()
		if expired {
			delete(s.store, sid)
		}
	}
	return nil
}

// Touch is a no-op for the in-memory store; the shared session object keeps
// its own up-to-date timestamp and expiry is evaluated against it.
func (s *memSessionStore) Touch(sid string, at time.Time) error {
	return nil
}

func (s *memSessionStore) Len() int {
	s.lock.Lock()
	defer s.lock.Unlock()
	return len(s.store)
}

// dbSessionStore persists sessions in the shared database, so every Cloud Run
// instance sees the same sessions. Data is a JSON snapshot of the session and
// last_seen is an integer nanoseconds-since-epoch timestamp used for GC.
type dbSessionStore struct {
	db *sqlx.DB
}

func newDBSessionStore(db *sqlx.DB) *dbSessionStore {
	return &dbSessionStore{db: db}
}

func (s *dbSessionStore) Init() error {
	_, err := s.db.Exec(schema_session)
	if err != nil {
		log.Err(err).Msg("session store init failed")
		return fmt.Errorf("session store Init: %w", err)
	}
	return nil
}

func (s *dbSessionStore) Get(sid string) (*MoraSession, bool) {
	start := time.Now()
	var data string
	err := s.db.Get(&data, "SELECT data FROM session WHERE sid = ?", sid)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Err(err).Str("sid", sid).Msg("session Get failed")
		}
		log.Debug().Str("sid", sid).Stringer("duration", time.Since(start)).Msg("session Get (miss)")
		return nil, false
	}

	sess := NewMoraSession()
	if err := json.Unmarshal([]byte(data), sess); err != nil {
		log.Err(err).Str("sid", sid).Msg("session Get: unmarshal failed")
		_ = s.Delete(sid)
		return nil, false
	}
	log.Debug().Str("sid", sid).Stringer("duration", time.Since(start)).Msg("session Get (hit)")
	return sess, true
}

func (s *dbSessionStore) Put(sid string, sess *MoraSession) error {
	start := time.Now()
	data, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("session Put marshal: %w", err)
	}

	sess.lock.Lock()
	lastSeen := sess.timestamp.UnixNano()
	sess.lock.Unlock()

	_, err = s.db.Exec(`
		INSERT INTO session (sid, data, last_seen) VALUES (?, ?, ?)
		ON CONFLICT(sid) DO UPDATE SET data = excluded.data, last_seen = excluded.last_seen`,
		sid, string(data), lastSeen)
	if err != nil {
		return fmt.Errorf("session Put: %w", err)
	}
	log.Debug().Str("sid", sid).Int("bytes", len(data)).Stringer("duration", time.Since(start)).Msg("session Put")
	return nil
}

// Touch refreshes only the last_seen column, skipping the session data
// rewrite. A missing session (already GC'd) is a no-op.
func (s *dbSessionStore) Touch(sid string, at time.Time) error {
	start := time.Now()
	_, err := s.db.Exec("UPDATE session SET last_seen = ? WHERE sid = ?", at.UnixNano(), sid)
	if err != nil {
		return fmt.Errorf("session Touch: %w", err)
	}
	log.Debug().Str("sid", sid).Stringer("duration", time.Since(start)).Msg("session Touch")
	return nil
}

func (s *dbSessionStore) Delete(sid string) error {
	start := time.Now()
	_, err := s.db.Exec("DELETE FROM session WHERE sid = ?", sid)
	if err != nil {
		return fmt.Errorf("session Delete: %w", err)
	}
	log.Debug().Str("sid", sid).Stringer("duration", time.Since(start)).Msg("session Delete")
	return nil
}

func (s *dbSessionStore) Has(sid string) bool {
	start := time.Now()
	var one int
	err := s.db.Get(&one, "SELECT 1 FROM session WHERE sid = ?", sid)
	log.Debug().Str("sid", sid).Stringer("duration", time.Since(start)).Msg("session Has")
	return err == nil
}

func (s *dbSessionStore) DeleteExpired(before time.Time) error {
	start := time.Now()
	cutoff := before.UnixNano()

	_, err := s.db.Exec("DELETE FROM session WHERE last_seen <= ?", cutoff)
	if err != nil && isHranaStreamError(err) {
		// The failing query hit a stale Hrana stream left over in the pool.
		// The go-libsql driver reports it as a plain error (not
		// driver.ErrBadConn), so database/sql would hand the same broken
		// connection back and keep failing. Drop idle connections and retry
		// once so the next Exec establishes a fresh Hrana stream.
		log.Warn().Err(err).Msg("session DeleteExpired: Hrana stream error, resetting pool and retrying")
		resetIdlePool(s.db)
		if _, retryErr := s.db.Exec("DELETE FROM session WHERE last_seen <= ?", cutoff); retryErr != nil {
			return fmt.Errorf("session DeleteExpired: %w", retryErr)
		}
	} else if err != nil {
		return fmt.Errorf("session DeleteExpired: %w", err)
	}

	log.Debug().Stringer("duration", time.Since(start)).Msg("session DeleteExpired")
	return nil
}

// resetIdlePool closes every idle pooled connection so the next query opens a
// new one with a live Hrana stream. Connections currently in use are left
// untouched; they are recycled by the pool's max idle/lifetime settings.
//
// It is a variable so tests can stub the reset while still exercising the
// retry flow, avoiding driver lifecycle quirks (sqlmock unregisters its DSN
// when its last connection is closed).
var resetIdlePool = func(db *sqlx.DB) {
	db.SetMaxIdleConns(0)
	db.SetMaxIdleConns(defaultMaxIdleConns)
}

// isHranaStreamError reports whether err is a Turso/libSQL Hrana protocol
// failure caused by a stream being closed or expired server-side. Such errors
// are transient: the affected pooled connection is stale and should be
// replaced, after which the operation can be retried.
func isHranaStreamError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, marker := range []string{
		"stream not found",
		"stream has expired",
		"HRANA_CLOSED",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func (s *dbSessionStore) Len() int {
	var n int
	if err := s.db.Get(&n, "SELECT COUNT(*) FROM session"); err != nil {
		return 0
	}
	return n
}

// cachedSession is a single entry in cachingSessionStore. sess is nil for
// negative entries, which remember that a sid had no backing row so a reload
// burst does not query the shared database for a row that does not exist.
type cachedSession struct {
	sess    *MoraSession
	expires time.Time
}

// cachingSessionStore decorates a sessionStore with a short-lived
// process-local read cache. Only the database-backed store is wrapped: every
// cookie-bearing request would otherwise pay a full remote round trip (about
// 350ms on Turso) even when the request lands on the same instance that just
// served or missed that session milliseconds ago.
//
// Positive hits reuse the cached *MoraSession object. This is safe because
// the session middleware keeps requests for the same sid concurrent and every
// access to a *MoraSession is guarded by the object's own lock; only the load
// and the write-back are serialized per session (via lockSession), and the
// write-back marshals its snapshot under the object's lock. The shared
// database remains the source of truth; the cache only shortcuts reads within
// the TTL.
//
// Negative entries make the middleware build a fresh anonymous session on each
// hit (exactly as it does today), but skip the database round trip that would
// otherwise miss every time. Both entry kinds expire after the fixed TTL and
// are swept by DeleteExpired.
type cachingSessionStore struct {
	inner sessionStore
	mu    sync.Mutex
	cache map[string]cachedSession
	ttl   time.Duration
}

func newCachingSessionStore(inner sessionStore, ttl time.Duration) *cachingSessionStore {
	return &cachingSessionStore{
		inner: inner,
		cache: map[string]cachedSession{},
		ttl:   ttl,
	}
}

func (s *cachingSessionStore) Get(sid string) (*MoraSession, bool) {
	now := time.Now()
	s.mu.Lock()
	if e, ok := s.cache[sid]; ok {
		if now.Before(e.expires) {
			if e.sess == nil {
				s.mu.Unlock()
				return nil, false
			}
			sess := e.sess
			s.mu.Unlock()
			return sess, true
		}
		delete(s.cache, sid)
	}
	s.mu.Unlock()

	sess, found := s.inner.Get(sid)
	if !found {
		// Negative cache: remember that the sid does not exist so a burst of
		// read-only request does not keep querying a row that is absent.
		sess = nil
	}
	s.mu.Lock()
	s.cache[sid] = cachedSession{sess: sess, expires: time.Now().Add(s.ttl)}
	s.mu.Unlock()
	return sess, found
}

func (s *cachingSessionStore) Put(sid string, sess *MoraSession) error {
	if err := s.inner.Put(sid, sess); err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[sid] = cachedSession{sess: sess, expires: time.Now().Add(s.ttl)}
	s.mu.Unlock()
	return nil
}

func (s *cachingSessionStore) Touch(sid string, at time.Time) error {
	return s.inner.Touch(sid, at)
}

func (s *cachingSessionStore) Delete(sid string) error {
	if err := s.inner.Delete(sid); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.cache, sid)
	s.mu.Unlock()
	return nil
}

func (s *cachingSessionStore) Has(sid string) bool {
	return s.inner.Has(sid)
}

func (s *cachingSessionStore) DeleteExpired(before time.Time) error {
	if err := s.inner.DeleteExpired(before); err != nil {
		return err
	}
	now := time.Now()
	s.mu.Lock()
	for sid, e := range s.cache {
		if !now.Before(e.expires) {
			delete(s.cache, sid)
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *cachingSessionStore) Len() int {
	return s.inner.Len()
}
