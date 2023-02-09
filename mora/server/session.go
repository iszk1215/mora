package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/drone/go-scm/scm"
	"github.com/rs/zerolog/log"
)

type moraSessionKey int

const (
	sessionMoraSessionKey moraSessionKey = iota
	contextMoraSessionKey moraSessionKey = iota
)

type MoraSession struct {
	reposMap    map[int64]map[int64]bool // [scm][owner/repo]
	tokenMap    map[int64]scm.Token
	timestamp   time.Time
	loggingInto int64
}

func NewMoraSession() *MoraSession {
	return &MoraSession{
		reposMap:    map[int64]map[int64]bool{},
		tokenMap:    map[int64]scm.Token{},
		timestamp:   time.Now(),
		loggingInto: -1,
	}
}

func (s *MoraSession) getReposCache(scmID int64) map[int64]bool {
	return s.reposMap[scmID]
}

func (s *MoraSession) setReposCache(scmID int64, repos map[int64]bool) {
	s.reposMap[scmID] = repos
}

func (s *MoraSession) getToken(scmID int64) (scm.Token, bool) {
	token, ok := s.tokenMap[scmID]
	return token, ok
}

func (s *MoraSession) setToken(scmID int64, token scm.Token) {
	s.tokenMap[scmID] = token
}

func (s *MoraSession) Remove(scmID int64) {
	delete(s.tokenMap, scmID)
	delete(s.reposMap, scmID)
}

func (s *MoraSession) WithToken(ctx context.Context, scmID int64) (context.Context, error) {
	token, ok := s.getToken(scmID)
	if !ok {
		return nil, errorTokenNotFound
	}

	return scm.WithContext(ctx, &token), nil
}

// Session Manager

type MoraSessionManager struct {
	cookiename string
	store      map[string]*MoraSession
	lifetime   time.Duration
	lock       sync.Mutex
}

func NewMoraSessionManager() *MoraSessionManager {
	return &MoraSessionManager{
		cookiename: "morasessionid",
		store:      map[string]*MoraSession{},
		lifetime:   3600 * 24 * time.Hour,
	}
}

func WithMoraSession(ctx context.Context, sess *MoraSession) context.Context {
	return context.WithValue(ctx, contextMoraSessionKey, sess)
}

func MoraSessionFrom(ctx context.Context) (*MoraSession, bool) {
	sess, ok := ctx.Value(contextMoraSessionKey).(*MoraSession)
	return sess, ok
}

func sessionID() string {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

func (m *MoraSessionManager) GC() {
	m.lock.Lock()
	defer m.lock.Unlock()

	now := time.Now()
	for sid, sess := range m.store {
		if now.Sub(sess.timestamp) > m.lifetime {
			delete(m.store, sid)
		}
	}
}

func (m *MoraSessionManager) get(sid string) (*MoraSession, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	sess, ok := m.store[sid]
	return sess, ok
}

func (m *MoraSessionManager) put(sid string, session *MoraSession) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.store[sid] = session
}

func (m *MoraSessionManager) SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.GC()

		cookie, err := r.Cookie(m.cookiename)

		var sid string
		if err != nil || cookie.Value == "" {
			sid = sessionID()
		} else {
			sid = cookie.Value
		}

		sess, ok := m.get(sid)
		if !ok {
			log.Info().Msgf("SessionMiddleware: create new MoraSession")
			sess = NewMoraSession()
			m.put(sid, sess)
		}
		sess.timestamp = time.Now()

		cookie = &http.Cookie{
			Name:     m.cookiename,
			Value:    sid,
			Path:     "/",
			HttpOnly: true,
		}

		http.SetCookie(w, cookie)

		r = r.WithContext(WithMoraSession(r.Context(), sess))
		next.ServeHTTP(w, r)
	})
}
