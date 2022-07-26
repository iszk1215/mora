package mora

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
	"text/template"

	"github.com/drone/drone/handler/api/render"
	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

var (
	errorTokenNotFound = errors.New("token not found in a session")
)

type Repo = scm.Repository

// Source Code Management System
type SCM interface {
	Name() string // unique name in mora
	URL() *url.URL
	Client() *scm.Client
	RevisionURL(repo *Repo, revision string) string
	LoginHandler(next http.Handler) http.Handler
}

type contextKey int

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

func WithRepo(ctx context.Context, repo *Repo) context.Context {
	return context.WithValue(ctx, contextRepoKey, repo)
}

func RepoFrom(ctx context.Context) (*Repo, bool) {
	repo, ok := ctx.Value(contextRepoKey).(*Repo)
	return repo, ok
}

// API Handler

type RepoResponse struct {
	SCM       string `json:"scm"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Link      string `json:"link"`
}

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

func (s *MoraServer) handleRepoList(w http.ResponseWriter, r *http.Request) {
	log.Print("HandleRepoList")

	repos := []RepoResponse{}
	sess, _ := MoraSessionFrom(r.Context())

	if s.coverage != nil {
		for _, link := range s.coverage.Repos() {
			scmURL, owner, name, err := parseRepoURL(link)
			if err != nil {
				log.Err(err).Msg("")
				render.NotFound(w, render.ErrNotFound)
				return
			}

			scm := findSCMFromURL(s.scms, scmURL)
			if scm == nil {
				log.Print("scm not found for repository: ", link, " (skipped)")
				continue
			}

			repo, err := checkRepoAccess(sess, scm, owner, name)
			if err == nil {
				repos = append(repos, RepoResponse{
					scm.Name(), repo.Namespace, repo.Name, repo.Link})
			}
		}
	}

	render.JSON(w, repos, http.StatusOK)
}

type SCMResponse struct {
	URL     string `json:"url"`
	Name    string `json:"name"`
	Logined bool   `json:"logined"`
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

func (s *MoraServer) HandleSync(w http.ResponseWriter, r *http.Request) {
	s.coverage.Sync()
}

// Web Handler

type TemplateLoader struct {
	fsys fs.FS
}

func NewTemplateLoader(fsys fs.FS) *TemplateLoader {
	return &TemplateLoader{fsys}
}

func (l *TemplateLoader) load(filename string) (*template.Template, error) {
	t, err := template.ParseFS(l.fsys, filename, "header.html", "footer.html")
	if err != nil {
		return nil, err
	}
	return t, nil
}

var templateLoader *TemplateLoader

func initTemplateLoader(fsys fs.FS) {
	templateLoader = NewTemplateLoader(fsys)
}

func loadTemplate(filename string) (*template.Template, error) {
	return templateLoader.load(filename)
}

func renderTemplate(w http.ResponseWriter, filename string) error {
	templ, err := loadTemplate(filename)
	if err != nil {
		return err
	}
	return templ.Execute(w, nil)
}

func templateRenderingHandler(filename string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := renderTemplate(w, filename)
		if err != nil {
			log.Err(err).Msg(filename)
			render.NotFound(w, render.ErrNotFound)
		}
	}
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

func findRepoFromSCM(session *MoraSession, scm SCM, owner, name string) (*Repo, error) {
	ctx, err := session.WithToken(context.Background(), scm.Name())
	if err != nil {
		return nil, err
	}

	repo, meta, err := scm.Client().Repositories.Find(ctx, owner+"/"+name)
	if err != nil {
		log.Print(meta)
		return nil, err
	}

	return repo, nil
}

// checkRepoAccess checks if token in session can access a repo 'owner/name'
func checkRepoAccess(sess *MoraSession, scm SCM, owner, name string) (*Repo, error) {
	cache := sess.getReposCache(scm.Name())
	key := owner + "/" + name
	repo := cache[key]
	if repo != nil {
		log.Print("checkRepoAccess: found in cache")
		return repo, nil
	}

	repo, err := findRepoFromSCM(sess, scm, owner, name)
	if err == nil {
		log.Print("checkRepoAccess: found in SCM")
	} else {
		log.Print("checkRepoAccess: no repo or no access")
	}

	// store cache
	if cache == nil {
		cache = map[string]*Repo{}
	}
	cache[key] = repo
	sess.setReposCache(scm.Name(), cache)

	return repo, err
}

func injectRepo(scms []SCM) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scmName := chi.URLParam(r, "scm")
			owner := chi.URLParam(r, "owner")
			repoName := chi.URLParam(r, "repo")

			log.Print("injectRepo: scmName=", scmName)

			scm := findSCMFromName(scms, scmName)
			if scm == nil {
				log.Error().Msgf("repoChecker: unknown scm: %s", scmName)
				render.NotFound(w, render.ErrNotFound)
				return
			}

			sess, _ := MoraSessionFrom(r.Context())
			repo, err := checkRepoAccess(sess, scm, owner, repoName)
			if err == errorTokenNotFound {
				http.Redirect(w, r, "/scms", http.StatusSeeOther)
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
}

func (s *MoraServer) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if s.moraCoverageProvider != nil {
		s.moraCoverageProvider.HandleUpload(w, r)
		s.coverage.Sync()
	}
}

func (s *MoraServer) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(s.sessionManager.SessionMiddleware)

	// api

	r.Post("/api/sync", s.HandleSync)
	r.Get("/api/scms", s.handleSCMList)
	r.Get("/api/repos", s.handleRepoList)

	r.Post("/api/upload", s.HandleUpload)

	r.Route("/api/{scm}/{owner}/{repo}", func(r chi.Router) {
		r.Use(injectRepo(s.scms))
		r.Mount("/coverages", s.coverage.APIHandler())
	})

	// web

	redirectHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/scms", http.StatusSeeOther)
		})

	r.Get("/", templateRenderingHandler("index.html"))
	r.Get("/scms", templateRenderingHandler("login.html"))

	r.Mount("/login", LoginHandler(s.scms, redirectHandler))
	r.Mount("/logout", LogoutHandler(s.scms, redirectHandler))

	r.Route("/{scm}/{owner}/{repo}", func(r chi.Router) {
		r.Use(injectRepo(s.scms))
		r.Mount("/coverages", s.coverage.WebHandler())
	})

	r.Get("/public/*", func(w http.ResponseWriter, r *http.Request) {
		fs := http.StripPrefix("/public/", s.publicFileServer)
		fs.ServeHTTP(w, r)
	})

	return r
}

type MoraServer struct {
	scms     []SCM
	coverage *CoverageService

	sessionManager   *MoraSessionManager
	publicFileServer http.Handler

	moraCoverageProvider *MoraCoverageProvider
	htmlCoverageProvider *HTMLCoverageProvider
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

	staticDir := "mora/static" // FIXME
	fsys, err := getStaticFS(staticDir, "templates", debug)
	if err != nil {
		return nil, err
	}
	initTemplateLoader(fsys)

	publicFS, err := getStaticFS(staticDir, "public", debug)
	if err != nil {
		return nil, err
	}

	s.sessionManager = sessionManager
	s.scms = scms
	s.publicFileServer = http.FileServer(http.FS(publicFS))

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

func initStoreForCoverageProvider() (*CoverageStore, error) {
	db, err := Connect("mora.db")
	if err != nil {
		return nil, err
	}
	return NewCoverageStore(db), nil
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

	dir := os.DirFS("data") // TODO
	htmlCoverageProvider := NewHTMLCoverageProvider(dir)

	store, err := initStoreForCoverageProvider()
	if err != nil {
		return nil, err
	}
	moraCoverageProvider := NewMoraCoverageProvider(store)

	coverage := NewCoverageService()
	coverage.AddProvider(moraCoverageProvider)
	coverage.AddProvider(htmlCoverageProvider)
	coverage.SyncProviders()
	coverage.Sync()

	s.coverage = coverage
	s.htmlCoverageProvider = htmlCoverageProvider
	s.moraCoverageProvider = moraCoverageProvider

	if err != nil {
		log.Err(err).Msg("init_store")
	}

	return s, nil
}
