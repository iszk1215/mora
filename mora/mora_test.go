package mora

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/drone/go-login/login"
	"github.com/drone/go-scm/scm"
	"github.com/rs/zerolog"
)

type MockSCMClient struct {
	name         string
	url          *url.URL
	loginHandler func(http.Handler) http.Handler
	repos        []*Repo
}

func (m MockSCMClient) Name() string {
	return m.name
}

func (m MockSCMClient) URL() *url.URL {
	return m.url
}

func (m MockSCMClient) RevisionURL(repo *Repo, revision string) string {
	return path.Join(repo.Link, "revision", revision)
}

func (m MockSCMClient) LoginHandler(next http.Handler) http.Handler {
	return m.loginHandler(next)
}

func (m MockSCMClient) ListRepos(token *scm.Token) ([]*Repo, error) {
	return m.repos, nil
}

func (m *MockSCMClient) AddRepo(repos ...*Repo) {
	m.repos = append(m.repos, repos...)
}

func NewMockSCMClient(name string) *MockSCMClient {
	m := &MockSCMClient{}
	m.name = name
	m.url, _ = url.Parse(strings.Join([]string{"https://", name, ".com"}, ""))

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

type MockRepo struct {
	scm   string
	owner string
	name  string
}

func (r MockRepo) Link() string {
	return fmt.Sprintf("https://%s.com/%s/%s", r.scm, r.owner, r.name)
}

func (r MockRepo) Path() string {
	return path.Join(r.scm, r.owner, r.name)
}

/*
func (r MockRepo) RevisionURL(revision string) string {
	return path.Join(r.Link(), "revision", revision)
}
*/

func EmptyToken() scm.Token {
	return scm.Token{}
}

func NewMoraSessionWithEmptyTokenFor(names ...string) *MoraSession {
	s := NewMoraSession()
	for _, name := range names {
		s.setToken(name, EmptyToken())
	}
	return s
}

func TestMain(m *testing.M) {
	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.Disabled)
	}
	os.Exit(m.Run())
}
