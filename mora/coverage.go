package mora

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/drone/drone/handler/api/render"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	mapset "github.com/deckarep/golang-set/v2"
)

type CoverageEntry interface {
	Name() string
	Lines() int
	Hits() int
}

type Coverage interface {
	Time() time.Time
	Revision() string
	Entries() []CoverageEntry
}

type CoverageProvider interface {
	CoveragesFor(repoURL string) []Coverage
	Handler() http.Handler
	WebHandler() http.Handler
	Repos() []string
	Sync() error
}

type CoverageEntryResponse struct {
	Name  string `json:"name"`
	Lines int    `json:"lines"`
	Hits  int    `json:"hits"`
}

type CoverageResponse struct {
	Index       int                     `json:"index"`
	Time        time.Time               `json:"time"`
	Revision    string                  `json:"revision"`
	RevisionURL string                  `json:"revision_url"`
	Entries     []CoverageEntryResponse `json:"entries"`
}

type ProvidedCoverage struct {
	coverage Coverage
	provider CoverageProvider
}

type CoverageService struct {
	providers []CoverageProvider
	repos     []string
	provided  map[string][]*ProvidedCoverage
}

func NewCoverageService() *CoverageService {
	return &CoverageService{}
}

func (m *CoverageService) AddProvider(provider CoverageProvider) {
	m.providers = append(m.providers, provider)
}

func (m *CoverageService) Sync() {
	for _, p := range m.providers {
		p.Sync()
	}

	repos := mapset.NewSet[string]()
	for _, provider := range m.providers {
		tmp := provider.Repos()
		for _, v := range tmp {
			repos.Add(v)
		}
	}

	m.repos = repos.ToSlice()

	provided := map[string][]*ProvidedCoverage{}
	for _, repo := range m.repos {
		e := []*ProvidedCoverage{}
		for _, p := range m.providers {
			tmp := p.CoveragesFor(repo)
			for _, cov := range tmp {
				e = append(e, &ProvidedCoverage{coverage: cov, provider: p})
			}
		}
		provided[repo] = e
	}

	m.provided = provided
}

func (m *CoverageService) Repos() []string {
	return m.repos
}

type coverageContextKey int

const (
	coverageKey      coverageContextKey = iota
	coverageEntryKey coverageContextKey = iota
)

func withCoverage(ctx context.Context, cov *ProvidedCoverage) context.Context {
	return context.WithValue(ctx, coverageKey, cov)
}

func providedCoverageFrom(ctx context.Context) (*ProvidedCoverage, bool) {
	cov, ok := ctx.Value(coverageKey).(*ProvidedCoverage)
	if !ok {
		log.Error().Msg("not ProvidedCoverage")
		return nil, false
	}
	return cov, ok
}

func CoverageFrom(ctx context.Context) (Coverage, bool) {
	cov, ok := providedCoverageFrom(ctx)
	return cov.coverage, ok
}

func providerFrom(ctx context.Context) (CoverageProvider, bool) {
	cov, ok := providedCoverageFrom(ctx)
	return cov.provider, ok
}

func WithCoverageEntry(ctx context.Context, entry CoverageEntry) context.Context {
	return context.WithValue(ctx, coverageEntryKey, entry)
}

func CoverageEntryFrom(ctx context.Context) (CoverageEntry, bool) {
	entry, ok := ctx.Value(coverageEntryKey).(CoverageEntry)
	return entry, ok
}

func InjectCoverageEntry(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Print("InjectCoverageEntry")
		entryName := chi.URLParam(r, "entry")

		cov, ok := CoverageFrom(r.Context())
		if !ok {
			log.Error().Msg("unknown coverage")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		var entry CoverageEntry = nil
		for _, e := range cov.Entries() {
			if e.Name() == entryName {
				entry = e
			}
		}

		if entry == nil {
			log.Error().Msg("can not find entry")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithCoverageEntry(r.Context(), entry)))
	})
}

func (m *CoverageService) injectCoverage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index, err := strconv.Atoi(chi.URLParam(r, "index"))
		if err != nil {
			log.Error().Err(err).Msg("")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		repo, ok := RepoFrom(r.Context())
		if !ok {
			render.NotFound(w, render.ErrNotFound)
			return
		}

		coverages := m.provided[repo.Link]
		if index < 0 || index >= len(coverages) {
			log.Error().Msgf("coverage index is out of range: index=%d", index)
			render.NotFound(w, render.ErrNotFound)
			return
		}
		cov := coverages[index]

		r = r.WithContext(withCoverage(r.Context(), cov))
		next.ServeHTTP(w, r)
	})
}

func convertCoverage(revisionURL string, cov Coverage, index int) CoverageResponse {
	ret := CoverageResponse{
		Index:       index,
		Time:        cov.Time(),
		Revision:    cov.Revision(),
		RevisionURL: revisionURL,
		Entries:     []CoverageEntryResponse{},
	}

	for _, e := range cov.Entries() {
		d := CoverageEntryResponse{
			Name:  e.Name(),
			Hits:  e.Hits(),
			Lines: e.Lines(),
		}
		ret.Entries = append(ret.Entries, d)
	}

	return ret
}

func convertCoverages(scm Client, repo *Repo, coverages []Coverage) []CoverageResponse {
	var ret []CoverageResponse
	for i, cov := range coverages {
		revURL := scm.RevisionURL(repo, cov.Revision())
		ret = append(ret, convertCoverage(revURL, cov, i))
	}

	return ret
}

func (s *CoverageService) handleCoverageList(w http.ResponseWriter, r *http.Request) {
	scm, ok := SCMFrom(r.Context())
	if !ok {
		log.Error().Msg("handleCoverageList: scm not found in a context")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	repo, ok := RepoFrom(r.Context())
	if !ok {
		log.Error().Msg("handleCoverageList: repo not found in a context")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	log.Print("handleCoverageList: making list...")
	coverages := []Coverage{}
	for _, provider := range s.providers {
		log.Print("handleCoverageList: call CoverageFor")
		tmp := provider.CoveragesFor(repo.Link)
		coverages = append(coverages, tmp...)
	}

	covs := convertCoverages(scm, repo, coverages)
	render.JSON(w, covs, http.StatusOK)
}

// API
func (s *CoverageService) handleCoverage(w http.ResponseWriter, r *http.Request) {
	provider, _ := providerFrom(r.Context())
	provider.Handler().ServeHTTP(w, r)
}

func (s *CoverageService) APIHandler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", s.handleCoverageList)

	r.Route("/{index}", func(r chi.Router) {
		r.Use(s.injectCoverage)
		r.Mount("/", http.HandlerFunc(s.handleCoverage))
	})
	return r
}

// Web
func (s *CoverageService) handleCoveragePage(w http.ResponseWriter, r *http.Request) {
	provider, _ := providerFrom(r.Context())
	provider.WebHandler().ServeHTTP(w, r)
}

func (s *CoverageService) WebHandler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", templateRenderingHandler("coverage/coverage.html"))

	r.Route("/{index}", func(r chi.Router) {
		r.Use(s.injectCoverage)
		r.Mount("/", http.HandlerFunc(s.handleCoveragePage))
	})
	return r
}
