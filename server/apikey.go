package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/render"
	"github.com/rs/zerolog/log"
)

type CreateAPIKeyRequest struct {
	Name string `json:"name"`
}

type CreateAPIKeyResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"`
	KeyPrefix string `json:"key_prefix"`
	CreatedAt string `json:"created_at"`
}

func APIKeyHandler(userStore UserStore) http.Handler {
	r := chi.NewRouter()

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		// listAPIKeys godoc
		// @Summary      List API keys
		// @Description  Return API keys for the current user
		// @Tags         api-key
		// @Success      200  {array}   server.UserAPIKey
		// @Failure      401  {object}  core.ErrorResponse
		// @Router       /api/user/me/api-keys [get]
		sess, ok := MoraSessionFrom(r.Context())
		if !ok || sess.UserID() == nil {
			render.Forbidden(w, render.ErrForbidden)
			return
		}

		keys, err := userStore.ListAPIKeys(*sess.UserID())
		if err != nil {
			log.Error().Err(err).Msg("APIKeyHandler list")
			render.InternalError(w, err)
			return
		}
		if keys == nil {
			keys = []UserAPIKey{}
		}
		render.JSON(w, keys, http.StatusOK)
	})

	r.Post("/", func(w http.ResponseWriter, r *http.Request) {
		// createAPIKey godoc
		// @Summary      Create an API key
		// @Description  Create a new API key for the current user
		// @Tags         api-key
		// @Accept       json
		// @Produce      json
		// @Param        body  body  server.CreateAPIKeyRequest  true  "API key name"
		// @Success      201   {object}  server.CreateAPIKeyResponse
		// @Failure      400   {object}  core.ErrorResponse
		// @Failure      401   {object}  core.ErrorResponse
		// @Router       /api/user/me/api-keys [post]
		if !verifyCSRF(r) {
			log.Warn().Msg("CSRF token mismatch on API key create")
			render.Forbidden(w, render.ErrForbidden)
			return
		}

		sess, ok := MoraSessionFrom(r.Context())
		if !ok || sess.UserID() == nil {
			render.Forbidden(w, render.ErrForbidden)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		defer func() {
			_ = r.Body.Close()
		}()

		var req CreateAPIKeyRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			render.BadRequest(w, errors.New("invalid request body"))
			return
		}
		if req.Name == "" {
			render.BadRequest(w, errors.New("name is required"))
			return
		}

		plaintext, err := userStore.CreateAPIKey(*sess.UserID(), req.Name)
		if err != nil {
			log.Error().Err(err).Msg("APIKeyHandler create")
			render.InternalError(w, err)
			return
		}

		// Fetch the created key to get ID and timestamps
		keys, err := userStore.ListAPIKeys(*sess.UserID())
		if err != nil || len(keys) == 0 {
			log.Error().Err(err).Msg("APIKeyHandler list after create")
			render.InternalError(w, err)
			return
		}

		created := keys[0]
		resp := CreateAPIKeyResponse{
			ID:        created.ID,
			Name:      created.Name,
			Key:       plaintext,
			KeyPrefix: created.KeyPrefix,
			CreatedAt: created.CreatedAt,
		}
		render.JSON(w, resp, http.StatusCreated)
	})

	r.Delete("/{keyId}", func(w http.ResponseWriter, r *http.Request) {
		// revokeAPIKey godoc
		// @Summary      Revoke an API key
		// @Description  Revoke an API key by ID
		// @Tags         api-key
		// @Param        keyId  path  int  true  "API key ID"
		// @Success      204
		// @Failure      400  {object}  core.ErrorResponse
		// @Failure      401  {object}  core.ErrorResponse
		// @Failure      404  {object}  core.ErrorResponse
		// @Router       /api/user/me/api-keys/{keyId} [delete]
		if !verifyCSRF(r) {
			log.Warn().Msg("CSRF token mismatch on API key revoke")
			render.Forbidden(w, render.ErrForbidden)
			return
		}

		sess, ok := MoraSessionFrom(r.Context())
		if !ok || sess.UserID() == nil {
			render.Forbidden(w, render.ErrForbidden)
			return
		}

		keyID, err := strconv.ParseInt(chi.URLParam(r, "keyId"), 10, 64)
		if err != nil {
			render.BadRequest(w, errors.New("invalid key id"))
			return
		}

		err = userStore.RevokeAPIKey(*sess.UserID(), keyID)
		if err != nil {
			log.Error().Err(err).Msg("APIKeyHandler revoke")
			render.NotFound(w, errors.New("api key not found"))
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	return r
}
