package server

import (
	"net/http"
	"net/url"
	"os"

	"github.com/drone/go-login/login"
	"github.com/drone/go-scm/scm"
	"github.com/drone/go-scm/scm/transport/oauth2"
	"github.com/pelletier/go-toml/v2"
)

type BaseRepositoryManager struct {
	id              int64
	client          *scm.Client
	url             *url.URL
	loginMiddleware login.Middleware
}

func (s *BaseRepositoryManager) Init(id int64, url *url.URL, client *scm.Client,
	loginMiddleware login.Middleware) {
	s.id = id
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

func (s *BaseRepositoryManager) ID() int64 {
	return s.id
}

func (s *BaseRepositoryManager) Client() *scm.Client {
	return s.client
}

func (s *BaseRepositoryManager) URL() *url.URL {
	return s.url
}

func (s *BaseRepositoryManager) LoginHandler(next http.Handler) http.Handler {
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
