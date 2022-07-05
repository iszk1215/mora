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
	"text/template"

	"github.com/drone/drone/handler/api/render"
	"github.com/drone/go-scm/scm"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
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

func (s *MoraServer) HandleRepoList(w http.ResponseWriter, r *http.Request) {
	log.Print("HandleRepoList")

	repos := []RepoResponse{}
	sess, _ := MoraSessionFrom(r.Context())
	for _, scm := range s.smcs {
		tmp, err := getReposWithCache(scm, sess)
		if err == errorTokenNotFound {
			// ignore
		} else if err != nil {
			render.NotFound(w, render.ErrNotFound)
			return
		}

		for _, repo := range tmp {
			for _, link := range s.coverage.Repos() {
				// log.Print(link)
				if repo.Link == link {
					repos = append(repos, RepoResponse{
						scm.Name(), repo.Namespace, repo.Name, repo.Link})
				}
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

func HandleSCMList(scms []SCM) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := []SCMResponse{}
		sess, _ := MoraSessionFrom(r.Context())

		for _, scm := range scms {
			_, ok := sess.getToken(scm.Name())
			resp = append(resp, SCMResponse{
				URL:     scm.URL().String(),
				Name:    scm.Name(),
				Logined: ok,
			})
		}

		render.JSON(w, resp, 200)
	}
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
		log.Print("Errror here")
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

func listRepos(ctx context.Context, client *scm.Client) ([]*Repo, error) {
	ret := []*Repo{}
	opts := scm.ListOptions{Size: 100}
	for {
		result, meta, err := client.Repositories.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		ret = append(ret, result...)

		opts.Page = meta.Page.Next
		opts.URL = meta.Page.NextURL

		if opts.Page == 0 && opts.URL == "" {
			break
		}
	}
	return ret, nil
}

func getReposWithCache(scm SCM, session *MoraSession) ([]*Repo, error) {
	repos, ok := session.getReposCache(scm.Name())
	if ok {
		log.Print("Load repos from session for ", scm.Name())
		return repos, nil
	}

	ctx, err := session.WithToken(context.Background(), scm.Name())
	if err != nil {
		return nil, err
	}

	log.Print("try to load repos from scm: ", scm.Name())
	repos, err = listRepos(ctx, scm.Client())
	if err == nil {
		log.Print("Store repos to cache")
		session.setReposCache(scm.Name(), repos)
	}

	return repos, err
}

func findSCM(scms []SCM, name string) (SCM, bool) {
	for _, scm := range scms {
		if scm.Name() == name {
			return scm, true
		}
	}
	return nil, false
}

// checkRepoAccess checks if token in session can access a repo 'owner/name'
func checkRepoAccess(sess *MoraSession, scm SCM, owner, name string) (*Repo, error) {
	repos, err := getReposWithCache(scm, sess)
	if err != nil {
		return nil, err
	}

	for _, repo := range repos {
		if repo.Namespace == owner && repo.Name == name {
			return repo, nil
		}
	}

	return nil, fmt.Errorf("requested repo not exist or not visible: %s/%s",
		owner, name)
}

func injectRepo(scms []SCM) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scmName := chi.URLParam(r, "scm")
			owner := chi.URLParam(r, "owner")
			repoName := chi.URLParam(r, "repo")

			log.Print("injectRepo: scmName=", scmName)

			scm, ok := findSCM(scms, scmName)
			if !ok {
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
	if s.tool != nil {
		s.tool.HandleUpload(w, r)
		s.coverage.Sync()
	}
}

func (s *MoraServer) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(s.sessionManager.SessionMiddleware)

	// api

	r.Post("/api/sync", s.HandleSync)
	r.Get("/api/scms", HandleSCMList(s.smcs))
	r.Get("/api/repos", s.HandleRepoList)
	r.Post("/api/upload", s.HandleUpload)

	r.Route("/api/{scm}/{owner}/{repo}", func(r chi.Router) {
		r.Use(injectRepo(s.smcs))
		r.Mount("/coverages", s.coverage.APIHandler())
	})

	// web

	redirectHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/scms", http.StatusSeeOther)
		})

	r.Get("/", templateRenderingHandler("index.html"))
	r.Get("/scms", templateRenderingHandler("login.html"))

	r.Mount("/login", LoginHandler(s.smcs, redirectHandler))
	r.Mount("/logout", LogoutHandler(s.smcs, redirectHandler))

	r.Route("/{scm}/{owner}/{repo}", func(r chi.Router) {
		r.Use(injectRepo(s.smcs))
		r.Mount("/coverages", s.coverage.WebHandler())
	})

	r.Get("/public/*", func(w http.ResponseWriter, r *http.Request) {
		fs := http.StripPrefix("/public/", s.publicFileServer)
		fs.ServeHTTP(w, r)
	})

	return r
}

type MoraServer struct {
	smcs     []SCM
	coverage *CoverageService

	sessionManager   *MoraSessionManager
	publicFileServer http.Handler

	tool *ToolCoverageProvider
	html *HTMLCoverageProvider
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
	s.smcs = scms
	s.publicFileServer = http.FileServer(http.FS(publicFS))

	return s, nil
}

type ScmConfig struct {
	Type           string `yaml:"type"`
	Name           string `yaml:"name"`
	SecretFilename string `yaml:"secret_filename"`
	URL            string `yaml:"url"`
}

type MoraConfig struct {
	URL        string      `yaml:"url"`
	Port       string      `yaml:"port"`
	ScmConfigs []ScmConfig `yaml:"scms"`
	Debug      bool
}

func createSMCs(config MoraConfig) []SCM {
	scms := []SCM{}
	for _, scmConfig := range config.ScmConfigs {
		log.Print(scmConfig.Type)
		var scm SCM
		var err error
		if scmConfig.Type == "gitea" {
			scm, err = NewGiteaFromFile(
				scmConfig.Name,
				scmConfig.SecretFilename,
				scmConfig.URL,
				config.URL+"/login/"+scmConfig.Name)
		} else if scmConfig.Type == "github" {
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

func NewMoraServerFromConfig(config MoraConfig) (*MoraServer, error) {
	scms := createSMCs(config)
	if len(scms) == 0 {
		return nil, errors.New("no SCM is configured")
	}

	log.Print("config.Debug=", config.Debug)
	s, err := NewMoraServer(scms, config.Debug)
	if err != nil {
		return nil, err
	}

	dir := os.DirFS("data") // TODO
	html := NewHTMLCoverageProvider(dir)
	tool := NewToolCoverageProvider()

	coverage := NewCoverageService()
	coverage.AddProvider(tool)
	coverage.AddProvider(html)
	coverage.SyncProviders()
	coverage.Sync()

	s.coverage = coverage
	s.html = html
	s.tool = tool

	return s, nil
}

func ReadMoraConfig(filename string) (MoraConfig, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return MoraConfig{}, err
	}
	config := MoraConfig{}
	if err := yaml.Unmarshal(b, &config); err != nil {
		return MoraConfig{}, err
	}

	return config, nil
}
