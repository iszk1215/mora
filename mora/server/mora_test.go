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

type MockSCM struct {
	id           int64
	name         string
	url          *url.URL
	loginHandler func(http.Handler) http.Handler
	client       *scm.Client
}

func (m *MockSCM) ID() int64 {
	return m.id
}

func (m *MockSCM) Name() string {
	return m.name
}

func (m *MockSCM) Client() *scm.Client {
	return m.client
}

func (m *MockSCM) URL() *url.URL {
	return m.url
}

func (m *MockSCM) RevisionURL(baseURL string, revision string) string {
	joined, _ := url.JoinPath(baseURL, "revision", revision)
	return joined
}

func (m *MockSCM) LoginHandler(next http.Handler) http.Handler {
	return m.loginHandler(next)
}

func NewMockSCMWithID(id int64, name string) *MockSCM {
	m := NewMockSCM(name)
	m.id = id
	return m
}

func NewMockSCM(name string) *MockSCM {
	m := &MockSCM{}
	m.name = name
	m.url, _ = url.Parse(strings.Join([]string{"https://", name, ".com"}, ""))

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

func NewMoraSessionWithTokenFor(scms ...SCM) *MoraSession {
	sess := NewMoraSession()
	for _, s := range scms {
		sess.setToken(s.ID(), scm.Token{})
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
