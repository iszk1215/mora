package server

import (
	"net/http"

	"github.com/drone/drone/handler/api/render"
	"github.com/drone/go-login/login"
	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

func convertToken(token *login.Token) scm.Token {
	return scm.Token{
		Token:   token.Access,
		Refresh: token.Refresh,
		Expires: token.Expires,
	}
}

func createLoginHandler(scm SCM, next http.Handler) http.Handler {
	h := func(w http.ResponseWriter, r *http.Request) {
		err := login.ErrorFrom(r.Context())
		if err != nil {
			log.Error().Err(err).Msg("")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		log.Print("Set token to session: id=", scm.ID(), "url=", scm.URL())
		token := convertToken(login.TokenFrom(r.Context()))

		sess, _ := MoraSessionFrom(r.Context())
		sess.setToken(scm.ID(), token)

		next.ServeHTTP(w, r)
	}

	return scm.LoginHandler(http.HandlerFunc(h))
}

func LoginHandler(scms []SCM, next http.Handler) http.Handler {
	r := chi.NewRouter()

	handlers := map[string]http.Handler{}

	for _, scm := range scms {
		handlers[scm.Name()] = createLoginHandler(scm, next)
	}

	r.Get("/{scm}", func(w http.ResponseWriter, r *http.Request) {
		scm := chi.URLParam(r, "scm")
		handler, ok := handlers[scm]
		if !ok {
			render.NotFound(w, render.ErrNotFound)
			return
		}
		handler.ServeHTTP(w, r)
	})

	return r
}

func LogoutHandler(scms []SCM, next http.Handler) http.Handler {
	r := chi.NewRouter()

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		s, _ := MoraSessionFrom(r.Context())
		for _, scm := range scms {
			s.Remove(scm.ID())
		}
		next.ServeHTTP(w, r)
	})

	r.Get("/{scm}", func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "scm")
		s, _ := MoraSessionFrom(r.Context())
		// TODO
		for _, scm := range scms {
			if scm.Name() == name {
				s.Remove(scm.ID())
				break
			}
		}
		next.ServeHTTP(w, r)
	})

	return r
}
