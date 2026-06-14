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
	lock sync.Mutex

	reposMap    map[int64]map[int64]bool // [rmID][repoID]
	tokenMap    map[int64]scm.Token      // [rmID]
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

func (s *MoraSession) getReposCache(rmID int64) map[int64]bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.reposMap[rmID]
}

func (s *MoraSession) setReposCache(rmID int64, repos map[int64]bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.reposMap[rmID] = repos
}

func (s *MoraSession) getToken(rmID int64) (scm.Token, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	token, ok := s.tokenMap[rmID]
	return token, ok
}

func (s *MoraSession) setToken(rmID int64, token scm.Token) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.tokenMap[rmID] = token
}

func (s *MoraSession) Remove(rmID int64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.tokenMap, rmID)
	delete(s.reposMap, rmID)
}

func (s *MoraSession) WithToken(ctx context.Context, rmID int64) (context.Context, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	token, ok := s.tokenMap[rmID]
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
		lifetime:   24 * time.Hour,
	}
}

func WithMoraSession(ctx context.Context, sess *MoraSession) context.Context {
	return context.WithValue(ctx, contextMoraSessionKey, sess)
}

func MoraSessionFrom(ctx context.Context) (*MoraSession, bool) {
	sess, ok := ctx.Value(contextMoraSessionKey).(*MoraSession)
	return sess, ok
}

func sessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (m *MoraSessionManager) GC() {
	m.lock.Lock()
	defer m.lock.Unlock()

	now := time.Now()
	for sid, sess := range m.store {
		sess.lock.Lock()
		expired := now.Sub(sess.timestamp) > m.lifetime
		sess.lock.Unlock()
		if expired {
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
			sid, err = sessionID()
			if err != nil {
				log.Err(err).Msg("failed to generate session ID")
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		} else {
			sid = cookie.Value
		}

		sess, ok := m.get(sid)
		if !ok {
			log.Info().Msgf("SessionMiddleware: create new MoraSession")
			sess = NewMoraSession()
			m.put(sid, sess)
		}
		sess.lock.Lock()
		sess.timestamp = time.Now()
		sess.lock.Unlock()

		cookie = &http.Cookie{
			Name:     m.cookiename,
			Value:    sid,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		}

		http.SetCookie(w, cookie)

		r = r.WithContext(WithMoraSession(r.Context(), sess))
		next.ServeHTTP(w, r)
	})
}
