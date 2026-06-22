package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"time"

	"github.com/drone/go-scm/scm"
	"golang.org/x/oauth2"
)

type oauthContextKey int

const (
	oauthErrorKey oauthContextKey = iota
)

func withOAuthError(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, oauthErrorKey, err)
}

func oauthErrorFrom(ctx context.Context) error {
	err, _ := ctx.Value(oauthErrorKey).(error)
	return err
}

func tokenFromContext(ctx context.Context) (*scm.Token, bool) {
	token, ok := ctx.Value(scm.TokenKey{}).(*scm.Token)
	return token, ok
}

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	RedirectURL  string
	Scopes       []string
}

type OAuthHandler struct {
	config *oauth2.Config
}

func NewOAuthHandler(cfg OAuthConfig) *OAuthHandler {
	return &OAuthHandler{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  cfg.AuthURL,
				TokenURL: cfg.TokenURL,
			},
			RedirectURL: cfg.RedirectURL,
			Scopes:      cfg.Scopes,
		},
	}
}

func (h *OAuthHandler) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if errStr := r.URL.Query().Get("error"); errStr != "" {
			ctx := withOAuthError(r.Context(), fmt.Errorf("oauth2 error: %s", errStr))
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			state, err := generateCSRFToken()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     "oauth_state",
				Value:    state,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   int((10 * time.Minute).Seconds()),
				Secure:   r.TLS != nil,
			})
			http.Redirect(w, r, h.config.AuthCodeURL(state), http.StatusFound)
			return
		}

		state := r.URL.Query().Get("state")
		cookie, err := r.Cookie("oauth_state")
		if err != nil {
			ctx := withOAuthError(r.Context(), fmt.Errorf("state cookie not found"))
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) != 1 {
			ctx := withOAuthError(r.Context(), fmt.Errorf("state mismatch"))
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		oauth2Token, err := h.config.Exchange(r.Context(), code)
		if err != nil {
			ctx := withOAuthError(r.Context(), fmt.Errorf("token exchange: %w", err))
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		token := &scm.Token{
			Token:   oauth2Token.AccessToken,
			Refresh: oauth2Token.RefreshToken,
			Expires: oauth2Token.Expiry,
		}

		ctx := scm.WithContext(r.Context(), token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
