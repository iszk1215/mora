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
	"strings"

	"github.com/drone/drone/handler/api/render"
	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

var (
	errorTokenNotFound = errors.New("token not found in a session")
)

type (
	//Repo = scm.Repository

	Repository struct {
		ID        int64
		Namespace string
		Name      string
		Link      string
	}

	// Source Code Management System
	SCM interface {
		Name() string // unique name in mora
		URL() *url.URL
		Client() *scm.Client
		RevisionURL(baseURL string, revision string) string
		LoginHandler(next http.Handler) http.Handler
	}

	contextKey int

	RepoResponse struct {
		SCM       string `json:"scm"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		Link      string `json:"link"`
	}

	SCMResponse struct {
		URL     string `json:"url"`
		Name    string `json:"name"`
		Logined bool   `json:"logined"`
	}

	RepositoryService interface {
		FindRepoByURL(string) (Repository, bool)
	}

	MoraServer struct {
		scms         []SCM
		repositories []Repository
		coverage     *CoverageHandler

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

// API Handler

func parseRepoURL(str string) (string, string, string, error) {
	tmp := strings.Split(str, "/")
	if len(tmp) != 5 {
		return "", "", "", fmt.Errorf("invalid repo url: %s", str)
	}

	scm := tmp[0] + "//" + tmp[2]
	owner := tmp[3]
	name := tmp[4]

	return scm, owner, name, nil
}

func (s *MoraServer) FindRepoByURL(url string) (Repository, bool) {
	for _, repo := range s.repositories {
		if repo.Link == url {
			return repo, true
		}
	}

	return Repository{}, false
}

func (s *MoraServer) findRepoByID(id int64) (Repository, bool) {
	for _, repo := range s.repositories {
		if repo.ID == id {
			return repo, true
		}
	}

	return Repository{}, false
}

func (s *MoraServer) handleRepoList(w http.ResponseWriter, r *http.Request) {
	log.Print("HandleRepoList")

	repos := []RepoResponse{}
	sess, _ := MoraSessionFrom(r.Context())

	if s.coverage != nil {
		for _, repoID := range s.coverage.Repos() {
			repo, ok := s.findRepoByID(repoID)
			if !ok {
				log.Error().Msgf("repoID not found: %d", repoID)
				render.NotFound(w, render.ErrNotFound)
				return
			}
			scmURL, owner, name, err := parseRepoURL(repo.Link)
			if err != nil {
				log.Err(err).Msg("")
				render.NotFound(w, render.ErrNotFound)
				return
			}

			scm := findSCMFromURL(s.scms, scmURL)
			if scm == nil {
				log.Print("scm not found for repository: ", repoID, " (skipped)")
				continue
			}

			repo, err = checkRepoAccess(sess, scm, owner, name)
			if err == nil {
				repos = append(repos, RepoResponse{
					scm.Name(), repo.Namespace, repo.Name, repo.Link})
			}
		}
	}

	render.JSON(w, repos, http.StatusOK)
}

func (s *MoraServer) handleSCMList(w http.ResponseWriter, r *http.Request) {
	resp := []SCMResponse{}
	sess, _ := MoraSessionFrom(r.Context())

	for _, scm := range s.scms {
		_, ok := sess.getToken(scm.Name())
		resp = append(resp, SCMResponse{
			URL:     scm.URL().String(),
			Name:    scm.Name(),
			Logined: ok,
		})
	}

	render.JSON(w, resp, 200)
}

// ----------------------------------------------------------------------

func findSCM(list []SCM, f func(scm SCM) bool) SCM {
	for _, scm := range list {
		if f(scm) {
			return scm
		}
	}
	return nil
}

func findSCMFromName(scms []SCM, name string) SCM {
	return findSCM(scms, func(scm SCM) bool { return scm.Name() == name })
}

func findSCMFromURL(scms []SCM, url string) SCM {
	return findSCM(scms, func(scm SCM) bool {
		tmp := scm.URL().String()
		tmp = strings.TrimSuffix(tmp, "/")
		return tmp == url
	})
}

func findRepoFromSCM(session *MoraSession, scm SCM, owner, name string) (Repository, error) {
	ctx, err := session.WithToken(context.Background(), scm.Name())
	if err != nil {
		return Repository{}, err
	}

	repo, meta, err := scm.Client().Repositories.Find(ctx, owner+"/"+name)
	if err != nil {
		log.Print(meta)
		return Repository{}, err
	}

	return Repository{
		Name:      repo.Name,
		Namespace: repo.Namespace,
		Link:      repo.Link,
	}, nil
}

// checkRepoAccess checks if token in session can access a repo 'owner/name'
func checkRepoAccess(sess *MoraSession, scm SCM, owner, name string) (Repository, error) {
	cache := sess.getReposCache(scm.Name())
	key := owner + "/" + name
	repo, ok := cache[key]
	if ok {
		log.Print("checkRepoAccess: found in cache")
		return repo, nil
	}

	repo, err := findRepoFromSCM(sess, scm, owner, name)
	if err == nil {
		log.Print("checkRepoAccess: found in SCM")
	} else {
		log.Print("checkRepoAccess: no repo or no access")
		return Repository{}, err
	}

	// store cache
	if cache == nil {
		cache = map[string]Repository{}
	}
	cache[key] = repo
	sess.setReposCache(scm.Name(), cache)

	return repo, err
}

func (s *MoraServer) injectRepo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scmName := chi.URLParam(r, "scm")
		owner := chi.URLParam(r, "owner")
		repoName := chi.URLParam(r, "repo")

		log.Print("injectRepo: scmName=", scmName)

		scm := findSCMFromName(s.scms, scmName)
		if scm == nil {
			log.Error().Msgf("repoChecker: unknown scm: %s", scmName)
			render.NotFound(w, render.ErrNotFound)
			return
		}

		sess, _ := MoraSessionFrom(r.Context())
		scmRepo, err := checkRepoAccess(sess, scm, owner, repoName)
		if err == errorTokenNotFound {
			render.Forbidden(w, render.ErrForbidden)
			return
		} else if err != nil {
			log.Err(err).Msg("injectRepo")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		// TODO: move to checkRepoAccess
		repository, _ := s.FindRepoByURL(scmRepo.Link)

		ctx := r.Context()
		ctx = WithSCM(ctx, scm)
		ctx = WithRepo(ctx, repository)
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
	r.Get("/api/repos", s.handleRepoList)

	r.Post("/api/upload", s.HandleUpload)

	r.Route("/api/{scm}/{owner}/{repo}", func(r chi.Router) {
		r.Use(s.injectRepo)
		r.Mount("/coverages", s.coverage.Handler())
	})

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

// static includes public and templates
//go:embed static
var embedded embed.FS

func getStaticFS(staticDir string, path string, debug bool) (fs.FS, error) {
	if debug {
		return os.DirFS(filepath.Join(staticDir, path)), nil
	}

	return fs.Sub(embedded, filepath.Join("static", path))
}

func NewMoraServer(scms []SCM, debug bool) (*MoraServer, error) {
	s := &MoraServer{}

	sessionManager := NewMoraSessionManager()

	staticDir := "mora/server/static"
	frontendFS, err := getStaticFS(staticDir, "public", debug)
	if err != nil {
		return nil, err
	}

	s.sessionManager = sessionManager
	s.scms = scms
	s.frontendFileServer = http.FileServer(http.FS(frontendFS))

	return s, nil
}

func createSCMs(config MoraConfig) []SCM {
	scms := []SCM{}
	for _, scmConfig := range config.SCMs {
		log.Print(scmConfig.Type)
		var scm SCM
		var err error
		if scmConfig.Type == "gitea" {
			if scmConfig.Name == "" {
				scmConfig.Name = "gitea"
			}
			scm, err = NewGiteaFromFile(
				scmConfig.Name,
				scmConfig.SecretFilename,
				scmConfig.URL,
				config.Server.URL+"/login/"+scmConfig.Name)
		} else if scmConfig.Type == "github" {
			if scmConfig.Name == "" {
				scmConfig.Name = "github"
			}
			scm, err = NewGithubFromFile(
				scmConfig.Name,
				scmConfig.SecretFilename)
		} else {
			err = fmt.Errorf("unknown scm: %s", scmConfig.Type)
		}

		if err != nil {
			log.Warn().Err(err).Msgf(
				"ignore error during %s initialization", scmConfig.Name)
		} else {
			scms = append(scms, scm)
		}
	}

	return scms
}

func initStore() (RepositoryStore, CoverageStore, error) {
	db, err := Connect("mora.db")
	if err != nil {
		return nil, nil, err
	}

	rs := NewRepositoryStore(db)
	if err := rs.Init(); err != nil {
		return nil, nil, err
	}

	cs := NewCoverageStore(db)

	return rs, cs, nil
}

func NewMoraServerFromConfig(config MoraConfig) (*MoraServer, error) {
	scms := createSCMs(config)
	if len(scms) == 0 {
		return nil, errors.New("no SCM is configured")
	}

	log.Print("config.Debug=", config.Debug)
	s, err := NewMoraServer(scms, config.Debug)
	if err != nil {
		return nil, err
	}

	repoStore, covStore, err := initStore()
	if err != nil {
		log.Err(err).Msg("initStore")
		return nil, err
	}
	moraCoverageProvider := NewMoraCoverageProvider(covStore)
	coverage := NewCoverageHandler(moraCoverageProvider, s)

	s.coverage = coverage
	s.repositories, err = repoStore.Scan()

	return s, err
}
