package mora

import (
	"net/http"
	"net/url"

	login "github.com/drone/go-login/login/github"
	"github.com/drone/go-scm/scm"
	driver "github.com/drone/go-scm/scm/driver/github"
	"github.com/drone/go-scm/scm/transport/oauth2"
)

type Github struct {
	name   string
	url    *url.URL
	client *scm.Client
	config login.Config
}

func NewGithub(name string, config login.Config) *Github {
	github := new(Github)
	github.name = name
	github.url, _ = url.Parse("https://github.com")
	github.config = config

	client := driver.NewDefault()

	client.Client = &http.Client{
		Transport: &oauth2.Transport{
			Scheme: "token",
			Source: oauth2.ContextTokenSource(),
		},
	}

	github.client = client

	return github
}

func (g *Github) Name() string {
	return g.name
}

func (g *Github) URL() *url.URL {
	return g.url
}

func (g *Github) RevisionURL(repo *Repo, revision string) string {
	return repo.Link + "/tree/" + revision
}

func (g *Github) LoginHandler(next http.Handler) http.Handler {
	return g.config.Handler(next)
}

func (g *Github) GetRepos(token *scm.Token) ([]*Repo, error) {
	return getRepos(g.client, g.name, token)
}

func NewGithubFromFile(name string, filename string) (*Github, error) {
	secret, err := readSecret(filename)
	if err != nil {
		return nil, err
	}

	config := login.Config{
		ClientID:     secret.ClientID,
		ClientSecret: secret.ClientSecret,
		Scope:        []string{"repo"},
	}

	return NewGithub(name, config), nil
}
