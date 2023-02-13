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

	"github.com/drone/drone/handler/api/render"
	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

var (
	errorTokenNotFound = errors.New("token not found in a session")
)

type (
	Repository struct {
		ID        int64  `json:"id"`
		SCM       int64  `json:"scm_id"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		Link      string `json:"url"`
	}

	// Source Code Management System
	SCM interface {
		ID() int64
		URL() *url.URL
		Client() *scm.Client
		RevisionURL(baseURL string, revision string) string
		LoginHandler(next http.Handler) http.Handler
	}

	contextKey int

	// Protocols

	SCMResponse struct {
		ID      int64  `json:"id"`
		URL     string `json:"url"`
		Logined bool   `json:"logined"`
	}

	SCMStore interface {
		Init() error
		FindURL(string) (int64, string, error)
		Insert(driver string, url string) (int64, error)
	}

	RepositoryStore interface {
		Init() error
		Find(id int64) (Repository, error)
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
		scms     []SCM
		repos    RepositoryStore
		coverage ResourceHandler

		sessionManager     *MoraSessionManager
		frontendFileServer http.Handler
	}
)

const (
	contextRepoKey contextKey = iota
	contextSCMKey  contextKey = iota
)

func WithSCM(ctx context.Context, scm SCM) context.Context {
	return context.WithValue(ctx, contextSCMKey, scm)
}

func SCMFrom(ctx context.Context) (SCM, bool) {
	scm, ok := ctx.Value(contextSCMKey).(SCM)
	return scm, ok
}

func WithRepo(ctx context.Context, repo Repository) context.Context {
	return context.WithValue(ctx, contextRepoKey, repo)
}

func RepoFrom(ctx context.Context) (Repository, bool) {
	repo, ok := ctx.Value(contextRepoKey).(Repository)
	return repo, ok
}

func (s *MoraServer) findSCM(id int64) SCM {
	for _, scm := range s.scms {
		if scm.ID() == id {
			return scm
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

	resp := []Repository{}
	sess, _ := MoraSessionFrom(r.Context())

	for _, repo := range repositories {
		scm := s.findSCM(repo.SCM)
		if scm == nil {
			log.Warn().Msgf(
				"scm not found for repository: repo.ID=%d scm.ID=%d (skipped)",
				repo.ID, repo.SCM)
			continue
		}

		err = checkRepoAccess(sess, scm, repo)
		if err == nil {
			resp = append(resp, repo)
		}
	}

	render.JSON(w, resp, http.StatusOK)
}

func (s *MoraServer) handleSCMList(w http.ResponseWriter, r *http.Request) {
	resp := []SCMResponse{}
	sess, _ := MoraSessionFrom(r.Context())

	for _, scm := range s.scms {
		_, ok := sess.getToken(scm.ID())
		resp = append(resp, SCMResponse{
			ID:      scm.ID(),
			URL:     scm.URL().String(),
			Logined: ok,
		})
	}

	render.JSON(w, resp, 200)
}

func checkRepoAccessBySCM(session *MoraSession, scm SCM, owner, name string) error {
	ctx, err := session.WithToken(context.Background(), scm.ID())
	if err != nil {
		return err // errorTokenNotFound
	}

	_, _, err = scm.Client().Repositories.Find(ctx, owner+"/"+name)
	if err != nil {
		return err
	}

	return nil
}

// checkRepoAccess checks if token in session can access a repo 'owner/name'
func checkRepoAccess(sess *MoraSession, scm SCM, repo Repository) error {
	cache := sess.getReposCache(scm.ID())
	_, ok := cache[repo.ID]
	if ok {
		log.Print("checkRepoAccess: found in cache")
		return nil
	}

	err := checkRepoAccessBySCM(sess, scm, repo.Namespace, repo.Name)
	if err != nil {
		log.Print("checkRepoAccess: no repo or no access at SCM")
		return err
	}
	log.Print("checkRepoAccess: found in SCM")

	// store cache
	if cache == nil {
		cache = map[int64]bool{}
	}
	cache[repo.ID] = true
	sess.setReposCache(scm.ID(), cache)

	return err
}

func (s *MoraServer) injectRepo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repo_id, err := strconv.ParseInt(chi.URLParam(r, "repo_id"), 10, 64)
		if err != nil {
			log.Err(err).Msg("")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		log.Print("injectRepo: repo_id=", repo_id)

		repo, err := s.repos.Find(repo_id)
		if err != nil {
			log.Err(err).Msg("")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		scm := s.findSCM(repo.SCM)
		if scm == nil {
			log.Error().Msgf("scm not found: id=%d", repo.SCM)
			render.NotFound(w, render.ErrNotFound)
			return
		}

		sess, _ := MoraSessionFrom(r.Context())
		err = checkRepoAccess(sess, scm, repo)
		if err == errorTokenNotFound {
			render.Forbidden(w, render.ErrForbidden)
			return
		} else if err != nil {
			log.Err(err).Msg("injectRepo")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		ctx := r.Context()
		ctx = WithSCM(ctx, scm)
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

	r.Get("/api/scms", s.handleSCMList)

	r.Route("/api/repos", func(r chi.Router) {
		r.Get("/", s.handleRepoList)
		r.Route("/{repo_id}", func(r chi.Router) {
			r.Use(s.injectRepo)
			if s.coverage != nil {
				r.Mount("/coverages", s.coverage.Handler())
			}
		})
	})

	r.Post("/api/upload", s.HandleUpload)

	// login/logout

	redirectHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/scms", http.StatusSeeOther)
		})

	r.Mount("/login", LoginHandler(s.scms, redirectHandler))
	r.Mount("/logout", LogoutHandler(s.scms, redirectHandler))

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

func initSCM(config SCMConfig, baseURL string, store SCMStore) (SCM, error) {
	if config.Driver == "github" && config.URL == "" {
		config.URL = "https://github.com"
	}

	if config.URL == "" {
		return nil, errors.New("ConfigError: scm.url is empty")
	}

	if config.SecretFilename == "" {
		return nil, errors.New("ConfigError: scm.secret_url is empty")
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
		log.Info().Msgf("New scm is configured. ID=%d Driver=%s URL=%s",
			id, config.Driver, config.URL)
	} else {
		log.Info().Msgf("scm enabled. ID=%d Driver=%s URL=%s",
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

	return nil, fmt.Errorf("ConfigError: unknown scm: %s", config.Driver)
}

func initSCMs(config MoraConfig, store SCMStore) ([]SCM, error) {
	scms := []SCM{}
	for _, scmConfig := range config.SCMs {
		scm, err := initSCM(scmConfig, config.Server.URL, store)
		if err != nil {
			return nil, err
		}
		scms = append(scms, scm)
	}

	return scms, nil
}

func initStore(filename string) (SCMStore, RepositoryStore, CoverageStore, error) {
	log.Info().Msgf("Initialize store: filename=%s", filename)

	db, err := sqlx.Connect("sqlite3", filename)
	if err != nil {
		return nil, nil, nil, err
	}

	scmStore := NewSCMStore(db)
	if err := scmStore.Init(); err != nil {
		return nil, nil, nil, err
	}

	repoStore := NewRepositoryStore(db)
	if err := repoStore.Init(); err != nil {
		return nil, nil, nil, err
	}

	covStore := NewCoverageStore(db)
	if err := covStore.Init(); err != nil {
		return nil, nil, nil, err
	}

	return scmStore, repoStore, covStore, nil
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

	scmStore, repoStore, covStore, err := initStore(config.DatabaseFilename)
	if err != nil {
		log.Err(err).Msg("initStore")
		return nil, err
	}

	scms, err := initSCMs(config, scmStore)
	if err != nil {
		return nil, err
	}
	if len(scms) == 0 {
		return nil, errors.New("no SCM is configured")
	}

	frontendFileServer, err := initFrontendFileServer(config)
	if err != nil {
		return nil, err
	}

	coverage := NewCoverageHandler(repoStore, covStore)

	s := &MoraServer{
		sessionManager:     NewMoraSessionManager(),
		scms:               scms,
		repos:              repoStore,
		frontendFileServer: frontendFileServer,
		coverage:           coverage,
	}

	return s, err
}
