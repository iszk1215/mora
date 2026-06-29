package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/render"
	"github.com/rs/zerolog/log"
)

type PendingSignupResponse struct {
	Provider  string `json:"provider"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

func SignupHandler(userStore UserStore) http.Handler {
	r := chi.NewRouter()

	r.Get("/pending", func(w http.ResponseWriter, r *http.Request) {
		sess, ok := MoraSessionFrom(r.Context())
		if !ok {
			render.NotFound(w, render.ErrNotFound)
			return
		}

		p := sess.PendingSignup()
		if p == nil {
			render.NotFound(w, render.ErrNotFound)
			return
		}

		render.JSON(w, PendingSignupResponse{
			Provider:  p.provider,
			Username:  p.username,
			AvatarURL: p.avatarURL,
		}, http.StatusOK)
	})

	r.Post("/confirm", func(w http.ResponseWriter, r *http.Request) {
		sess, ok := MoraSessionFrom(r.Context())
		if !ok {
			render.NotFound(w, render.ErrNotFound)
			return
		}

		p := sess.PendingSignup()
		if p == nil {
			render.NotFound(w, render.ErrNotFound)
			return
		}

		if !verifyCSRF(r) {
			log.Warn().Msg("CSRF token mismatch on signup confirm")
			render.Forbidden(w, render.ErrForbidden)
			return
		}

		user, err := userStore.CreateUser(p.username, p.avatarURL)
		if err != nil {
			log.Error().Err(err).Msg("failed to create user during signup")
			render.InternalError(w, err)
			return
		}

		err = userStore.LinkAuth(user.ID, p.provider, p.providerUserID)
		if err != nil {
			log.Error().Err(err).Msg("failed to link auth during signup")
			render.InternalError(w, err)
			return
		}

		sess.SetUserID(user.ID)
		sess.ClearPendingSignup()

		render.JSON(w, user, http.StatusCreated)
	})

	return r
}
