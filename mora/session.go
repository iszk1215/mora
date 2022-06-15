package mora

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"

	"github.com/drone/go-scm/scm"
	"github.com/rs/zerolog/log"
)

type moraSessionKey int

const (
	sessionMoraSessionKey moraSessionKey = iota
	contextMoraSessionKey moraSessionKey = iota
)

type MoraSession struct {
	reposMap map[string][]*Repo
	tokenMap map[string]scm.Token
}

func NewMoraSession() *MoraSession {
	return &MoraSession{map[string][]*Repo{}, map[string]scm.Token{}}
}

func (s *MoraSession) getReposCache(scm string) ([]*Repo, bool) {
	repos, ok := s.reposMap[scm]
	return repos, ok
}

func (s *MoraSession) setReposCache(scm string, repos []*Repo) {
	s.reposMap[scm] = repos
}

func (s *MoraSession) getToken(scm string) (scm.Token, bool) {
	token, ok := s.tokenMap[scm]
	return token, ok
}

func (s *MoraSession) setToken(scm string, token scm.Token) {
	s.tokenMap[scm] = token
}

func (s *MoraSession) Remove(scm string) {
	delete(s.tokenMap, scm)
	delete(s.reposMap, scm)
}

type MoraSessionManager struct {
	cookiename string
	store      map[string]*MoraSession
}

func NewMoraSessionManager() (*MoraSessionManager, error) {
	s := &MoraSessionManager{}
	s.store = map[string]*MoraSession{}
	s.cookiename = "morasessionid"
	/*
		conf := &session.ManagerConfig{
			CookieName:      "morasessionid",
			Gclifetime:      3600 * 24 * 7,
			EnableSetCookie: true,
		}
		m, err := session.NewManager("memory", conf)
		if err != nil {
			return nil, err
		}
		go m.GC()
		s.sessionManager = m
	*/

	return s, nil
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

func (m *MoraSessionManager) SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(m.cookiename)

		var sid string
		if err != nil || cookie.Value == "" {
			sid = sessionID()
		} else {
			sid = cookie.Value
		}

		sess, ok := m.store[sid]
		if !ok {
			log.Info().Msg("SessionMiddleware: create new MoraSession")
			sess := NewMoraSession()
			m.store[sid] = sess
		}

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
