package server

import (
	"errors"
	"flag"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/drone/go-login/login"
	"github.com/drone/go-scm/scm"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type MockSCM struct {
	name         string
	url          *url.URL
	loginHandler func(http.Handler) http.Handler
	client       *scm.Client
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
	return path.Join(baseURL, "revision", revision)
}

func (m *MockSCM) LoginHandler(next http.Handler) http.Handler {
	return m.loginHandler(next)
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

func NewMoraSessionWithTokenFor(names ...string) *MoraSession {
	s := NewMoraSession()
	for _, name := range names {
		s.setToken(name, scm.Token{})
	}
	return s
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
