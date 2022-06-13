package mora

import (
	"context"
	"net/http"

	"github.com/beego/beego/v2/server/web/session"
	"github.com/drone/drone/handler/api/render"
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
	sessionManager *session.Manager
}

func NewMoraSessionManager() (*MoraSessionManager, error) {
	s := &MoraSessionManager{}
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

	return s, nil
}

func WithMoraSession(ctx context.Context, sess *MoraSession) context.Context {
	return context.WithValue(ctx, contextMoraSessionKey, sess)
}

func MoraSessionFrom(ctx context.Context) (*MoraSession, bool) {
	sess, ok := ctx.Value(contextMoraSessionKey).(*MoraSession)
	return sess, ok
}

func (s *MoraSessionManager) SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tmp, err := s.sessionManager.SessionStart(w, r)
		if err != nil {
			panic("SessionStart returns error: " + err.Error())
		}

		sess, ok := tmp.Get(r.Context(), sessionMoraSessionKey).(*MoraSession)
		if !ok {
			log.Info().Msg("SessionMiddleware: create new MoraSession")
			sess = NewMoraSession()
			err := tmp.Set(r.Context(), sessionMoraSessionKey, sess)
			if err != nil {
				log.Err(err).Msg("")
				render.NotFound(w, render.ErrNotFound)
			}
		}

		r = r.WithContext(WithMoraSession(r.Context(), sess))
		next.ServeHTTP(w, r)

		tmp.SessionRelease(r.Context(), w)
	})
}
