package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoogle_RevisionURL(t *testing.T) {
	g, err := NewGoogle(1, "https://accounts.google.com", "clientID", "clientSecret", "https://mora.example.com/login")
	require.NoError(t, err)

	assert.Equal(t, "", g.RevisionURL("https://mora.example.com", "abc123"))
}

func TestNewGoogle_DefaultEndpoints(t *testing.T) {
	g, err := NewGoogle(1, "https://accounts.google.com", "clientID", "clientSecret", "https://mora.example.com/login")
	require.NoError(t, err)

	require.NotNil(t, g.oauthHandler)
	assert.Equal(t, "clientID", g.oauthHandler.config.ClientID)
	assert.Equal(t, "clientSecret", g.oauthHandler.config.ClientSecret)
	assert.Equal(t, "https://accounts.google.com/o/oauth2/auth", g.oauthHandler.config.Endpoint.AuthURL)
	assert.Equal(t, "https://oauth2.googleapis.com/token", g.oauthHandler.config.Endpoint.TokenURL)
	assert.Equal(t, "https://mora.example.com/login", g.oauthHandler.config.RedirectURL)
	assert.Equal(t, []string{"openid", "profile", "email"}, g.oauthHandler.config.Scopes)
	assert.Equal(t, "google", g.Driver())
}

func TestNewGoogle_CustomServerEndpoints(t *testing.T) {
	g, err := NewGoogle(1, "http://localhost:4102", "clientID", "clientSecret", "https://mora.example.com/login")
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:4102/o/oauth2/v2/auth", g.oauthHandler.config.Endpoint.AuthURL)
	assert.Equal(t, "http://localhost:4102/o/oauth2/v2/token", g.oauthHandler.config.Endpoint.TokenURL)
	assert.Equal(t, "http://localhost:4102/oauth2/v2/userinfo", googleUserInfoURL(g.URL().String()))
}

func TestGoogleEndpoints(t *testing.T) {
	authURL, tokenURL, userInfoURL := googleEndpoints("https://accounts.google.com")
	assert.Equal(t, "https://accounts.google.com/o/oauth2/auth", authURL)
	assert.Equal(t, "https://oauth2.googleapis.com/token", tokenURL)
	assert.Equal(t, "https://www.googleapis.com/oauth2/v2/userinfo", userInfoURL)

	authURL, tokenURL, userInfoURL = googleEndpoints("https://accounts.google.com/")
	assert.Equal(t, "https://accounts.google.com/o/oauth2/auth", authURL)
	assert.Equal(t, "https://oauth2.googleapis.com/token", tokenURL)
	assert.Equal(t, "https://www.googleapis.com/oauth2/v2/userinfo", userInfoURL)
}
