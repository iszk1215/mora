package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/render"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

type PasswordLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func PasswordAuthHandler(userStore UserStore, insecureCookie bool) http.Handler {
	r := chi.NewRouter()

	r.Get("/csrf", func(w http.ResponseWriter, r *http.Request) {
		// getCSRFToken godoc
		// @Summary      Get CSRF token
		// @Description  Generate and return a CSRF token via cookie and JSON body
		// @Tags         auth
		// @Produce      json
		// @Success      200  {object}  map[string]string
		// @Router       /api/auth/csrf [get]
		csrfToken, err := generateCSRFToken()
		if err != nil {
			log.Error().Err(err).Msg("failed to generate CSRF token")
			render.InternalError(w, err)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     csrfCookieName,
			Value:    csrfToken,
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
			HttpOnly: false,
			Secure:   secureCookieAttr(insecureCookie, r),
		})
		render.JSON(w, map[string]string{"csrf_token": csrfToken}, http.StatusOK)
	})

	r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		// passwordLogin godoc
		// @Summary      Log in with username and password
		// @Description  Authenticate with a username and password, creating a session
		// @Tags         auth
		// @Accept       json
		// @Produce      json
		// @Param        body  body  server.PasswordLoginRequest  true  "Login credentials"
		// @Success      200   {object}  server.User
		// @Failure      400   {object}  core.ErrorResponse
		// @Failure      403   {object}  core.ErrorResponse
		// @Router       /api/auth/login [post]
		sess, ok := MoraSessionFrom(r.Context())
		if !ok {
			render.Forbidden(w, render.ErrForbidden)
			return
		}

		if !verifyCSRF(r) {
			log.Warn().Msg("CSRF token mismatch on password login")
			render.Forbidden(w, render.ErrForbidden)
			return
		}

		var req PasswordLoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Warn().Err(err).Msg("invalid password login request body")
			render.BadRequest(w, errors.New("invalid request body"))
			return
		}
		defer func() { _ = r.Body.Close() }()

		if req.Username == "" || req.Password == "" {
			render.BadRequest(w, errors.New("username and password are required"))
			return
		}

		user, err := userStore.FindByUsername(req.Username)
		if err != nil {
			log.Warn().Err(err).Str("username", req.Username).Msg("user not found for password login")
			render.Forbidden(w, errors.New("invalid username or password"))
			return
		}

		passwordHash, err := userStore.GetPasswordHash(user.ID)
		if err != nil {
			log.Error().Err(err).Int64("user_id", user.ID).Msg("failed to get password hash")
			render.InternalError(w, err)
			return
		}

		if passwordHash == nil {
			log.Warn().Int64("user_id", user.ID).Msg("user has no password set")
			render.Forbidden(w, errors.New("invalid username or password"))
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(*passwordHash), []byte(req.Password)); err != nil {
			log.Warn().Int64("user_id", user.ID).Msg("password mismatch")
			render.Forbidden(w, errors.New("invalid username or password"))
			return
		}

		sess.SetUserID(user.ID)

		render.JSON(w, user, http.StatusOK)
	})

	return r
}
