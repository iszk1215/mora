package mora

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/drone/drone/handler/api/render"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type htmlCoverageEntry struct {
	Name_  string `yaml:"name"`
	Lines_ int    `yaml:"lines"`
	Hits_  int    `yaml:"hits"`
	File   string `yaml:"file"`
}

func (e htmlCoverageEntry) Name() string {
	return e.Name_
}

func (e htmlCoverageEntry) Lines() int {
	return e.Lines_
}

func (e htmlCoverageEntry) Hits() int {
	return e.Hits_
}

type htmlCoverage struct {
	RepoURL   string               `yaml:"repo"`
	Time_     time.Time            `yaml:"time"`
	Revision_ string               `yaml:"revision"`
	Directory string               `yaml:"directory"` // where html files are stored
	Entries_  []*htmlCoverageEntry `yaml:"entries"`
}

func (c htmlCoverage) Time() time.Time {
	return c.Time_
}

func (c htmlCoverage) Revision() string {
	return c.Revision_
}

func (c htmlCoverage) Entries() map[string]CoverageEntry {
	ret := map[string]CoverageEntry{}
	for _, e := range c.Entries_ {
		ret[e.Name()] = e
	}
	return ret
}

// Loader

func loadFile(data []byte, path string) (*htmlCoverage, error) {
	var cov htmlCoverage
	err := yaml.Unmarshal(data, &cov)
	if err != nil {
		return nil, err
	}

	if cov.Directory == "" {
		cov.Directory = path
	} else {
		if !strings.HasPrefix(cov.Directory, "/") {
			cov.Directory = filepath.Join(path, cov.Directory)
		}
	}

	return &cov, nil
}

func loadDirectory(dir fs.FS, path, configFilename string) ([]*htmlCoverage, error) {
	// If config file exists in the directory, do not search more
	data, err := fs.ReadFile(dir, configFilename)
	if err == nil {
		cov, err := loadFile(data, path)
		if err != nil {
			return nil, err
		}
		return []*htmlCoverage{cov}, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	entries, err := fs.ReadDir(dir, ".")
	if err != nil {
		return nil, err
	}

	var ret []*htmlCoverage
	for _, e := range entries {
		if e.IsDir() {
			sub, err := fs.Sub(dir, e.Name())
			if err != nil {
				return nil, err
			}
			covs, err := loadDirectory(
				sub, filepath.Join(path, e.Name()), configFilename)
			if err != nil {
				return nil, err
			}
			ret = append(ret, covs...)
		}
	}

	return ret, nil
}

func load(dir fs.FS, configFilename string) (map[string][]*htmlCoverage, error) {
	covs, err := loadDirectory(dir, "", configFilename)
	if err != nil {
		return nil, err
	}

	covmap := map[string][]*htmlCoverage{}
	for _, cov := range covs {
		covmap[cov.RepoURL] = append(covmap[cov.RepoURL], cov)
	}

	for _, lst := range covmap {
		sort.Slice(lst, func(i, j int) bool {
			return lst[i].Time().Before(lst[j].Time())
		})
	}

	return covmap, nil
}

// HTML Coverage Handlers

type htmlCoverageContextKey int

const (
	coverageEntryKey htmlCoverageContextKey = iota
)

func withEntry(ctx context.Context, entry htmlCoverageEntry) context.Context {
	return context.WithValue(ctx, coverageEntryKey, entry)
}

func htmlCoverageFrom(ctx context.Context) (htmlCoverage, bool) {
	tmp, ok := coverageFrom(ctx)
	if !ok {
		return htmlCoverage{}, false
	}
	cov, ok := tmp.(htmlCoverage)
	return cov, ok
}

func entryFrom(ctx context.Context) (Client, *Repo, htmlCoverage, htmlCoverageEntry, bool) {
	scm, ok := SCMFrom(ctx)
	if !ok {
		return nil, nil, htmlCoverage{}, htmlCoverageEntry{}, false
	}

	repo, ok := RepoFrom(ctx)
	if !ok {
		return nil, nil, htmlCoverage{}, htmlCoverageEntry{}, false
	}

	cov, ok := htmlCoverageFrom(ctx)
	if !ok {
		return nil, nil, htmlCoverage{}, htmlCoverageEntry{}, false
	}

	entry, ok := ctx.Value(coverageEntryKey).(htmlCoverageEntry)
	return scm, repo, cov, entry, ok
}

type HTMLCoverageProvider struct {
	dataDirectory  fs.FS
	configFilename string
	covmap         map[string][]*htmlCoverage
	repos          []string
	sync.Mutex
}

func NewHTMLCoverageProvider(dataDirectory fs.FS) *HTMLCoverageProvider {
	m := new(HTMLCoverageProvider)
	m.dataDirectory = dataDirectory
	m.configFilename = "mora.yaml"
	return m
}

func (m *HTMLCoverageProvider) reload() error {
	covmap, err := load(m.dataDirectory, m.configFilename)
	if err != nil {
		return err
	}

	repos := []string{}
	for repo := range covmap {
		repos = append(repos, repo)
	}

	m.Lock()
	defer m.Unlock()
	m.covmap = covmap
	m.repos = repos

	return nil
}

func (m *HTMLCoverageProvider) Repos() ([]string, error) {
	if len(m.covmap) == 0 {
		err := m.reload()
		if err != nil {
			return nil, err
		}
	}

	return m.repos, nil
}

func (m *HTMLCoverageProvider) CoveragesFor(repoURL string) ([]Coverage, error) {
	if err := m.reload(); err != nil {
		return nil, err
	}

	covs, ok := m.covmap[repoURL]
	if !ok {
		return nil, errors.New("unknow repo")
	}

	ret := []Coverage{}
	for _, c := range covs {
		ret = append(ret, *c)
	}
	return ret, nil
}

func CoverageEntryChecker(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Print("CoverageEntryChecker")
		entryName := chi.URLParam(r, "entry")

		cov, ok := htmlCoverageFrom(r.Context())
		if !ok {
			log.Error().Msg("unknown coverage")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		var entry *htmlCoverageEntry = nil
		for _, e := range cov.Entries_ {
			if e.Name_ == entryName {
				entry = e
			}
		}

		if entry == nil {
			log.Error().Msg("can not find entry")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		next.ServeHTTP(w, r.WithContext(withEntry(r.Context(), *entry)))
	})
}

func (m *HTMLCoverageProvider) handleCoverageEntryData(w http.ResponseWriter, r *http.Request) {
	cov, ok := htmlCoverageFrom(r.Context())
	if !ok {
		log.Error().Msg("no html coverage found")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	log.Print("cov.Directory=", cov.Directory)
	sub, err := fs.Sub(m.dataDirectory, cov.Directory)
	if err != nil {
		log.Err(err).Msg("")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	fs := http.FileServer(http.FS(sub))

	r.URL.Path = chi.URLParam(r, "*")
	fs.ServeHTTP(w, r)
}

func handleCoverageEntryJSON(w http.ResponseWriter, r *http.Request) {
	scm, repo, cov, entry, ok := entryFrom(r.Context())
	if !ok {
		log.Error().Msg("can not find coverage entry")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	file := ""
	if entry.File != "" {
		file = entry.Name() + "/data/" + entry.File
	}
	log.Print("entry.Name=", entry.Name(), " file=", file)

	type htmlCoverageEntryResponse struct {
		File        string    `json:"file"`
		Revision    string    `json:"revision"`
		RevisionURL string    `json:"revision_url"`
		Time        time.Time `json:"time"`
	}

	json := htmlCoverageEntryResponse{
		File:        file,
		Revision:    cov.Revision(),
		RevisionURL: scm.RevisionURL(repo, cov.Revision()),
		Time:        cov.Time(),
	}

	render.JSON(w, json, http.StatusOK)
}

func (m *HTMLCoverageProvider) Handler() http.Handler {
	r := chi.NewRouter()
	r.Route("/{entry}", func(r chi.Router) {
		r.Use(CoverageEntryChecker)
		r.Get("/", templateRenderingHandler("coverage/html_coverage.html"))
		r.Get("/data/*", m.handleCoverageEntryData)
	})
	return r
}

func (m *HTMLCoverageProvider) HandleCoverage() http.Handler {
	r := chi.NewRouter()
	r.Route("/{entry}", func(r chi.Router) {
		r.Use(CoverageEntryChecker)
		r.Get("/", handleCoverageEntryJSON)
	})
	return r
}
