package server

import (
	"net/http"
	"strconv"

	"github.com/drone/go-login/login"
	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/mora/render"
	"github.com/rs/zerolog/log"
)

func convertToken(token *login.Token) scm.Token {
	return scm.Token{
		Token:   token.Access,
		Refresh: token.Refresh,
		Expires: token.Expires,
	}
}

func createLoginHandler(rm RepositoryManager, next http.Handler) http.Handler {
	h := func(w http.ResponseWriter, r *http.Request) {
		err := login.ErrorFrom(r.Context())
		if err != nil {
			log.Error().Err(err).Msg("")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		log.Print("Set token to session for RepositoryManager: id=", rm.ID(), " url=", rm.URL())
		token := convertToken(login.TokenFrom(r.Context()))

		sess, _ := MoraSessionFrom(r.Context())
		sess.setToken(rm.ID(), token)

		next.ServeHTTP(w, r)
	}

	return rm.LoginHandler(http.HandlerFunc(h))
}

func LoginHandler(repositoryManagers []RepositoryManager, next http.Handler) http.Handler {
	r := chi.NewRouter()

	handlers := map[int64]http.Handler{}

	for _, rm := range repositoryManagers {
		handlers[rm.ID()] = createLoginHandler(rm, next)
	}

	// redirect from scm
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		sess, _ := MoraSessionFrom(r.Context())
		if sess == nil {
			log.Error().Msg("No session found in context")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		log.Print("LoginHandler: sess.loggingInto=", sess.loggingInto)
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
			log.Err(err).Msg("")
			render.NotFound(w, render.ErrNotFound)
			return
		}
		log.Print("login: rm_id=", rm_id)

		sess, _ := MoraSessionFrom(r.Context())
		if sess != nil {
			log.Print("LoginHandler: sess.loggingInto=", sess.loggingInto)
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

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		s, _ := MoraSessionFrom(r.Context())
		for _, rm := range repositoryManagers {
			s.Remove(rm.ID())
		}
		next.ServeHTTP(w, r)
	})

	r.Get("/{scm_id}", func(w http.ResponseWriter, r *http.Request) {
		rm_id, err := strconv.ParseInt(chi.URLParam(r, "scm_id"), 10, 64)
		if err != nil {
			log.Err(err).Msg("")
			render.NotFound(w, render.ErrNotFound)
			return
		}
		s, _ := MoraSessionFrom(r.Context())
		s.Remove(rm_id)
		next.ServeHTTP(w, r)
	})

	return r
}
