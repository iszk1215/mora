package server

import (
	"context"
	"embed"
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
	"github.com/iszk1215/mora/render"
	"github.com/iszk1215/mora/track"
	"github.com/iszk1215/mora/udm"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
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
	}

	// Protocols

	RepositoryManagerResponse struct {
		ID      int64  `json:"id"`
		URL     string `json:"url"`
		LoggedIn bool   `json:"logined"`
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
		track              *track.Service
		apiKey             string

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
			LoggedIn: ok,
		})
	}

	render.JSON(w, resp, http.StatusOK)
}

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

func (s *MoraServer) requireTrackAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := MoraSessionFrom(r.Context())
		if ok && sess.IsLoggedIn() {
			r = r.WithContext(track.ContextWithAuth(r.Context()))
		}
		next.ServeHTTP(w, r)
	})
}

func (s *MoraServer) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(s.sessionManager.SessionMiddleware)

	// api

	r.Get("/api/scms", s.handleRepositoryManagerList)
	r.Get("/api/me", s.handleMe)

	r.Route("/api/repos", func(r chi.Router) {
		r.Get("/", s.handleRepoList)
		r.Route("/{repo_id}", func(r chi.Router) {
			r.Use(s.injectRepo)
			if s.coverage != nil {
				r.Mount("/coverages", s.coverage.Handler())
			}

			if s.udm != nil {
				r.Mount("/udm", s.udm.Handler())
			}
		})
	})

	if s.track != nil {
		r.With(s.requireTrackAuth).Mount("/api/track", s.track.Handler())
	}

	// login/logout

	redirectHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/scms", http.StatusSeeOther)
		})

	r.Mount("/login", LoginHandler(s.repositoryManagers, s.userStore, redirectHandler))
	r.Mount("/logout", LogoutHandler(s.repositoryManagers, redirectHandler))

	// frontend

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

func initStore(filename string) (*sqlx.DB, RepositoryManagerStore, RepositoryStore, UserStore, error) {
	log.Info().Msgf("Initialize store: filename=%s", filename)

	if filename != "" && !strings.HasPrefix(filename, ":memory:") &&
		!strings.HasPrefix(filename, "file::memory:") {
		filename += "?_journal_mode=WAL&_busy_timeout=5000"
	}
	db, err := sqlx.Connect("sqlite3", filename)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("sqlx.Connect: %w", err)
	}
	db.SetMaxOpenConns(1)

	db.MustExec("PRAGMA foreign_keys = ON")

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
	db, rmStore, repoStore, userStore, err := initStore(cfg.DatabaseFilename)
	if err != nil {
		log.Err(err).Msg("initStore")
		return nil, err
	}

	repositoryManagers, err := initRepositoryManagers(cfg, rmStore)
	if err != nil {
		return nil, err
	}
	if len(repositoryManagers) == 0 {
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

	trackService, err := track.NewService(db, os.Getenv("MORA_API_KEY"))
	if err != nil {
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
		track:              trackService,
		apiKey:             os.Getenv("MORA_API_KEY"),
	}

	return s, err
}

func (s *MoraServer) Close() error {
	if err := s.sessionManager.Close(); err != nil {
		return fmt.Errorf("session manager close: %w", err)
	}
	return s.db.Close()
}
