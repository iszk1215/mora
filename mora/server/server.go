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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/drone/drone/handler/api/render"
	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/iszk1215/mora/mora/model"
	"github.com/iszk1215/mora/mora/udm"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

var (
	errorTokenNotFound = errors.New("token not found in a session")
)

type (
	Repository = model.Repository

	// Source Code Management System
	RepositoryManager interface {
		ID() int64
		URL() *url.URL
		Client() *scm.Client
		RevisionURL(baseURL string, revision string) string
		LoginHandler(next http.Handler) http.Handler
	}

	// contextKey int

	// Protocols

	RepositoryManagerResponse struct {
		ID      int64  `json:"id"`
		URL     string `json:"url"`
		Logined bool   `json:"logined"`
	}

	RepositoryManagerStore interface {
		Init() error
		FindURL(string) (int64, string, error)
		Insert(driver string, url string) (int64, error)
	}

	RepositoryStore interface {
		Init() error
		Find(id int64) (model.Repository, error)
		FindURL(url string) (Repository, error)
		ListAll() ([]Repository, error)
		Put(repo *Repository) error
	}

	CoverageStore interface {
		Init() error
		Find(id int64) (*Coverage, error)
		FindRevision(id int64, revision string) (*Coverage, error)
		List(id int64) ([]*Coverage, error)
		ListAll() ([]*Coverage, error)
		Put(*Coverage) error
	}

	ResourceHandler interface {
		Handler() http.Handler
		HandleUpload(w http.ResponseWriter, r *http.Request)
	}

	MoraServer struct {
		repositoryManagers []RepositoryManager
		repos              RepositoryStore
		coverage           ResourceHandler
		udm                *udm.Service
		apiKey             string

		sessionManager     *MoraSessionManager
		frontendFileServer http.Handler
	}
)

const (
	contextRepositoryManagerKey = model.ContextRepositoryManagerKey
)

func WithRepostioryManager(ctx context.Context, rm RepositoryManager) context.Context {
	return context.WithValue(ctx, contextRepositoryManagerKey, rm)
}

func RepositoryManagerFrom(ctx context.Context) (RepositoryManager, bool) {
	rm, ok := ctx.Value(contextRepositoryManagerKey).(RepositoryManager)
	return rm, ok
}

func WithRepo(ctx context.Context, repo Repository) context.Context {
	return model.WithRepo(ctx, repo)
	// return context.WithValue(ctx, contextRepoKey, repo)
}

func RepoFrom(ctx context.Context) (Repository, bool) {
	return model.RepoFrom(ctx)
	/*
		repo, ok := ctx.Value(contextRepoKey).(Repository)
		return repo, ok
	*/
}

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
	log.Print("HandleRepoList")

	repositories, err := s.repos.ListAll()
	if err != nil {
		log.Err(err).Msg("")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	log.Print("HandleRepoList: token=", token)

	resp := []Repository{}
	sess, _ := MoraSessionFrom(r.Context())

	for _, repo := range repositories {
		rm := s.findRepositoryManager(repo.RepositoryManager)
		if rm == nil {
			log.Warn().Msgf(
				"rm not found for repository: repo.ID=%d rm.ID=%d (skipped)",
				repo.Id, repo.RepositoryManager)
			continue
		}

		if s.apiKey != "" && s.apiKey == token {
			resp = append(resp, repo)
			continue
		}

		err = checkRepoAccess(sess, rm, repo)
		if err == nil {
			resp = append(resp, repo)
		}
	}

	render.JSON(w, resp, http.StatusOK)
}

func (s *MoraServer) handleRepositoryManagerList(w http.ResponseWriter, r *http.Request) {
	resp := []RepositoryManagerResponse{}
	sess, _ := MoraSessionFrom(r.Context())

	for _, rm := range s.repositoryManagers {
		_, ok := sess.getToken(rm.ID())
		resp = append(resp, RepositoryManagerResponse{
			ID:      rm.ID(),
			URL:     rm.URL().String(),
			Logined: ok,
		})
	}

	render.JSON(w, resp, 200)
}

func checkRepoAccessByRepositoryManager(session *MoraSession, rm RepositoryManager, owner, name string) error {
	ctx, err := session.WithToken(context.Background(), rm.ID())
	if err != nil {
		return err // errorTokenNotFound
	}

	_, _, err = rm.Client().Repositories.Find(ctx, owner+"/"+name)
	if err != nil {
		return err
	}

	return nil
}

// checkRepoAccess checks if token in session can access a repo 'owner/name'
func checkRepoAccess(sess *MoraSession, rm RepositoryManager, repo Repository) error {
	cache := sess.getReposCache(rm.ID())
	_, ok := cache[repo.Id]
	if ok {
		log.Print("checkRepoAccess: found in cache")
		return nil
	}

	err := checkRepoAccessByRepositoryManager(sess, rm, repo.Namespace, repo.Name)
	if err != nil {
		log.Print("checkRepoAccess: no repo or no access at RepositoryManager")
		return err
	}
	log.Print("checkRepoAccess: found in RepositoryManager: ", repo.Url)

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
			log.Err(err).Msg("")
			render.BadRequest(w, errors.New("invalid repository id"))
			return
		}

		log.Print("injectRepo: repo_id=", repo_id)

		repo, err := s.repos.Find(repo_id)
		if err != nil {
			log.Err(err).Msg("")
			render.BadRequest(w, errors.New("invalid repository id"))
			return
		}

		rm := s.findRepositoryManager(repo.RepositoryManager)
		if rm == nil {
			log.Error().Msgf("rm not found: id=%d", repo.RepositoryManager)
			render.InternalError(w, errors.New("internal error"))
			return
		}

		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

		if s.apiKey == "" || s.apiKey != token {
			sess, _ := MoraSessionFrom(r.Context())
			err = checkRepoAccess(sess, rm, repo)
			if err == errorTokenNotFound {
				render.Forbidden(w, render.ErrForbidden)
				return
			} else if err != nil {
				log.Err(err).Msg("injectRepo")
				render.InternalError(w, errors.New("internal error"))
				return
			}
		} else {
			log.Print("injectRepo: skip checking repo access")
		}

		ctx := r.Context()
		ctx = WithRepostioryManager(ctx, rm)
		ctx = WithRepo(ctx, repo)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *MoraServer) HandleUpload(w http.ResponseWriter, r *http.Request) {
	s.coverage.HandleUpload(w, r)
}

func (s *MoraServer) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(s.sessionManager.SessionMiddleware)

	// api

	r.Get("/api/scms", s.handleRepositoryManagerList)

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

	r.Post("/api/upload", s.HandleUpload)

	// login/logout

	redirectHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/scms", http.StatusSeeOther)
		})

	r.Mount("/login", LoginHandler(s.repositoryManagers, redirectHandler))
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

func initRepositoryManager(config RepositoryManagerConfig, baseURL string, store RepositoryManagerStore) (RepositoryManager, error) {
	if config.Driver == "github" && config.URL == "" {
		config.URL = "https://github.com"
	}

	if config.URL == "" {
		return nil, errors.New("ConfigError: rm.url is empty")
	}

	if config.SecretFilename == "" {
		return nil, errors.New("ConfigError: rm.secret_url is empty")
	}

	id, _, err := store.FindURL(config.URL)
	if err != nil {
		return nil, err
	}

	if id < 0 {
		id, err = store.Insert(config.Driver, config.URL)
		if err != nil {
			return nil, err
		}
		log.Info().Msgf("New repository manager is configured. ID=%d Driver=%s URL=%s",
			id, config.Driver, config.URL)
	} else {
		log.Info().Msgf("repository manager enabled. ID=%d Driver=%s URL=%s",
			id, config.Driver, config.URL)
	}

	if config.Driver == "gitea" {
		return NewGiteaFromFile(
			id,
			config.SecretFilename,
			config.URL,
			baseURL+"/login")
	} else if config.Driver == "github" {
		return NewGithubFromFile(id, config.URL, config.SecretFilename)
	}

	return nil, fmt.Errorf("ConfigError: unknown repository manager: %s", config.Driver)
}

func initRepositoryManagers(config MoraConfig, store RepositoryManagerStore) ([]RepositoryManager, error) {
	repositoryManagers := []RepositoryManager{}
	for _, rmConfig := range config.RepositoryManagers {
		rm, err := initRepositoryManager(rmConfig, config.Server.URL, store)
		if err != nil {
			return nil, err
		}
		repositoryManagers = append(repositoryManagers, rm)
	}

	return repositoryManagers, nil
}

func initStore(filename string) (*sqlx.DB, RepositoryManagerStore, RepositoryStore, CoverageStore, error) {
	log.Info().Msgf("Initialize store: filename=%s", filename)

	db, err := sqlx.Connect("sqlite3", filename)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	rmStore := NewRepositoryManagerStore(db)
	if err := rmStore.Init(); err != nil {
		return nil, nil, nil, nil, err
	}

	repoStore := NewRepositoryStore(db)
	if err := repoStore.Init(); err != nil {
		return nil, nil, nil, nil, err
	}

	covStore := NewCoverageStore(db)
	if err := covStore.Init(); err != nil {
		return nil, nil, nil, nil, err
	}

	return db, rmStore, repoStore, covStore, nil
}

//go:embed static
var embedded embed.FS

func getStaticFS(staticDir string, path string, debug bool) (fs.FS, error) {
	if debug {
		return os.DirFS(filepath.Join(staticDir, path)), nil
	}

	return fs.Sub(embedded, filepath.Join("static", path))
}

func initFrontendFileServer(config MoraConfig) (http.Handler, error) {
	staticDir := "mora/server/static"
	frontendFS, err := getStaticFS(staticDir, "public", config.Debug)
	if err != nil {
		return nil, err
	}

	return http.FileServer(http.FS(frontendFS)), err
}

func NewMoraServerFromConfig(config MoraConfig) (*MoraServer, error) {
	log.Print("config.Debug=", config.Debug)

	db, rmStore, repoStore, covStore, err := initStore(config.DatabaseFilename)
	if err != nil {
		log.Err(err).Msg("initStore")
		return nil, err
	}

	repositoryManagers, err := initRepositoryManagers(config, rmStore)
	if err != nil {
		return nil, err
	}
	if len(repositoryManagers) == 0 {
		return nil, errors.New("no RepositoryManager is configured")
	}

	frontendFileServer, err := initFrontendFileServer(config)
	if err != nil {
		return nil, err
	}

	coverage := NewCoverageHandler(repoStore, covStore)

	udm, err := udm.NewService(db)
	if err != nil {
		return nil, err
	}

	s := &MoraServer{
		sessionManager:     NewMoraSessionManager(),
		repositoryManagers: repositoryManagers,
		repos:              repoStore,
		frontendFileServer: frontendFileServer,
		coverage:           coverage,
		udm:                udm,
		apiKey:             os.Getenv("MORA_API_KEY"),
	}

	return s, err
}
