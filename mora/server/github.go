package server

import (
	"net/url"

	login "github.com/drone/go-login/login/github"
	driver "github.com/drone/go-scm/scm/driver/github"
)

type Github struct {
	BaseSCM
}

func (g *Github) RevisionURL(baseURL string, revision string) string {
	return baseURL + "/tree/" + revision
}

func NewGithub(id int64, config login.Config) *Github {
	url, _ := url.Parse("https://github.com")
	github := new(Github)
	github.Init(id, url, driver.NewDefault(), &config)

	return github
}

func NewGithubFromFile(id int64, filename string) (*Github, error) {
	secret, err := readSecret(filename)
	if err != nil {
		return nil, err
	}

	config := login.Config{
		ClientID:     secret.ClientID,
		ClientSecret: secret.ClientSecret,
		Scope:        []string{"repo"},
	}

	return NewGithub(id, config), nil
}
