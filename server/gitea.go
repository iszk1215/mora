package server

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	driver "github.com/drone/go-scm/scm/driver/gitea"
	"github.com/drone/go-scm/scm/transport/oauth2"
)

type Gitea struct {
	BaseRepositoryManager
}

func (g *Gitea) RevisionURL(baseURL string, revision string) string {
	return baseURL + "/src/commit/" + revision
}

func defaultTransport(skipverify bool) http.RoundTripper {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipverify,
		},
	}
}

func NewGitea(id int64, serverURL, clientID, clientSecret, redirectURL string, insecureSkipVerify bool) (*Gitea, error) {
	client, err := driver.New(serverURL)
	if err != nil {
		return nil, fmt.Errorf("gitea driver.New(%s): %w", serverURL, err)
	}

	oauthCfg := OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      strings.TrimSuffix(serverURL, "/") + "/login/oauth/authorize",
		TokenURL:     strings.TrimSuffix(serverURL, "/") + "/login/oauth/access_token",
		RedirectURL:  redirectURL,
		Scopes:       []string{"repo"},
	}

	gitea := new(Gitea)
	gitea.Init(id, client.BaseURL, client, NewOAuthHandler(oauthCfg))

	gitea.client.Client = &http.Client{
		Transport: &oauth2.Transport{
			Scheme: oauth2.SchemeBearer,
			Source: &oauth2.Refresher{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Endpoint:     strings.TrimSuffix(serverURL, "/") + "/login/oauth/access_token",
				Source:       oauth2.ContextTokenSource(),
			},
			Base: defaultTransport(insecureSkipVerify),
		},
	}

	return gitea, nil
}

func NewGiteaFromFile(id int64, filename, url, redirectURL string, insecureSkipVerify bool) (*Gitea, error) {
	secret, err := readSecret(filename)
	if err != nil {
		return nil, err
	}

	return NewGitea(id, url, secret.ClientID, secret.ClientSecret, redirectURL, insecureSkipVerify)
}
