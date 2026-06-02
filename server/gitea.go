package server

import (
	"crypto/tls"
	"net/http"
	"strings"

	login "github.com/drone/go-login/login/gitea"
	driver "github.com/drone/go-scm/scm/driver/gitea"
	"github.com/drone/go-scm/scm/transport/oauth2"
)

type Gitea struct {
	BaseRepositoryManager
}

func (g *Gitea) RevisionURL(baseURL string, revision string) string {
	return baseURL + "/src/commit/" + revision
}

// from drone
func defaultTransport(skipverify bool) http.RoundTripper {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipverify,
		},
	}
}

func NewGitea(id int64, url string, config login.Config) (*Gitea, error) {
	client, err := driver.New(url)
	if err != nil {
		return nil, err
	}

	gitea := new(Gitea)
	gitea.Init(id, client.BaseURL, client, &config)

	gitea.client.Client = &http.Client{
		Transport: &oauth2.Transport{
			Scheme: oauth2.SchemeBearer,
			Source: &oauth2.Refresher{
				ClientID:     config.ClientID,
				ClientSecret: config.ClientSecret,
				Endpoint:     strings.TrimSuffix(url, "/") + "/login/oauth/access_token",
				Source:       oauth2.ContextTokenSource(),
			},
			Base: defaultTransport( /*config.SkipVerify*/ false),
		},
	}
	return gitea, nil
}

func NewGiteaFromFile(id int64, filename string, url string, redirect_url string) (*Gitea, error) {
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

	return NewGitea(id, url, config)
}
