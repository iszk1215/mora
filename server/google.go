package server

import (
	"fmt"
	"net/url"
	"strings"
)

type Google struct {
	BaseRepositoryManager
}

func googleEndpoints(serverURL string) (authURL, tokenURL, userInfoURL string) {
	if strings.Contains(serverURL, "accounts.google.com") {
		return "https://accounts.google.com/o/oauth2/auth",
			"https://oauth2.googleapis.com/token",
			"https://www.googleapis.com/oauth2/v2/userinfo"
	}

	base := strings.TrimSuffix(serverURL, "/")
	return base + "/o/oauth2/v2/auth",
		base + "/o/oauth2/v2/token",
		base + "/oauth2/v2/userinfo"
}

func googleUserInfoURL(serverURL string) string {
	_, _, userInfoURL := googleEndpoints(serverURL)
	return userInfoURL
}

func (g *Google) RevisionURL(baseURL string, revision string) string {
	return ""
}

func NewGoogle(id int64, serverURL, clientID, clientSecret, redirectURL string) (*Google, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("NewGoogle: invalid URL %q: %w", serverURL, err)
	}

	authURL, tokenURL, _ := googleEndpoints(serverURL)

	oauthCfg := OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      authURL,
		TokenURL:     tokenURL,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "profile", "email"},
	}

	google := new(Google)
	google.Init(id, u, nil, NewOAuthHandler(oauthCfg), "google")

	return google, nil
}

func NewGoogleFromFile(id int64, url, filename, redirectURL string) (*Google, error) {
	secret, err := readSecret(filename)
	if err != nil {
		return nil, err
	}

	return NewGoogle(id, url, secret.ClientID, secret.ClientSecret, redirectURL)
}
