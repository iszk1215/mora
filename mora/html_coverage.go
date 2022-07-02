package mora

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/drone/drone/handler/api/render"
	"github.com/elliotchance/pie/v2"
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

func (c htmlCoverage) Entries() []CoverageEntry {
	return pie.Map(c.Entries_,
		func(e *htmlCoverageEntry) CoverageEntry { return e })
}

// Loader

func loadFile(data []byte, path string) (*htmlCoverage, error) {
	log.Print("loadFile: path=", path)
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

// HTML Coverage Handlers

func htmlCoverageFrom(ctx context.Context) (*htmlCoverage, bool) {
	tmp, ok := CoverageFrom(ctx)
	if !ok {
		return nil, false
	}
	cov, ok := tmp.(*htmlCoverage)
	return cov, ok
}

func htmlCoverageEntryFrom(ctx context.Context) (*htmlCoverage, *htmlCoverageEntry, bool) {
	cov, ok := htmlCoverageFrom(ctx)
	if !ok {
		return nil, nil, false
	}

	tmp, _ := CoverageEntryFrom(ctx)
	entry, ok := tmp.(*htmlCoverageEntry)

	return cov, entry, ok
}

type HTMLCoverageProvider struct {
	dataDirectory  fs.FS
	configFilename string
	covmap         map[string][]Coverage
	repos          []string
	sync.Mutex
}

func NewHTMLCoverageProvider(dataDirectory fs.FS) *HTMLCoverageProvider {
	m := new(HTMLCoverageProvider)
	m.dataDirectory = dataDirectory
	m.configFilename = "mora.yaml"
	return m
}

func (m *HTMLCoverageProvider) Sync() error {
	list, err := loadDirectory(m.dataDirectory, "", m.configFilename)
	if err != nil {
		return err
	}

	coverageMap := map[string][]Coverage{}
	for _, cov := range list {
		coverageMap[cov.RepoURL] = append(coverageMap[cov.RepoURL], cov)
	}

	repos := pie.Keys(coverageMap)

	m.Lock()
	defer m.Unlock()
	m.covmap = coverageMap
	m.repos = repos

	return nil
}

func (m *HTMLCoverageProvider) Repos() []string {
	return m.repos
}

func (m *HTMLCoverageProvider) CoveragesFor(repoURL string) []Coverage {
	return m.covmap[repoURL]
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
	scm, _ := SCMFrom(r.Context())
	repo, _ := RepoFrom(r.Context())

	cov, entry, ok := htmlCoverageEntryFrom(r.Context())
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
	r.Get("/", handleCoverageEntryJSON)
	return r
}

func (m *HTMLCoverageProvider) WebHandler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", templateRenderingHandler("coverage/html_coverage.html"))
	r.Get("/data/*", m.handleCoverageEntryData)
	return r
}
