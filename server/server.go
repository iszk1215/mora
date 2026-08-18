package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/iszk1215/mora/config"
	"github.com/iszk1215/mora/core"
	"github.com/iszk1215/mora/coverage"
	_ "github.com/iszk1215/mora/docs"
	"github.com/iszk1215/mora/render"
	"github.com/iszk1215/mora/tracker"
	"github.com/iszk1215/mora/udm"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/swaggo/http-swagger/v2"
)

var (
	errorTokenNotFound = errors.New("token not found in a session")
)

type (
	Repository = core.Repository

	// Source Code Management System
	RepositoryManager interface {
		ID() int64
		URL() *url.URL
		Client() *scm.Client
		RevisionURL(baseURL string, revision string) string
		LoginHandler(next http.Handler) http.Handler
		Driver() string
	}

	// Protocols

	RepositoryManagerResponse struct {
		ID      int64  `json:"id"`
		URL     string `json:"url"`
		Name    string `json:"name"`
		LoggedIn bool  `json:"logined"`
	}

	RepositoryManagerStore interface {
		Init() error
		FindURL(string) (int64, string, error)
		Insert(driver string, url string) (int64, error)
	}

	RepositoryStore interface {
		Init() error
		Find(id int64) (core.Repository, error)
		FindURL(url string) (Repository, error)
		ListAll() ([]Repository, error)
		Put(repo *Repository) error
	}

	MoraServer struct {
		db                 *sqlx.DB
		repositoryManagers []RepositoryManager
		repos              RepositoryStore
		userStore          UserStore
		coverage           *coverage.CoverageService
		udm                *udm.Service
		tracker              *tracker.Service
		apiKey             string
		siteName           string
		demo               bool

		sessionManager     *MoraSessionManager
		frontendFileServer http.Handler
	}
)

func (s *MoraServer) findRepositoryManager(id int64) RepositoryManager {
	for _, rm := range s.repositoryManagers {
		if rm.ID() == id {
			return rm
		}
	}

	return nil
}

// API Handler

// handleRepoList godoc
// @Summary      List repositories
// @Description  List repositories the current session can access
// @Tags         server
// @Success      200  {array}   core.Repository
// @Failure      403  {object}  core.ErrorResponse
// @Router       /api/repos [get]
func (s *MoraServer) handleRepoList(w http.ResponseWriter, r *http.Request) {
	repositories, err := s.repos.ListAll()
	if err != nil {
		log.Err(err).Msg("failed to list repositories")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

	resp := []Repository{}
	sess, ok := MoraSessionFrom(r.Context())
	if !ok {
		render.Forbidden(w, render.ErrForbidden)
		return
	}

	for _, repo := range repositories {
		log.Debug().Int64("repo_id", repo.Id).Str("name", repo.Name).Msg("handleRepoList: processing repo")

		rm := s.findRepositoryManager(repo.RepositoryManager)
		if rm == nil {
			log.Warn().Int64("repo_id", repo.Id).Int64("rm_id", repo.RepositoryManager).
				Msg("handleRepoList: rm not found (skipped)")
			continue
		}

		if s.apiKey != "" && s.apiKey == token {
			log.Debug().Int64("repo_id", repo.Id).Msg("handleRepoList: included via api key")
			resp = append(resp, repo)
			continue
		}

		err = checkRepoAccess(sess, rm, repo)
		if err == nil {
			log.Debug().Int64("repo_id", repo.Id).Msg("handleRepoList: included via checkRepoAccess")
			resp = append(resp, repo)
		} else {
			log.Debug().Int64("repo_id", repo.Id).Err(err).Msg("handleRepoList: checkRepoAccess denied (skipped)")
		}
	}

	render.JSON(w, resp, http.StatusOK)
}

// handleRepositoryManagerList godoc
// @Summary      List SCM providers
// @Description  Return configured SCM repository managers and their login status
// @Tags         server
// @Success      200  {array}   server.RepositoryManagerResponse
// @Failure      403  {object}  core.ErrorResponse
// @Router       /api/providers [get]
func (s *MoraServer) handleRepositoryManagerList(w http.ResponseWriter, r *http.Request) {
	resp := []RepositoryManagerResponse{}
	sess, ok := MoraSessionFrom(r.Context())
	if !ok {
		render.Forbidden(w, render.ErrForbidden)
		return
	}

	for _, rm := range s.repositoryManagers {
		_, ok := sess.getToken(rm.ID())
		resp = append(resp, RepositoryManagerResponse{
			ID:      rm.ID(),
			URL:     rm.URL().String(),
			Name:    rm.Driver(),
			LoggedIn: ok,
		})
	}

	render.JSON(w, resp, http.StatusOK)
}

// handleMe godoc
// @Summary      Get current user
// @Description  Return the current session user, or 204 if not logged in
// @Tags         server
// @Success      200  {object}  server.User
// @Failure      204
// @Router       /api/me [get]
func (s *MoraServer) handleMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := MoraSessionFrom(r.Context())
	if !ok || sess.UserID() == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	user, err := s.userStore.FindByID(*sess.UserID())
	if err != nil {
		log.Error().Err(err).Msg("failed to find user")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	render.JSON(w, user, http.StatusOK)
}

type ConfigResponse struct {
	SiteName string `json:"site_name"`
}

// handleConfig godoc
// @Summary      Get server config
// @Description  Return server configuration (e.g. site name)
// @Tags         server
// @Success      200  {object}  server.ConfigResponse
// @Router       /api/config [get]
func (s *MoraServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	name := s.siteName
	if name == "" {
		name = "Mora"
	}
	render.JSON(w, ConfigResponse{
		SiteName: name,
	}, http.StatusOK)
}

func checkRepoAccessByRepositoryManager(session *MoraSession, rm RepositoryManager, owner, name string) error {
	ctx, err := session.WithToken(context.Background(), rm.ID())
	if err != nil {
		return err // errorTokenNotFound
	}

	_, _, err = rm.Client().Repositories.Find(ctx, owner+"/"+name)
	if err != nil {
		return fmt.Errorf("Repositories.Find(%s/%s): %w", owner, name, err)
	}

	return nil
}

// checkRepoAccess checks if token in session can access a repo 'owner/name'
func checkRepoAccess(sess *MoraSession, rm RepositoryManager, repo Repository) error {
	cache := sess.getReposCache(rm.ID())
	_, ok := cache[repo.Id]
	if ok {
		return nil
	}

	err := checkRepoAccessByRepositoryManager(sess, rm, repo.Namespace, repo.Name)
	if err != nil {
		return err
	}

	// store cache
	if cache == nil {
		cache = map[int64]bool{}
	}
	cache[repo.Id] = true
	sess.setReposCache(rm.ID(), cache)

	return err
}

func (s *MoraServer) injectRepo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repo_id, err := strconv.ParseInt(chi.URLParam(r, "repo_id"), 10, 64)
		if err != nil {
			log.Err(err).Msg("invalid repo_id in URL")
			render.BadRequest(w, errors.New("invalid repository id"))
			return
		}

		repo, err := s.repos.Find(repo_id)
		if err != nil {
			log.Err(err).Msg("failed to find repository")
			render.NotFound(w, errors.New("repository not found"))
			return
		}

		rm := s.findRepositoryManager(repo.RepositoryManager)
		if rm == nil {
			log.Error().Msgf("rm not found: id=%d", repo.RepositoryManager)
			render.InternalError(w, errors.New("internal error"))
			return
		}

		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

		ctx := r.Context()

		if s.apiKey == "" || s.apiKey != token {
			sess, ok := MoraSessionFrom(r.Context())
			if !ok {
				render.Forbidden(w, render.ErrForbidden)
				return
			}
			err = checkRepoAccess(sess, rm, repo)
			if errors.Is(err, errorTokenNotFound) {
				render.Forbidden(w, render.ErrForbidden)
				return
			} else if err != nil {
				log.Err(err).Msg("injectRepo")
				render.InternalError(w, errors.New("internal error"))
				return
			} else {
				ctx, _ = sess.WithToken(ctx, rm.ID())
			}
		}

		ctx = core.WithRepositoryClient(ctx, rm)
		ctx = core.WithRepo(ctx, repo)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *MoraServer) requireTrackerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := MoraSessionFrom(r.Context())
		if ok && sess.IsLoggedIn() {
			r = r.WithContext(tracker.ContextWithAuth(r.Context(), sess.UserID()))
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token != "" && token != r.Header.Get("Authorization") {
			user, err := s.userStore.FindUserByAPIKey(token)
			if err == nil && user != nil {
				r = r.WithContext(tracker.ContextWithAuth(r.Context(), &user.ID))
				next.ServeHTTP(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *MoraServer) injectTrackerCoverage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trackerID, err := strconv.ParseInt(chi.URLParam(r, "trackerId"), 10, 64)
		if err != nil {
			log.Err(err).Msg("invalid trackerId in URL")
			render.BadRequest(w, errors.New("invalid tracker id"))
			return
		}

		repo, err := s.coverage.FindRepoByTrackerID(trackerID)
		if err != nil {
			log.Err(err).Msg("failed to find repo by tracker_id")
			render.InternalError(w, errors.New("internal error"))
			return
		}
		if repo == nil {
			render.NotFound(w, errors.New("tracker has no associated repository"))
			return
		}

		rm := s.findRepositoryManager(repo.RepositoryManager)
		if rm == nil {
			log.Error().Msgf("rm not found: id=%d", repo.RepositoryManager)
			render.InternalError(w, errors.New("internal error"))
			return
		}

		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

		ctx := r.Context()

		if s.apiKey == "" || s.apiKey != token {
			sess, ok := MoraSessionFrom(r.Context())
			if !ok {
				render.Forbidden(w, render.ErrForbidden)
				return
			}
			err = checkRepoAccess(sess, rm, *repo)
			if errors.Is(err, errorTokenNotFound) {
				render.Forbidden(w, render.ErrForbidden)
				return
			} else if err != nil {
				log.Err(err).Msg("injectTrackerCoverage")
				render.InternalError(w, errors.New("internal error"))
				return
			} else {
				ctx, _ = sess.WithToken(ctx, rm.ID())
			}
		}

		ctx = core.WithRepositoryClient(ctx, rm)
		ctx = core.WithRepo(ctx, *repo)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// handleCoverageListPublic godoc
// @Summary      List coverages for a tracker
// @Description  Return coverage history for a tracker (public list endpoint)
// @Tags         coverage
// @Param        trackerId  path  int  true  "Tracker ID"
// @Success      200  {object}  coverage.CoverageListResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/coverages/{trackerId} [get]
func (s *MoraServer) handleCoverageListPublic(w http.ResponseWriter, r *http.Request) {
	trackerID, err := strconv.ParseInt(chi.URLParam(r, "trackerId"), 10, 64)
	if err != nil {
		log.Err(err).Msg("invalid trackerId in URL")
		render.BadRequest(w, errors.New("invalid tracker id"))
		return
	}

	repo, err := s.coverage.FindRepoByTrackerID(trackerID)
	if err != nil {
		log.Err(err).Msg("failed to find repo by tracker_id")
		render.InternalError(w, errors.New("internal error"))
		return
	}
	if repo == nil {
		render.NotFound(w, errors.New("tracker has no associated repository"))
		return
	}

	ctx := core.WithRepo(r.Context(), *repo)

	rm := s.findRepositoryManager(repo.RepositoryManager)
	if rm != nil {
		sess, ok := MoraSessionFrom(r.Context())
		if ok {
			if err := checkRepoAccess(sess, rm, *repo); err == nil {
				ctx, _ = sess.WithToken(ctx, rm.ID())
				ctx = core.WithRepositoryClient(ctx, rm)
			}
		}
	}

	s.coverage.HandleCoverageListPublic(w, r.WithContext(ctx))
}

type createCoverageTrackerRequest struct {
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
	RepoID     int64  `json:"repo_id"`
}

// handleCreateCoverageTracker godoc
//
//	@Summary		Create a coverage tracker
//	@Description	Create a coverage-type tracker linked to a repository. The repository must not already have a coverage tracker.
//	@Tags			coverage
//	@Accept			json
//	@Produce		json
//	@Param			body	body		createCoverageTrackerRequest	true	"Coverage tracker information"
//	@Success		201		{object}	tracker.TrackerModel
//	@Failure		400		{object}	core.ErrorResponse
//	@Failure		401		{object}	core.ErrorResponse
//	@Failure		403		{object}	core.ErrorResponse
//	@Failure		404		{object}	core.ErrorResponse
//	@Failure		409		{object}	core.ErrorResponse
//	@Router			/api/coverages [post]
func (s *MoraServer) handleCreateCoverageTracker(w http.ResponseWriter, r *http.Request) {
	uid, ok := tracker.UserIDFromContext(r.Context())
	if !ok {
		render.Forbidden(w, render.ErrForbidden)
		return
	}

	var req createCoverageTrackerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn().Err(err).Msg("invalid coverage tracker request body")
		render.BadRequest(w, errors.New("invalid request body"))
		return
	}

	if req.Name == "" {
		render.BadRequest(w, errors.New("name is required"))
		return
	}
	if req.Visibility != "public" && req.Visibility != "private" {
		render.BadRequest(w, errors.New("visibility must be one of: public, private"))
		return
	}
	if req.RepoID == 0 {
		render.BadRequest(w, errors.New("repo_id is required"))
		return
	}

	repo, err := s.repos.Find(req.RepoID)
	if err != nil {
		render.NotFound(w, errors.New("repository not found"))
		return
	}

	tr, err := s.coverage.CreateCoverageTracker(s.tracker, req.Name, "", req.Visibility, uid, repo)
	if err != nil {
		if errors.Is(err, coverage.ErrCoverageTrackerAlreadyLinked) {
			render.Conflict(w, errors.New("repository already has a coverage tracker"))
			return
		}
		log.Err(err).Msg("handleCreateCoverageTracker")
		render.InternalError(w, errors.New("internal error"))
		return
	}

	render.JSON(w, tr, http.StatusCreated)
}

func (s *MoraServer) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(s.sessionManager.SessionMiddleware)

	// api

	r.Get("/api/providers", s.handleRepositoryManagerList)
	r.Get("/api/me", s.handleMe)
	r.Get("/api/config", s.handleConfig)

	if s.userStore != nil {
		r.Mount("/api/user/me/api-keys", APIKeyHandler(s.userStore))
	}

	r.Route("/api/repos", func(r chi.Router) {
		r.Get("/", s.handleRepoList)
		r.Route("/{repo_id}", func(r chi.Router) {
			r.Use(s.injectRepo)

			if s.udm != nil {
				r.Mount("/udm", s.udm.Handler())
			}
		})
	})

	if s.tracker != nil {
		r.With(s.requireTrackerAuth).Mount("/api/trackers", s.tracker.Handler())
	}

	if s.tracker != nil && s.userStore != nil {
		r.Route("/api/users", func(r chi.Router) {
			r.With(s.requireTrackerAuth).Get("/{userName}", s.handleUserGet)
			r.With(s.requireTrackerAuth).Get("/{userName}/trackers", s.handleUserTrackers)
		})
	}

	if s.coverage != nil {
		r.Route("/api/coverages", func(r chi.Router) {
			r.With(s.requireTrackerAuth).Post("/", s.handleCreateCoverageTracker)
			r.Route("/{trackerId}", func(r chi.Router) {
				// List endpoint - no SCM auth required, but tracker visibility is checked
				r.With(s.requireTrackerAuth, s.tracker.InjectTracker, s.tracker.RequireReadPermission).Get("/", s.handleCoverageListPublic)
				// Preview endpoint - tracker visibility is checked
				r.With(s.requireTrackerAuth, s.tracker.InjectTracker, s.tracker.RequireReadPermission).Get("/preview", s.coverage.HandleCoveragePreview)
				// Other endpoints - tracker visibility + SCM auth required
				r.With(s.requireTrackerAuth, s.tracker.InjectTracker, s.tracker.RequireReadPermission, s.injectTrackerCoverage).Mount("/", s.coverage.Handler())
			})
		})
	}

	// login/logout

	redirectHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		})

	r.Mount("/login", LoginHandler(s.repositoryManagers, s.userStore, redirectHandler))
	r.Mount("/logout", LogoutHandler(s.repositoryManagers, redirectHandler))

	if s.userStore != nil {
		r.Mount("/api/signup", SignupHandler(s.userStore))
		r.Mount("/api/auth", PasswordAuthHandler(s.userStore))
	}

	// frontend

	r.Mount("/swagger/", httpSwagger.WrapHandler)

	r.Route("/", func(r chi.Router) {
		r.Get("/assets/*", func(w http.ResponseWriter, r *http.Request) {
			s.frontendFileServer.ServeHTTP(w, r)
		})

		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = "/"
			s.frontendFileServer.ServeHTTP(w, r)
		})
	})

	return r
}

func initRepositoryManager(cfg config.RepositoryManagerConfig, baseURL string, store RepositoryManagerStore) (RepositoryManager, error) {
	if cfg.Driver == "github" && cfg.URL == "" {
		cfg.URL = "https://github.com"
	}

	if cfg.Driver == "google" && cfg.URL == "" {
		cfg.URL = "https://accounts.google.com"
	}

	if cfg.URL == "" {
		return nil, fmt.Errorf("ConfigError: rm.url is empty")
	}

	if cfg.SecretFilename == "" {
		return nil, fmt.Errorf("ConfigError: rm.secret_file is empty")
	}

	id, _, err := store.FindURL(cfg.URL)
	if err != nil {
		return nil, err
	}

	if id < 0 {
		id, err = store.Insert(cfg.Driver, cfg.URL)
		if err != nil {
			return nil, err
		}
		log.Info().Msgf("New repository manager is configured. ID=%d Driver=%s URL=%s",
			id, cfg.Driver, cfg.URL)
	} else {
		log.Info().Msgf("repository manager enabled. ID=%d Driver=%s URL=%s",
			id, cfg.Driver, cfg.URL)
	}

	switch cfg.Driver {
	case "gitea":
		return NewGiteaFromFile(
			id,
			cfg.SecretFilename,
			cfg.URL,
			baseURL+"/login",
			cfg.InsecureSkipVerify)
	case "github":
		return NewGithubFromFile(id, cfg.URL, cfg.SecretFilename, baseURL+"/login")
	case "google":
		return NewGoogleFromFile(id, cfg.URL, cfg.SecretFilename, baseURL+"/login")
	default:
		return nil, fmt.Errorf("ConfigError: unknown repository manager: %s", cfg.Driver)
	}

}

func initRepositoryManagers(cfg config.MoraConfig, store RepositoryManagerStore) ([]RepositoryManager, error) {
	repositoryManagers := []RepositoryManager{}
	for _, rmConfig := range cfg.RepositoryManagers {
		rm, err := initRepositoryManager(rmConfig, cfg.Server.URL, store)
		if err != nil {
			return nil, fmt.Errorf("initRepositoryManager(%s): %w", rmConfig.Driver, err)
		}
		repositoryManagers = append(repositoryManagers, rm)
	}

	return repositoryManagers, nil
}

func initStore(cfg config.MoraConfig) (*sqlx.DB, RepositoryManagerStore, RepositoryStore, UserStore, error) {
	db, err := OpenDB(cfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("OpenDB: %w", err)
	}

	rmStore := NewRepositoryManagerStore(db)
	if err := rmStore.Init(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("rmStore.Init: %w", err)
	}

	repoStore := NewRepositoryStore(db)
	if err := repoStore.Init(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("repoStore.Init: %w", err)
	}

	userStore := NewUserStore(db)
	if err := userStore.Init(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("userStore.Init: %w", err)
	}

	return db, rmStore, repoStore, userStore, nil
}

//go:embed static
var staticFS embed.FS

func initFrontendFileServer() (http.Handler, error) {
	frontendFS, err := fs.Sub(staticFS, "static/public")
	if err != nil {
		return nil, fmt.Errorf("fs.Sub(static/public): %w", err)
	}

	return http.FileServer(http.FS(frontendFS)), nil
}

func NewMoraServerFromConfig(cfg config.MoraConfig) (*MoraServer, error) {
	db, rmStore, repoStore, userStore, err := initStore(cfg)
	if err != nil {
		log.Err(err).Msg("initStore")
		return nil, err
	}

	repositoryManagers, err := initRepositoryManagers(cfg, rmStore)
	if err != nil {
		return nil, err
	}
	if !cfg.Demo && len(repositoryManagers) == 0 {
		return nil, fmt.Errorf("ConfigError: no RepositoryManager is configured")
	}

	frontendFileServer, err := initFrontendFileServer()
	if err != nil {
		return nil, err
	}

	coverage, err := coverage.NewCoverageService(db)
	if err != nil {
		return nil, err
	}

	udm, err := udm.NewService(db)
	if err != nil {
		return nil, err
	}

	trackerService, err := tracker.NewService(db)
	if err != nil {
		return nil, err
	}

	// Migrate coverage-type trackers for repositories that have none yet.
	// Runs after both services exist so that tracker creation goes through
	// the tracker service while linking is owned by the coverage service.
	if err := coverage.MigrateCoverageTrackers(trackerService); err != nil {
		return nil, err
	}

	s := &MoraServer{
		db:                 db,
		sessionManager:     NewMoraSessionManager(),
		repositoryManagers: repositoryManagers,
		repos:              repoStore,
		userStore:          userStore,
		frontendFileServer: frontendFileServer,
		coverage:           coverage,
		udm:                udm,
		tracker:              trackerService,
		apiKey:             os.Getenv("MORA_API_KEY"),
		siteName:           cfg.Server.SiteName,
		demo:               cfg.Demo,
	}

	if s.demo {
		if err := s.seedDemoData(); err != nil {
			return nil, fmt.Errorf("seed demo data: %w", err)
		}
	}

	return s, err
}

func (s *MoraServer) Close() error {
	if err := s.sessionManager.Close(); err != nil {
		return fmt.Errorf("session manager close: %w", err)
	}
	return s.db.Close()
}
