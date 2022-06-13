package mora

import (
	"net/http"
	"net/url"

	login "github.com/drone/go-login/login/gitea"
	"github.com/drone/go-scm/scm"
	driver "github.com/drone/go-scm/scm/driver/gitea"
	"github.com/drone/go-scm/scm/transport/oauth2"
)

type Gitea struct {
	name   string
	client *scm.Client
	config login.Config
}

func NewGitea(name string, url string, config login.Config) (*Gitea, error) {
	gitea := new(Gitea)
	gitea.name = name
	gitea.config = config

	client, err := driver.New(url)
	if err != nil {
		return nil, err
	}

	client.Client = &http.Client{
		Transport: &oauth2.Transport{
			Scheme: "token",
			Source: oauth2.ContextTokenSource(),
		},
	}

	gitea.client = client

	return gitea, nil
}

func (gitea *Gitea) Name() string {
	return gitea.name
}

func (gitea *Gitea) URL() *url.URL {
	return gitea.client.BaseURL
}

func (g *Gitea) RevisionURL(repo *Repo, revision string) string {
	return repo.Link + "/src/commit/" + revision
}

func (g *Gitea) LoginHandler(next http.Handler) http.Handler {
	return g.config.Handler(next)
}

func (g *Gitea) GetRepos(token *scm.Token) ([]*Repo, error) {
	return getRepos(g.client, g.name, token)
}

func NewGiteaFromFile(name string, filename string, url string, redirect_url string) (*Gitea, error) {
	secret, err := readSecret(filename)
	if err != nil {
		return nil, err
	}

	config := login.Config{
		ClientID:     secret.ClientID,
		ClientSecret: secret.ClientSecret,
		Server:       url,
		RedirectURL:  redirect_url,
	}

	return NewGitea(name, url, config)
}
