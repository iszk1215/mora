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
)

type (
	// handleCoverageList
	CoverageResponse struct {
		Index       int64            `json:"index"`
		RevisionURL string           `json:"revision_url"`
		Revision    string           `json:"revision"`
		Timestamp   time.Time        `json:"time"`
		Entries     []*CoverageEntry `json:"entries"`
		// profiles are emptry
	}

	// hanldleFileList
	FileResponse struct {
		FileName string `json:"filename"`
		Hits     int    `json:"hits"`
		Lines    int    `json:"lines"`
	}

	MetaResonse struct {
		Revision    string    `json:"revision"`
		RevisionURL string    `json:"revision_url"`
		Time        time.Time `json:"time"`
		Hits        int       `json:"hits"`
		Lines       int       `json:"lines"`
	}

	FileListResponse struct {
		Metadata MetaResonse     `json:"meta"`
		Files    []*FileResponse `json:"files"`
	}

	// handleFile
	CodeResponse struct {
		FileName string  `json:"filename"`
		Code     string  `json:"code"`
		Blocks   [][]int `json:"blocks"`
	}

	// Upload
	CoverageEntryUploadRequest struct {
		Name     string             `json:"entry"`
		Hits     int                `json:"hits"`
		Lines    int                `json:"lines"`
		Profiles []*profile.Profile `json:"profiles"`
	}

	CoverageUploadRequest struct {
		RepoURL   string                        `json:"repo"`
		Revision  string                        `json:"revision"`
		Timestamp time.Time                     `json:"time"`
		Entries   []*CoverageEntryUploadRequest `json:"entries"`
	}

	CoverageProvider interface {
		AddCoverage(*Coverage) error
	}

	CoverageHandler struct {
		provider  CoverageProvider
		repos     RepositoryStore
		coverages CoverageStore
		sync.Mutex
	}

	coverageContextKey int
)

const (
	coverageKey      coverageContextKey = iota
	coverageEntryKey coverageContextKey = iota
)

func parseCoverageEntryUploadRequest(req *CoverageEntryUploadRequest) (*CoverageEntry, error) {
	if req.Name == "" {
		return nil, errors.New("entry name is empty")
	}

	files := map[string]*profile.Profile{}
	for _, p := range req.Profiles {
		files[p.FileName] = p
	}

	entry := &CoverageEntry{}
	entry.Name = req.Name
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

func (s *CoverageHandler) parseCoverageUploadRequest(req *CoverageUploadRequest) (*Coverage, error) {
	if req.RepoURL == "" {
		return nil, errors.New("repo url is empty")
	}

	log.Print(s.repos)
	repo, err := s.repos.FindByURL(req.RepoURL)
	if err != nil {
		return nil, errors.New("repo is not found")
	}

	entries, err := parseCoverageEntryUploadRequests(req.Entries)
	if err != nil {
		return nil, err
	}

	cov := &Coverage{}
	cov.RepoID = repo.ID
	cov.Revision = req.Revision
	cov.Entries = entries
	cov.Timestamp = req.Timestamp

	return cov, nil
}

func NewCoverageHandler(provider CoverageProvider, repos RepositoryStore, coverages CoverageStore) *CoverageHandler {
	s := &CoverageHandler{provider: provider, repos: repos, coverages: coverages}
	return s
}

func withCoverage(ctx context.Context, cov *Coverage) context.Context {
	return context.WithValue(ctx, coverageKey, cov)
}

func CoverageFrom(ctx context.Context) (*Coverage, bool) {
	cov, ok := ctx.Value(coverageKey).(*Coverage)
	return cov, ok
}

func WithCoverageEntry(ctx context.Context, entry *CoverageEntry) context.Context {
	return context.WithValue(ctx, coverageEntryKey, entry)
}

func CoverageEntryFrom(ctx context.Context) (*CoverageEntry, bool) {
	entry, ok := ctx.Value(coverageEntryKey).(*CoverageEntry)
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

		entry := cov.FindEntry(entryName)
		if entry == nil {
			log.Error().Msg("can not find entry")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithCoverageEntry(r.Context(), entry)))
	})
}

func (s *CoverageHandler) injectCoverage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.provider == nil {
			log.Print("No provider enabled")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		/*
			repo, ok := RepoFrom(r.Context())
			if !ok {
				render.NotFound(w, render.ErrNotFound)
				return
			}
		*/

		index, err := strconv.ParseInt(chi.URLParam(r, "index"), 10, 64)
		if err != nil {
			log.Error().Err(err).Msg("")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		log.Print("injectCoverage: index=", index)

		cov, err := s.coverages.Find(index)
		if err != nil {
			log.Error().Err(err).Msg("")
			render.NotFound(w, render.ErrNotFound)
			return
		}
		if cov == nil {
			log.Print("injectCoverage: cov is nil")
			render.NotFound(w, render.ErrNotFound)
			return
		}
		r = r.WithContext(withCoverage(r.Context(), cov))
		next.ServeHTTP(w, r)
	})
}

func makeCoverageResponse(revisionURL string, cov *Coverage) CoverageResponse {
	resp := CoverageResponse{
		Index:       cov.ID,
		Timestamp:   cov.Timestamp,
		Revision:    cov.Revision,
		RevisionURL: revisionURL,
		Entries:     []*CoverageEntry{},
	}

	for _, e := range cov.Entries {
		f := &CoverageEntry{
			Name:  e.Name,
			Hits:  e.Hits,
			Lines: e.Lines,
		}
		resp.Entries = append(resp.Entries, f)
	}

	return resp
}

func makeCoverageResponseList(scm SCM, repo Repository, coverages []*Coverage) []CoverageResponse {
	var ret []CoverageResponse
	for _, cov := range coverages {
		revURL := scm.RevisionURL(repo.Link, cov.Revision)
		ret = append(ret, makeCoverageResponse(revURL, cov))
	}

	return ret
}

func (s *CoverageHandler) handleCoverageList(w http.ResponseWriter, r *http.Request) {
	scm, _ := SCMFrom(r.Context())
	repo, _ := RepoFrom(r.Context())

	log.Print("repo.ID=", repo.ID)
	coverages, err := s.coverages.List(repo.ID)
	if err != nil {
		log.Err(err).Msg("")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	log.Print("len(coverages)=", len(coverages))

	if len(coverages) == 0 {
		log.Error().Msgf("Unknown repo.Link: %s", repo.Link)
		render.NotFound(w, render.ErrNotFound)
		return
	}

	sort.Slice(coverages, func(i, j int) bool {
		return coverages[i].Timestamp.Before(coverages[j].Timestamp)
	})

	resp := makeCoverageResponseList(scm, repo, coverages)
	render.JSON(w, resp, http.StatusOK)
}

func makeFileListResponse(scm SCM, repo Repository, cov *Coverage, entry *CoverageEntry) FileListResponse {
	files := []*FileResponse{}
	for _, pr := range entry.Profiles {
		files = append(files, &FileResponse{
			FileName: pr.FileName, Lines: pr.Lines, Hits: pr.Hits})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].FileName < files[j].FileName
	})

	return FileListResponse{
		Files: files,
		Metadata: MetaResonse{
			Revision:    cov.Revision,
			RevisionURL: scm.RevisionURL(repo.Link, cov.Revision),
			Time:        cov.Timestamp,
			Hits:        entry.Hits,
			Lines:       entry.Lines,
		},
	}
}

func handleFileList(w http.ResponseWriter, r *http.Request) {
	log.Print("handleFileList")
	scm, _ := SCMFrom(r.Context())
	repo, _ := RepoFrom(r.Context())
	cov, _ := CoverageFrom(r.Context())
	entry, _ := CoverageEntryFrom(r.Context())

	resp := makeFileListResponse(scm, repo, cov, entry)
	render.JSON(w, resp, http.StatusOK)
}

func getSourceCode(ctx context.Context, revision, path string) ([]byte, error) {
	scm, _ := SCMFrom(ctx)
	repo, _ := RepoFrom(ctx)

	sess, ok := MoraSessionFrom(ctx)
	if !ok {
		return nil, errors.New("MoraSession not found in a context")
	}

	ctx, err := sess.WithToken(context.Background(), scm.Name())
	if err != nil {
		return nil, err
	}

	client := scm.Client()
	repoPath := repo.Namespace + "/" + repo.Name
	content, meta, err := client.Contents.Find(ctx, repoPath, path, revision)
	if err != nil {
		log.Print(meta)
		return nil, err
	}

	return content.Data, nil
}

func handleFile(w http.ResponseWriter, r *http.Request) {
	log.Print("handleFile")
	cov, _ := CoverageFrom(r.Context())
	entry, _ := CoverageEntryFrom(r.Context())

	file := chi.URLParam(r, "*")
	log.Print("file=", file)

	profile, ok := entry.Profiles[file]
	if !ok {
		log.Error().Msgf("No file found in a CoverageEntry: %s", file)
		render.NotFound(w, render.ErrNotFound)
		return
	}

	code, err := getSourceCode(r.Context(), cov.Revision, file)
	if err != nil {
		log.Error().Err(err).Msg("handleFile")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	resp := CodeResponse{
		FileName: profile.FileName,
		Code:     string(code),
		Blocks:   profile.Blocks,
	}

	render.JSON(w, resp, http.StatusOK)
}

func (s *CoverageHandler) Handler() http.Handler {
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

func readUploadRequest(reader io.Reader) (*CoverageUploadRequest, error) {
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

func (s *CoverageHandler) processUploadRequest(req *CoverageUploadRequest) error {
	cov, err := s.parseCoverageUploadRequest(req)
	if err == nil {
		err = s.provider.AddCoverage(cov)
	}

	return err
}

func (s *CoverageHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	req, err := readUploadRequest(r.Body)

	if err == nil {
		err = s.processUploadRequest(req)
	}

	if err != nil {
		log.Err(err).Msg("HandleUpload")
		render.NotFound(w, render.ErrNotFound)
		return
	}
}
