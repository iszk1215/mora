package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/drone/drone/handler/api/render"
	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/mora/profile"
	"github.com/rs/zerolog/log"

	mapset "github.com/deckarep/golang-set/v2"
)

type CoverageProvider interface {
	Coverages() []*Coverage
	AddCoverage(*Coverage) error
}

type CoverageResponse struct {
	Index       int              `json:"index"`
	Time        time.Time        `json:"time"`
	Revision    string           `json:"revision"`
	RevisionURL string           `json:"revision_url"`
	Entries     []*CoverageEntry `json:"entries"`
}

type CoverageEntryUploadRequest struct {
	EntryName string             `json:"entry"`
	Profiles  []*profile.Profile `json:"profiles"`
	Hits      int                `json:"hits"`
	Lines     int                `json:"lines"`
}

type CoverageUploadRequest struct {
	RepoURL  string                        `json:"repo"`
	Revision string                        `json:"revision"`
	Time     time.Time                     `json:"time"`
	Entries  []*CoverageEntryUploadRequest `json:"entries"`
}

func parseCoverageEntryUploadRequest(req *CoverageEntryUploadRequest) (*CoverageEntry, error) {
	if req.EntryName == "" {
		return nil, errors.New("entry name is empty")
	}

	files := map[string]*profile.Profile{}
	for _, p := range req.Profiles {
		files[p.FileName] = p
	}

	entry := &CoverageEntry{}
	entry.Name = req.EntryName
	entry.Profiles = files
	entry.Hits = req.Hits
	entry.Lines = req.Lines

	return entry, nil
}

func parseCoverageEntryUploadRequests(req []*CoverageEntryUploadRequest) ([]*CoverageEntry, error) {
	entries := []*CoverageEntry{}
	for _, e := range req {
		entry, err := parseCoverageEntryUploadRequest(e)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func parseCoverageUploadRequest(req *CoverageUploadRequest) (*Coverage, error) {
	if req.RepoURL == "" {
		return nil, errors.New("repo url is empty")
	}

	entries, err := parseCoverageEntryUploadRequests(req.Entries)
	if err != nil {
		return nil, err
	}

	cov := &Coverage{}
	cov.url = req.RepoURL
	cov.revision = req.Revision
	cov.entries = entries
	cov.time = req.Time

	return cov, nil
}

type CoverageService struct {
	provider  CoverageProvider
	repos     []string
	coverages map[string][]*Coverage
	sync.Mutex
}

func NewCoverageService(provider CoverageProvider) *CoverageService {
	s := &CoverageService{provider: provider}
	s.Sync()
	return s
}

func (s *CoverageService) Sync() {
	coverages := map[string][]*Coverage{}
	repos := mapset.NewSet[string]()
	for _, cov := range s.provider.Coverages() {
		url := cov.RepoURL()
		repos.Add(url)
		coverages[url] = append(coverages[url], cov)
	}

	for _, list := range coverages {
		sort.Slice(list, func(i, j int) bool {
			return list[i].Time().Before(list[j].Time())
		})
	}

	s.Lock()
	defer s.Unlock()
	s.repos = repos.ToSlice()
	s.coverages = coverages
}

func (s *CoverageService) Repos() []string {
	return s.repos
}

type coverageContextKey int

const (
	coverageKey      coverageContextKey = iota
	coverageEntryKey coverageContextKey = iota
)

func withCoverage(ctx context.Context, cov *Coverage) context.Context {
	return context.WithValue(ctx, coverageKey, cov)
}

func CoverageFrom(ctx context.Context) (*Coverage, bool) {
	cov, ok := ctx.Value(coverageKey).(*Coverage)
	return cov, ok
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

		coverages := m.coverages[repo.Link]
		if index < 0 || index >= len(coverages) {
			log.Error().Msgf("coverage index is out of range: index=%d", index)
			render.NotFound(w, render.ErrNotFound)
			return
		}

		r = r.WithContext(withCoverage(r.Context(), coverages[index]))
		next.ServeHTTP(w, r)
	})
}

func makeCoverageResponse(revisionURL string, cov *Coverage, index int) CoverageResponse {
	ret := CoverageResponse{
		Index:       index,
		Time:        cov.Time(),
		Revision:    cov.Revision(),
		RevisionURL: revisionURL,
		Entries:     cov.Entries(),
	}

	return ret
}

func makeCoverageResponseList(scm SCM, repo *Repo, coverages []*Coverage) []CoverageResponse {
	var ret []CoverageResponse
	for i, cov := range coverages {
		revURL := scm.RevisionURL(repo.Link, cov.Revision())
		ret = append(ret, makeCoverageResponse(revURL, cov, i))
	}

	return ret
}

func (s *CoverageService) handleCoverageList(w http.ResponseWriter, r *http.Request) {
	scm, _ := SCMFrom(r.Context())
	repo, _ := RepoFrom(r.Context())

	_, ok := s.coverages[repo.Link]
	if !ok {
		log.Error().Msg("handleCoverageList: no coverage for repo")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	resp := makeCoverageResponseList(scm, repo, s.coverages[repo.Link])
	render.JSON(w, resp, http.StatusOK)
}

// API

func entryImplFrom(ctx context.Context) (*Coverage, *CoverageEntry, bool) {
	cov, ok0 := CoverageFrom(ctx)
	name, _ := CoverageEntryFrom(ctx)

	var entry *CoverageEntry
	ok1 := false
	for _, e := range cov.entries {
		if e.Name == name {
			entry = e
			ok1 = true
		}
	}

	return cov, entry, ok0 && ok1
}

func handleFileList(w http.ResponseWriter, r *http.Request) {
	log.Print("handleFileList")
	scm, _ := SCMFrom(r.Context())
	repo, _ := RepoFrom(r.Context())

	cov, entry, ok := entryImplFrom(r.Context())
	if !ok {
		log.Error().Msg("entry not found")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	type FileResponse struct {
		FileName string `json:"filename"`
		Hits     int    `json:"hits"`
		Lines    int    `json:"lines"`
	}

	type MetaResonse struct {
		Revision    string    `json:"revision"`
		RevisionURL string    `json:"revision_url"`
		Time        time.Time `json:"time"`
		Hits        int       `json:"hits"`
		Lines       int       `json:"lines"`
	}

	type Response struct {
		Files []*FileResponse `json:"files"`
		Meta  MetaResonse     `json:"meta"`
	}

	files := []*FileResponse{}
	for _, pr := range entry.Profiles {
		files = append(files, &FileResponse{
			FileName: pr.FileName, Lines: pr.Lines, Hits: pr.Hits})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].FileName < files[j].FileName
	})

	resp := Response{
		Files: files,
		Meta: MetaResonse{
			Revision:    cov.Revision(),
			RevisionURL: scm.RevisionURL(repo.Link, cov.Revision()),
			Time:        cov.Time(),
			Hits:        entry.Hits,
			Lines:       entry.Lines,
		},
	}

	render.JSON(w, resp, http.StatusOK)
}

func getSourceCode(ctx context.Context, revision, path string) ([]byte, error) {
	repo, _ := RepoFrom(ctx)
	repoPath := repo.Namespace + "/" + repo.Name

	scm, _ := SCMFrom(ctx)
	client := scm.Client()

	sess, _ := MoraSessionFrom(ctx)
	ctx, err := sess.WithToken(context.Background(), scm.Name())
	if err != nil {
		return nil, err
	}

	content, meta, err := client.Contents.Find(ctx, repoPath, path, revision)
	if err != nil {
		log.Print(meta)
		return nil, err
	}

	return content.Data, nil
}

func handleFile(w http.ResponseWriter, r *http.Request) {
	log.Print("handleFile")

	cov, entry, ok := entryImplFrom(r.Context())
	if !ok {
		log.Error().Msg("entryImplFrom returns false")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	file := chi.URLParam(r, "*")
	log.Print("file=", file)

	profile, ok := entry.Profiles[file]
	if !ok {
		log.Error().Msg("handleEntry")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	code, err := getSourceCode(r.Context(), cov.Revision(), file)
	if err != nil {
		log.Err(err).Msg("handleFile")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	type ProfileResponse struct {
		FileName string  `json:"filename"`
		Code     string  `json:"code"`
		Blocks   [][]int `json:"blocks"`
	}

	resp := ProfileResponse{
		FileName: profile.FileName,
		Code:     string(code),
		Blocks:   profile.Blocks,
	}

	render.JSON(w, resp, http.StatusOK)
}

func (s *CoverageService) Handler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", s.handleCoverageList)

	r.Route("/{index}", func(r chi.Router) {
		r.Use(s.injectCoverage)
		r.Route("/{entry}", func(r chi.Router) {
			r.Use(injectCoverageEntry)
			r.Get("/files", handleFileList)
			r.Get("/files/*", handleFile)
		})
	})

	return r
}

func parseFromReader(reader io.Reader) (*CoverageUploadRequest, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var req *CoverageUploadRequest
	err = json.Unmarshal(b, &req)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (s *CoverageService) processUploadRequest(req *CoverageUploadRequest) error {
	cov, err := parseCoverageUploadRequest(req)
	if err == nil {
		err = s.provider.AddCoverage(cov)
	}

	return err
}

func (s *CoverageService) HandleUpload(w http.ResponseWriter, r *http.Request) {
	req, err := parseFromReader(r.Body)

	if err == nil {
		err = s.processUploadRequest(req)
	}

	if err != nil {
		s.Sync()
		log.Err(err).Msg("HandleUpload")
		render.NotFound(w, render.ErrNotFound)
		return
	}
}
