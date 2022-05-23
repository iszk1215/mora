package mora

import (
	"net/http"

	"github.com/drone/drone/handler/api/render"
	"github.com/drone/go-login/login"
	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

func createLoginHandler(client Client, next http.Handler) http.Handler {
	h := func(w http.ResponseWriter, r *http.Request) {
		err := login.ErrorFrom(r.Context())
		if err != nil {
			log.Error().Err(err).Msg("")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		log.Print("Set token to session: ", client.Name())
		token := login.TokenFrom(r.Context())

		scmToken := scm.Token{
			Token:   token.Access,
			Refresh: token.Refresh,
		}

		sess, _ := MoraSessionFrom(r.Context())
		sess.setToken(client.Name(), scmToken)

		next.ServeHTTP(w, r)
	}

	return client.LoginHandler(http.HandlerFunc(h))
}

func LoginHandler(clients []Client, next http.Handler) http.Handler {
	r := chi.NewRouter()

	handlers := map[string]http.Handler{}

	for _, client := range clients {
		handlers[client.Name()] = createLoginHandler(client, next)
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

func LogoutHandler(clients []Client, next http.Handler) http.Handler {
	r := chi.NewRouter()

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		s, _ := MoraSessionFrom(r.Context())
		for _, c := range clients {
			s.Remove(c.Name())
		}
		next.ServeHTTP(w, r)
	})

	r.Get("/{scm}", func(w http.ResponseWriter, r *http.Request) {
		scm := chi.URLParam(r, "scm")
		s, _ := MoraSessionFrom(r.Context())
		s.Remove(scm)
		next.ServeHTTP(w, r)
	})

	return r
}
