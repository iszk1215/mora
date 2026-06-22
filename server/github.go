package server

import (
	"net/http"
	"net/url"

	driver "github.com/drone/go-scm/scm/driver/github"
	"github.com/drone/go-scm/scm/transport/oauth2"
)

type Github struct {
	BaseRepositoryManager
}

func (g *Github) RevisionURL(baseURL string, revision string) string {
	return baseURL + "/tree/" + revision
}

func NewGithub(id int64, urlstr string, clientID, clientSecret, redirectURL string) *Github {
	u, _ := url.Parse(urlstr)

	oauthCfg := OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		RedirectURL:  redirectURL,
		Scopes:       []string{"repo"},
	}

	github := new(Github)
	github.Init(id, u, driver.NewDefault(), NewOAuthHandler(oauthCfg))

	github.SetupTransport(
		&oauth2.Refresher{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     "https://github.com/login/oauth/access_token",
			Source:       oauth2.ContextTokenSource(),
			Client:       http.DefaultClient,
		},
		oauth2.SchemeBearer,
	)

	return github
}

func NewGithubFromFile(id int64, url, filename, redirectURL string) (*Github, error) {
	secret, err := readSecret(filename)
	if err != nil {
		return nil, err
	}

	return NewGithub(id, url, secret.ClientID, secret.ClientSecret, redirectURL), nil
}
