package server

import (
	"errors"
	"flag"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/drone/go-login/login"
	"github.com/drone/go-scm/scm"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type MockRepositoryManager struct {
	id           int64
	url          *url.URL
	loginHandler func(http.Handler) http.Handler
	client       *scm.Client
}

func (m *MockRepositoryManager) ID() int64 {
	return m.id
}

func (m *MockRepositoryManager) Client() *scm.Client {
	return m.client
}

func (m *MockRepositoryManager) URL() *url.URL {
	return m.url
}

func (m *MockRepositoryManager) RevisionURL(baseURL string, revision string) string {
	joined, _ := url.JoinPath(baseURL, "revision", revision)
	return joined
}

func (m *MockRepositoryManager) LoginHandler(next http.Handler) http.Handler {
	return m.loginHandler(next)
}

func NewMockRepositoryManager(id int64) *MockRepositoryManager {
	m := &MockRepositoryManager{id: id}
	m.url, _ = url.Parse(strings.Join([]string{"https://mock.scm"}, ""))

	m.client = &scm.Client{}

	// default login handler always returns error
	m.loginHandler = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := errors.New("login error(mock)")
			ctx := login.WithError(r.Context(), err)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	return m
}

func NewMoraSessionWithTokenFor(repositoryManagers ...RepositoryManager) *MoraSession {
	sess := NewMoraSession()
	for _, m := range repositoryManagers {
		sess.setToken(m.ID(), scm.Token{})
	}
	return sess
}

func TestMain(m *testing.M) {
	log.Logger = log.Output(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Caller().Logger()

	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.Disabled)
	}
	os.Exit(m.Run())
}
