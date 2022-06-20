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

// scm client
type Client interface {
	Name() string // unique name in mora
	URL() *url.URL
	RevisionURL(repo *Repo, revision string) string
	LoginHandler(next http.Handler) http.Handler
	ListRepos(token *scm.Token) ([]*Repo, error)
}

type contextKey int

const (
	contextRepoKey contextKey = iota
	contextSCMKey  contextKey = iota
)

func WithSCM(ctx context.Context, scm Client) context.Context {
	return context.WithValue(ctx, contextSCMKey, scm)
}

func SCMFrom(ctx context.Context) (Client, bool) {
	scm, ok := ctx.Value(contextSCMKey).(Client)
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

func HandleRepoList(clients []Client, provider CoverageProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("HandleRepoList")

		urls := provider.Repos()

		repos := []RepoResponse{}
		sess, _ := MoraSessionFrom(r.Context())
		for _, client := range clients {
			tmp, err := getReposWithCache(client, sess)
			if err == errorTokenNotFound {
				// ignore
			} else if err != nil {
				render.NotFound(w, render.ErrNotFound)
				return
			}

			for _, repo := range tmp {
				for _, link := range urls {
					if repo.Link == link {
						repos = append(repos, RepoResponse{
							client.Name(), repo.Namespace, repo.Name, repo.Link})
					}
				}
			}
		}

		render.JSON(w, repos, http.StatusOK)
	}
}

type SCMResponse struct {
	URL     string `json:"url"`
	Name    string `json:"name"`
	Logined bool   `json:"logined"`
}

func HandleSCMList(clients []Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scms := []SCMResponse{}
		sess, _ := MoraSessionFrom(r.Context())

		for _, client := range clients {
			_, ok := sess.getToken(client.Name())
			scms = append(scms, SCMResponse{
				URL:     client.URL().String(),
				Name:    client.Name(),
				Logined: ok,
			})
		}

		render.JSON(w, scms, 200)
	}
}

func HandleSync(provider CoverageProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider.Sync()
	}
}

// Web Handler

type TemplateLoader struct {
	fsys fs.FS
}

var templateLoader *TemplateLoader

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

func getReposWithCache(client Client, session *MoraSession) ([]*Repo, error) {
	repos, ok := session.getReposCache(client.Name())
	if ok {
		log.Print("Load repos from session for ", client.Name())
		return repos, nil
	}

	log.Print("try to load repos from scm: ", client.Name())

	token, ok := session.getToken(client.Name())
	if !ok {
		log.Print("token for scm was not found in session: ", client.Name())
		return nil, errorTokenNotFound
	}

	repos, err := client.ListRepos(&token)
	if err == nil {
		log.Print("Store repos to cache")
		session.setReposCache(client.Name(), repos)
	}

	return repos, err
}

func findClient(clients []Client, name string) (Client, bool) {
	for _, client := range clients {
		if client.Name() == name {
			return client, true
		}
	}
	return nil, false
}

func findRepo(sess *MoraSession, client Client, owner, name string) (*Repo, error) {
	repos, err := getReposWithCache(client, sess)
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

func injectRepo(clients []Client) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scm := chi.URLParam(r, "scm")
			owner := chi.URLParam(r, "owner")
			repoName := chi.URLParam(r, "repo")

			log.Print("injectRepo: scm=", scm)

			client, ok := findClient(clients, scm)
			if !ok {
				log.Error().Msgf("repoChecker: unknown scm: %s", scm)
				render.NotFound(w, render.ErrNotFound)
				return
			}

			sess, _ := MoraSessionFrom(r.Context())
			repo, err := findRepo(sess, client, owner, repoName)
			if err == errorTokenNotFound {
				http.Redirect(w, r, "/scms", http.StatusSeeOther)
				return
			} else if err != nil {
				log.Err(err).Msg("injectRepo")
				render.NotFound(w, render.ErrNotFound)
				return
			}

			ctx := r.Context()
			ctx = WithSCM(ctx, client)
			ctx = WithRepo(ctx, repo)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (s *MoraServer) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(s.sessionManager.SessionMiddleware)

	// api

	r.Post("/api/sync", HandleSync(s.provider))
	r.Get("/api/scms", HandleSCMList(s.clients))
	r.Get("/api/repos", HandleRepoList(s.clients, s.provider))

	r.Route("/api/{scm}/{owner}/{repo}", func(r chi.Router) {
		r.Use(injectRepo(s.clients))
		r.Mount("/coverages", HandleCoverage(s.provider))
	})

	// web

	redirectHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/scms", http.StatusSeeOther)
		})

	r.Get("/", templateRenderingHandler("index.html"))
	r.Get("/scms", templateRenderingHandler("login.html"))

	r.Mount("/login", LoginHandler(s.clients, redirectHandler))
	r.Mount("/logout", LogoutHandler(s.clients, redirectHandler))

	r.Route("/{scm}/{owner}/{repo}", func(r chi.Router) {
		r.Use(injectRepo(s.clients))
		r.Mount("/coverages", CoverageWebHandler(s.provider))
	})

	r.Get("/public/*", func(w http.ResponseWriter, r *http.Request) {
		fs := http.StripPrefix("/public/", s.publicFileServer)
		fs.ServeHTTP(w, r)
	})

	return r
}

type MoraServer struct {
	clients  []Client
	provider CoverageProvider

	sessionManager   *MoraSessionManager
	publicFileServer http.Handler
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

func NewMoraServer(clients []Client, provider CoverageProvider, debug bool) (*MoraServer, error) {
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
	s.clients = clients
	s.provider = provider
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

func createClients(config MoraConfig) []Client {
	clients := []Client{}
	for _, scmConfig := range config.ScmConfigs {
		log.Print(scmConfig.Type)
		var client Client
		var err error
		if scmConfig.Type == "gitea" {
			client, err = NewGiteaFromFile(
				scmConfig.Name,
				scmConfig.SecretFilename,
				scmConfig.URL,
				config.URL+"/login/"+scmConfig.Name)
		} else if scmConfig.Type == "github" {
			client, err = NewGithubFromFile(
				scmConfig.Name,
				scmConfig.SecretFilename)
		} else {
			err = fmt.Errorf("unknown scm: %s", scmConfig.Type)
		}

		if err != nil {
			log.Warn().Err(err).Msgf(
				"ignore error during %s initialization", scmConfig.Name)
		} else {
			clients = append(clients, client)
		}
	}

	return clients
}

func NewMoraServerFromConfig(config MoraConfig) (*MoraServer, error) {
	clients := createClients(config)
	if len(clients) == 0 {
		return nil, errors.New("no client is configured")
	}

	dir := os.DirFS("data") // TODO
	provider := NewHTMLCoverageProvider(dir)
	err := provider.Sync()
	if err != nil {
		return nil, err
	}

	log.Print("config.Debug=", config.Debug)
	return NewMoraServer(clients, provider, config.Debug)
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
