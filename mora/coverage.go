package mora

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/drone/drone/handler/api/render"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	mapset "github.com/deckarep/golang-set/v2"
)

type CoverageEntry struct {
	Name  string `json:"name"`
	Lines int    `json:"lines"`
	Hits  int    `json:"hits"`
}

type Coverage interface {
	RepoURL() string
	Revision() string
	Time() time.Time
	Entries() []CoverageEntry
}

type CoverageProvider interface {
	Coverages() []Coverage
	Handler() http.Handler
	WebHandler() http.Handler
	Sync() error
}

type CoverageResponse struct {
	Index       int             `json:"index"`
	Time        time.Time       `json:"time"`
	Revision    string          `json:"revision"`
	RevisionURL string          `json:"revision_url"`
	Entries     []CoverageEntry `json:"entries"`
}

type providedCoverage struct {
	coverage Coverage
	provider CoverageProvider
}

type CoverageService struct {
	providers []CoverageProvider
	repos     []string
	provided  map[string][]*providedCoverage
	sync.Mutex
}

func NewCoverageService() *CoverageService {
	return &CoverageService{}
}

func (m *CoverageService) AddProvider(provider CoverageProvider) {
	m.providers = append(m.providers, provider)
}

func (m *CoverageService) SyncProviders() {
	for _, p := range m.providers {
		p.Sync()
	}
}

func (s *CoverageService) Sync() {
	provided := map[string][]*providedCoverage{}
	repos := mapset.NewSet[string]()
	for _, p := range s.providers {
		for _, cov := range p.Coverages() {
			url := cov.RepoURL()
			repos.Add(url)
			pc := &providedCoverage{coverage: cov, provider: p}
			provided[url] = append(provided[url], pc)
		}
	}

	for _, list := range provided {
		sort.Slice(list, func(i, j int) bool {
			return list[i].coverage.Time().Before(list[j].coverage.Time())
		})
	}

	s.Lock()
	defer s.Unlock()
	s.repos = repos.ToSlice()
	s.provided = provided
}

func (s *CoverageService) Repos() []string {
	return s.repos
}

type coverageContextKey int

const (
	coverageKey      coverageContextKey = iota
	coverageEntryKey coverageContextKey = iota
)

func withCoverage(ctx context.Context, cov *providedCoverage) context.Context {
	return context.WithValue(ctx, coverageKey, cov)
}

func providedCoverageFrom(ctx context.Context) (*providedCoverage, bool) {
	cov, ok := ctx.Value(coverageKey).(*providedCoverage)
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

func WithCoverageEntry(ctx context.Context, entry string) context.Context {
	return context.WithValue(ctx, coverageEntryKey, entry)
}

func CoverageEntryFrom(ctx context.Context) (string, bool) {
	entry, ok := ctx.Value(coverageEntryKey).(string)
	return entry, ok
}

func injectCoverageEntry(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Print("injectCoverageEntry")
		entryName := chi.URLParam(r, "entry")

		cov, ok := CoverageFrom(r.Context())
		if !ok {
			log.Error().Msg("unknown coverage")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		found := false
		for _, e := range cov.Entries() {
			if e.Name == entryName {
				found = true
			}
		}

		if !found {
			log.Error().Msg("can not find entry")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithCoverageEntry(r.Context(), entryName)))
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

		r = r.WithContext(withCoverage(r.Context(), coverages[index]))
		next.ServeHTTP(w, r)
	})
}

func makeCoverageResponse(revisionURL string, cov Coverage, index int) CoverageResponse {
	ret := CoverageResponse{
		Index:       index,
		Time:        cov.Time(),
		Revision:    cov.Revision(),
		RevisionURL: revisionURL,
		Entries:     cov.Entries(),
	}

	return ret
}

func makeCoverageResponseList(scm SCM, repo *Repo, coverages []Coverage) []CoverageResponse {
	var ret []CoverageResponse
	for i, cov := range coverages {
		revURL := scm.RevisionURL(repo, cov.Revision())
		ret = append(ret, makeCoverageResponse(revURL, cov, i))
	}

	return ret
}

func (s *CoverageService) handleCoverageList(w http.ResponseWriter, r *http.Request) {
	scm, _ := SCMFrom(r.Context())
	repo, _ := RepoFrom(r.Context())

	_, ok := s.provided[repo.Link]
	if !ok {
		log.Error().Msg("handleCoverageList: no coverage for repo")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	coverages := []Coverage{}
	for _, pcov := range s.provided[repo.Link] {
		coverages = append(coverages, pcov.coverage)
	}

	resp := makeCoverageResponseList(scm, repo, coverages)
	render.JSON(w, resp, http.StatusOK)
}

// API
func (s *CoverageService) handleCoverageEntry(w http.ResponseWriter, r *http.Request) {
	provider, _ := providerFrom(r.Context())
	provider.Handler().ServeHTTP(w, r)
}

func (s *CoverageService) APIHandler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", s.handleCoverageList)

	r.Route("/{index}", func(r chi.Router) {
		r.Use(s.injectCoverage)
		r.Route("/{entry}", func(r chi.Router) {
			r.Use(injectCoverageEntry)
			r.Mount("/", http.HandlerFunc(s.handleCoverageEntry))
		})
	})
	return r
}

// Web
func (s *CoverageService) handleCoverageEntryPage(w http.ResponseWriter, r *http.Request) {
	provider, _ := providerFrom(r.Context())
	provider.WebHandler().ServeHTTP(w, r)
}

func (s *CoverageService) WebHandler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", templateRenderingHandler("coverage/coverage.html"))

	r.Route("/{index}", func(r chi.Router) {
		r.Use(s.injectCoverage)
		r.Route("/{entry}", func(r chi.Router) {
			r.Use(injectCoverageEntry)
			r.Mount("/", http.HandlerFunc(s.handleCoverageEntryPage))
		})
	})
	return r
}
