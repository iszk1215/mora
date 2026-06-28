package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/render"
	"github.com/rs/zerolog/log"
)

const csrfCookieName = "csrf_token"

func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("generateCSRFToken: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func verifyCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return false
	}
	bodyToken := r.FormValue(csrfCookieName)
	if bodyToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(bodyToken)) == 1
}

func createLoginHandler(rm RepositoryManager, userStore UserStore, next http.Handler) http.Handler {
	h := func(w http.ResponseWriter, r *http.Request) {
		err := oauthErrorFrom(r.Context())
		if err != nil {
			log.Error().Err(err).Msg("oauth error in login handler")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		token, ok := tokenFromContext(r.Context())
		if !ok {
			log.Error().Msg("No token found in context")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		sess, ok := MoraSessionFrom(r.Context())
		if !ok {
			log.Error().Msg("No session found in context")
			render.NotFound(w, render.ErrNotFound)
			return
		}
		sess.setToken(rm.ID(), *token)

		if userStore != nil {
			createUserForSession(sess, rm, token, userStore)
		}

		if csrfToken, err := generateCSRFToken(); err == nil {
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookieName,
				Value:    csrfToken,
				Path:     "/",
				SameSite: http.SameSiteLaxMode,
				HttpOnly: false,
			})
		} else {
			log.Error().Err(err).Msg("failed to generate CSRF token")
		}

		next.ServeHTTP(w, r)
	}

	return rm.LoginHandler(http.HandlerFunc(h))
}

func LoginHandler(repositoryManagers []RepositoryManager, userStore UserStore, next http.Handler) http.Handler {
	r := chi.NewRouter()

	handlers := map[int64]http.Handler{}

	for _, rm := range repositoryManagers {
		handlers[rm.ID()] = createLoginHandler(rm, userStore, next)
	}

	// redirect from scm
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		sess, _ := MoraSessionFrom(r.Context())
		if sess == nil {
			log.Error().Msg("No session found in context")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		if sess.loggingInto < 0 {
			log.Error().Msg("No current scm_id in session")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		rm_id := sess.loggingInto
		sess.loggingInto = -1 // reset

		handler, ok := handlers[rm_id]
		if !ok {
			render.NotFound(w, render.ErrNotFound)
			return
		}
		handler.ServeHTTP(w, r)
	})

	r.Get("/{scm_id}", func(w http.ResponseWriter, r *http.Request) {
		rm_id, err := strconv.ParseInt(chi.URLParam(r, "scm_id"), 10, 64)
		if err != nil {
			log.Err(err).Msg("invalid scm_id in login URL")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		sess, _ := MoraSessionFrom(r.Context())
		if sess != nil {
			sess.loggingInto = rm_id
		}

		handler, ok := handlers[rm_id]
		if !ok {
			render.NotFound(w, render.ErrNotFound)
			return
		}
		handler.ServeHTTP(w, r)
	})

	return r
}

func LogoutHandler(repositoryManagers []RepositoryManager, next http.Handler) http.Handler {
	r := chi.NewRouter()

	r.Post("/", func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		s, ok := MoraSessionFrom(r.Context())
		if !ok {
			log.Error().Msg("No session found in context")
			render.NotFound(w, render.ErrNotFound)
			return
		}
		if !verifyCSRF(r) {
			log.Warn().Msg("CSRF token mismatch on logout")
			render.Forbidden(w, render.ErrForbidden)
			return
		}
		for _, rm := range repositoryManagers {
			s.Remove(rm.ID())
		}
		next.ServeHTTP(w, r)
	})

	r.Post("/{scm_id}", func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		rm_id, err := strconv.ParseInt(chi.URLParam(r, "scm_id"), 10, 64)
		if err != nil {
			log.Err(err).Msg("invalid scm_id in logout URL")
			render.NotFound(w, render.ErrNotFound)
			return
		}
		s, ok := MoraSessionFrom(r.Context())
		if !ok {
			log.Error().Msg("No session found in context")
			render.NotFound(w, render.ErrNotFound)
			return
		}
		if !verifyCSRF(r) {
			log.Warn().Msg("CSRF token mismatch on logout")
			render.Forbidden(w, render.ErrForbidden)
			return
		}
		s.Remove(rm_id)
		next.ServeHTTP(w, r)
	})

	return r
}
