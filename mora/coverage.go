package mora

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/drone/drone/handler/api/render"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type CoverageEntry interface {
	Name() string
	Lines() int
	Hits() int
}

type Coverage interface {
	Time() time.Time
	Revision() string
	// Entries() map[string]CoverageEntry // TODO: use slice
	Entries() []CoverageEntry // TODO: use slice
}

type CoverageProvider interface {
	CoveragesFor(repoURL string) ([]Coverage, error)
	Handler() http.Handler
	HandleCoverage() http.Handler
	Repos() ([]string, error)
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

type coverageContextKey int

const (
	coverageKey coverageContextKey = iota
)

func withCoverage(ctx context.Context, cov Coverage) context.Context {
	ctx = context.WithValue(ctx, coverageKey, cov)
	return ctx
}

func coverageFrom(ctx context.Context) (Coverage, bool) {
	cov, ok := ctx.Value(coverageKey).(Coverage)
	return cov, ok
}

func injectCoverage(provider CoverageProvider) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
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

			coverages, err := provider.CoveragesFor(repo.Link)
			if err != nil || index < 0 || index >= len(coverages) {
				log.Error().Msgf("coverage index is out of range: index=%d", index)
				render.NotFound(w, render.ErrNotFound)
				return
			}
			cov := coverages[index]

			r = r.WithContext(withCoverage(r.Context(), cov))
			next.ServeHTTP(w, r)
		})
	}
}

func serializeCoverage(revisionURL string, cov Coverage, index int) CoverageResponse {
	ret := CoverageResponse{
		Index:       index,
		Time:        cov.Time(),
		Revision:    cov.Revision(),
		RevisionURL: revisionURL,
	}

	for _, e := range cov.Entries() {
		d := CoverageEntryResponse{
			Name:  e.Name(),
			Hits:  e.Hits(),
			Lines: e.Lines(),
		}
		ret.Entries = append(ret.Entries, d)
	}

	sort.Slice(ret.Entries, func(i, j int) bool {
		return ret.Entries[i].Name < ret.Entries[j].Name
	})

	return ret
}

func serializableCoverageList(scm Client, repo *Repo, coverages []Coverage) []CoverageResponse {
	var data []CoverageResponse
	for i, cov := range coverages {
		revURL := scm.RevisionURL(repo, cov.Revision())
		data = append(data, serializeCoverage(revURL, cov, i))
	}

	return data
}

func handleCoverageList(provider CoverageProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		coverages, err := provider.CoveragesFor(repo.Link)
		if err != nil {
			log.Err(err).Msg("")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		covs := serializableCoverageList(scm, repo, coverages)
		render.JSON(w, covs, http.StatusOK)
	}
}

func HandleCoverage(provider CoverageProvider) http.Handler {
	r := chi.NewRouter()
	r.Get("/", handleCoverageList(provider))
	r.Route("/{index}", func(r chi.Router) {
		r.Use(injectCoverage(provider))
		r.Mount("/", provider.HandleCoverage())
	})
	return r
}

func CoverageWebHandler(provider CoverageProvider) http.Handler {
	r := chi.NewRouter()
	r.Get("/", templateRenderingHandler("coverage/coverage.html"))
	r.Route("/{index}", func(r chi.Router) {
		r.Use(injectCoverage(provider))
		r.Mount("/", provider.Handler())
	})
	return r
}
