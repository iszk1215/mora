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
	reposMap  map[string]map[string]Repository // [scm][owner/repo]
	tokenMap  map[string]scm.Token
	timestamp time.Time
}

func NewMoraSession() *MoraSession {
	return &MoraSession{map[string]map[string]Repository{}, map[string]scm.Token{}, time.Now()}
}

func (s *MoraSession) getReposCache(scmName string) map[string]Repository {
	return s.reposMap[scmName]
}

func (s *MoraSession) setReposCache(scmName string, repos map[string]Repository) {
	s.reposMap[scmName] = repos
}

func (s *MoraSession) getToken(scmName string) (scm.Token, bool) {
	token, ok := s.tokenMap[scmName]
	return token, ok
}

func (s *MoraSession) setToken(scmName string, token scm.Token) {
	s.tokenMap[scmName] = token
}

func (s *MoraSession) Remove(scmName string) {
	delete(s.tokenMap, scmName)
	delete(s.reposMap, scmName)
}

func (s *MoraSession) WithToken(ctx context.Context, name string) (context.Context, error) {
	token, ok := s.getToken(name)
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
