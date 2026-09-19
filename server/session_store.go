package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	if err != nil {
		return fmt.Errorf("session DeleteExpired: %w", err)
	}
	log.Debug().Stringer("duration", time.Since(start)).Msg("session DeleteExpired")
	return nil
}

func (s *dbSessionStore) Len() int {
	var n int
	if err := s.db.Get(&n, "SELECT COUNT(*) FROM session"); err != nil {
		return 0
	}
	return n
}
