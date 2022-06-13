package mora

import (
	"net/http"
	"net/url"

	login "github.com/drone/go-login/login/github"
	driver "github.com/drone/go-scm/scm/driver/github"
)

type Github struct {
	BaseSCM
	config login.Config
}

func NewGithub(name string, config login.Config) *Github {
	github := new(Github)
	url, _ := url.Parse("https://github.com")
	github.Init(name, driver.NewDefault(), url)
	github.config = config

	return github
}

func (g *Github) RevisionURL(repo *Repo, revision string) string {
	return repo.Link + "/tree/" + revision
}

func (g *Github) LoginHandler(next http.Handler) http.Handler {
	return g.config.Handler(next)
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
