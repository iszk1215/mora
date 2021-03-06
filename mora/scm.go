package mora

import (
	"net/http"
	"net/url"
	"os"

	"github.com/drone/go-login/login"
	"github.com/drone/go-scm/scm"
	"github.com/drone/go-scm/scm/transport/oauth2"
	"github.com/pelletier/go-toml/v2"
)

type BaseSCM struct {
	name            string
	client          *scm.Client
	url             *url.URL
	loginMiddleware login.Middleware
}

func (s *BaseSCM) Init(name string, url *url.URL, client *scm.Client,
	loginMiddleware login.Middleware) {
	s.name = name
	s.url = url
	s.client = client
	s.loginMiddleware = loginMiddleware

	s.client.Client = &http.Client{
		Transport: &oauth2.Transport{
			Scheme: "token",
			Source: oauth2.ContextTokenSource(),
		},
	}
}

func (s *BaseSCM) Name() string {
	return s.name
}

func (s *BaseSCM) Client() *scm.Client {
	return s.client
}

func (s *BaseSCM) URL() *url.URL {
	return s.url
}

func (s *BaseSCM) LoginHandler(next http.Handler) http.Handler {
	return s.loginMiddleware.Handler(next)
}

type secret struct {
	ClientID     string `yaml:"ClientID"`
	ClientSecret string `yaml:"ClientSecret"`
}

func readSecret(filename string) (secret, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return secret{}, err
	}

	s := secret{}
	err = toml.Unmarshal(b, &s)
	if err != nil {
		return secret{}, err
	}

	return s, nil
}
