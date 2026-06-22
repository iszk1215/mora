package server

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/drone/go-scm/scm"
	"github.com/drone/go-scm/scm/transport/oauth2"
	"github.com/pelletier/go-toml/v2"
)

type BaseRepositoryManager struct {
	id           int64
	client       *scm.Client
	url          *url.URL
	oauthHandler *OAuthHandler
}

func (s *BaseRepositoryManager) Init(id int64, url *url.URL, client *scm.Client,
	oauthHandler *OAuthHandler) {
	s.id = id
	s.url = url
	s.client = client
	s.oauthHandler = oauthHandler
}

func (s *BaseRepositoryManager) SetupTransport(source scm.TokenSource, scheme string) {
	s.client.Client = &http.Client{
		Transport: &oauth2.Transport{
			Scheme: scheme,
			Source: source,
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
	return s.oauthHandler.Handler(next)
}

type secret struct {
	ClientID     string `toml:"ClientID"`
	ClientSecret string `toml:"ClientSecret"`
}

func readSecret(filename string) (secret, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return secret{}, fmt.Errorf("readSecret(%s): %w", filename, err)
	}

	s := secret{}
	err = toml.Unmarshal(b, &s)
	if err != nil {
		return secret{}, fmt.Errorf("readSecret Unmarshal: %w", err)
	}

	return s, nil
}
