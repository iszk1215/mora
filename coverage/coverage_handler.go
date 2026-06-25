package coverage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/iszk1215/mora/core"
	"github.com/iszk1215/mora/coverage/profile"
	"github.com/iszk1215/mora/render"
	"github.com/rs/zerolog/log"
)

type (
	// handleCoverageList
	CoverageResponse struct {
		ID          int64            `json:"index"`
		RevisionURL string           `json:"revision_url"`
		Revision    string           `json:"revision"`
		Timestamp   time.Time        `json:"time"`
		Entries     []*CoverageEntry `json:"entries"`
		// Profiles in entry is emptry
	}

	CoverageListResponse struct {
		Repo      core.Repository    `json:"repo"`
		Coverages []CoverageResponse `json:"coverages"`
	}

	// handleFileList
	FileResponse struct {
		FileName string `json:"filename"`
		Hits     int    `json:"hits"`
		Lines    int    `json:"lines"`
	}

	MetaResponse struct {
		Revision    string    `json:"revision"`
		RevisionURL string    `json:"revision_url"`
		Time        time.Time `json:"time"`
		Hits        int       `json:"hits"`
		Lines       int       `json:"lines"`
	}

	FileListResponse struct {
		Metadata MetaResponse     `json:"meta"`
		Repo     core.Repository `json:"repo"`
		Files    []*FileResponse `json:"files"`
	}

	// handleFile
	CodeResponse struct {
		Repo     core.Repository `json:"repo"`
		FileName string          `json:"filename"`
		Code     string          `json:"code"`
		Blocks   [][]int         `json:"blocks"`
	}

	// Upload
	CoverageEntryUploadRequest struct {
		Name     string             `json:"entry"`
		Hits     int                `json:"hits"`
		Lines    int                `json:"lines"`
		Profiles []*profile.Profile `json:"profiles"`
	}

	CoverageUploadRequest struct {
		Revision  string                        `json:"revision"`
		Timestamp time.Time                     `json:"time"`
		Entries   []*CoverageEntryUploadRequest `json:"entries"`
	}

	CoverageHandler struct {
		coverages CoverageStore
	}

	coverageContextKey int
)

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

func WithCoverageEntry(ctx context.Context, entry *CoverageEntry) context.Context {
	return context.WithValue(ctx, coverageEntryKey, entry)
}

func CoverageEntryFrom(ctx context.Context) (*CoverageEntry, bool) {
	entry, ok := ctx.Value(coverageEntryKey).(*CoverageEntry)
	return entry, ok
}

func injectCoverageEntry(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entryName := chi.URLParam(r, "entry")

		cov, ok := CoverageFrom(r.Context())
		if !ok {
			log.Error().Msg("unknown coverage")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		entry := cov.FindEntry(entryName)
		if entry == nil {
			log.Warn().Msg("can not find entry")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithCoverageEntry(r.Context(), entry)))
	})
}

func (s *CoverageHandler) injectCoverage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			log.Warn().Err(err).Msg("invalid coverage id in URL")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		cov, err := s.coverages.Find(id)
		if err != nil {
			log.Warn().Err(err).Msg("failed to find coverage")
			render.NotFound(w, render.ErrNotFound)
			return
		}
		if cov == nil {
			log.Warn().Msg("injectCoverage: cov is nil")
			render.NotFound(w, render.ErrNotFound)
			return
		}
		r = r.WithContext(withCoverage(r.Context(), cov))
		next.ServeHTTP(w, r)
	})
}

func makeCoverageResponse(revisionURL string, cov *Coverage) CoverageResponse {
	resp := CoverageResponse{
		ID:          cov.ID,
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

func makeCoverageListResponse(
	rm core.RepositoryClient, repo core.Repository, coverages []*Coverage) CoverageListResponse {

	var covs []CoverageResponse
	for _, cov := range coverages {
		revURL := rm.RevisionURL(repo.Url, cov.Revision)
		covs = append(covs, makeCoverageResponse(revURL, cov))
	}

	resp := CoverageListResponse{
		Repo:      repo,
		Coverages: covs,
	}

	return resp
}

func (s *CoverageHandler) handleCoverageList(w http.ResponseWriter, r *http.Request) {
	rm, _ := core.RepositoryClientFrom(r.Context())
	repo, _ := core.RepoFrom(r.Context())

	coverages, err := s.coverages.List(repo.Id)
	if err != nil {
		log.Warn().Err(err).Msg("failed to list coverages")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	if len(coverages) == 0 {
		log.Warn().Msgf("Unknown coverage not found for repo.Id=%d", repo.Id)
		render.NotFound(w, render.ErrNotFound)
		return
	}

	sort.Slice(coverages, func(i, j int) bool {
		return coverages[i].Timestamp.Before(coverages[j].Timestamp)
	})

	resp := makeCoverageListResponse(rm, repo, coverages)
	render.JSON(w, resp, http.StatusOK)
}

func makeFileListResponse(rm core.RepositoryClient, repo core.Repository, cov *Coverage, entry *CoverageEntry) FileListResponse {
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
		Repo:  repo,
		Metadata: MetaResponse{
			Revision:    cov.Revision,
			RevisionURL: rm.RevisionURL(repo.Url, cov.Revision),
			Time:        cov.Timestamp,
			Hits:        entry.Hits,
			Lines:       entry.Lines,
		},
	}
}

func handleFileList(w http.ResponseWriter, r *http.Request) {
	rm, _ := core.RepositoryClientFrom(r.Context())
	repo, _ := core.RepoFrom(r.Context())
	cov, _ := CoverageFrom(r.Context())
	entry, _ := CoverageEntryFrom(r.Context())

	resp := makeFileListResponse(rm, repo, cov, entry)
	render.JSON(w, resp, http.StatusOK)
}

func getSourceCode(ctx context.Context, revision, path string) ([]byte, error) {
	rm, _ := core.RepositoryClientFrom(ctx)
	repo, _ := core.RepoFrom(ctx)

	client := rm.Client()
	repoPath := repo.Namespace + "/" + repo.Name
	content, _, err := client.Contents.Find(ctx, repoPath, path, revision)
	if err != nil {
		return nil, fmt.Errorf("getSourceCode Contents.Find(%s/%s): %w", repoPath, path, err)
	}

	return content.Data, nil
}

func handleFile(w http.ResponseWriter, r *http.Request) {
	repo, _ := core.RepoFrom(r.Context())
	cov, _ := CoverageFrom(r.Context())
	entry, _ := CoverageEntryFrom(r.Context())

	file := chi.URLParam(r, "*")

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
		Repo:     repo,
		FileName: profile.FileName,
		Code:     string(code),
		Blocks:   profile.Blocks,
	}

	render.JSON(w, resp, http.StatusOK)
}

func (s *CoverageHandler) AddCoverage(cov *Coverage) (*Coverage, error) {
	found, err := s.coverages.FindRevision(cov.RepoID, cov.Revision)
	if err != nil {
		return nil, fmt.Errorf("AddCoverage FindRevision: %w", err)
	}

	if found != nil {
		cov.ID = found.ID
		cov, err = mergeCoverage(found, cov)
		if err != nil {
			return nil, fmt.Errorf("AddCoverage merge: %w", err)
		}
	}

	cov.ID, err = s.coverages.Put(cov)
	return cov, err
}

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
			return nil, fmt.Errorf("parseCoverageEntryUploadRequests: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (s *CoverageHandler) HandleCoverageUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
	defer func() {
		_ = r.Body.Close()
	}()

	var request CoverageUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		render.BadRequest(w, err)
		return
	}

	if err := validateCoverageUploadRequest(&request); err != nil {
		render.BadRequest(w, err)
		return
	}

	repo, _ := core.RepoFrom(r.Context())

	entries, err := parseCoverageEntryUploadRequests(request.Entries)
	if err != nil {
		render.BadRequest(w, err)
		return
	}

	cov := &Coverage{}
	cov.RepoID = repo.Id
	cov.Revision = request.Revision
	cov.Entries = entries
	cov.Timestamp = request.Timestamp

	cov, err = s.AddCoverage(cov)
	if err != nil {
		log.Error().Err(err).Msg("HandleCoverageUpload")
		render.InternalError(w, err)
		return
	}

	render.JSON(w, cov, http.StatusCreated)
}

func (s *CoverageHandler) Handler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", s.handleCoverageList)
	r.Post("/", s.HandleCoverageUpload)

	r.Route("/{id}", func(r chi.Router) {
		r.Use(s.injectCoverage)
		r.Route("/{entry}", func(r chi.Router) {
			r.Use(injectCoverageEntry)
			r.Get("/files", handleFileList)
			r.Get("/files/*", handleFile)
		})
	})

	return r
}

var validRevision = regexp.MustCompile(`^[a-fA-F0-9]{6,40}$`)

func validateCoverageUploadRequest(req *CoverageUploadRequest) error {
	if req.Revision == "" {
		return errors.New("revision is required")
	}
	if len(req.Revision) > 255 {
		return fmt.Errorf("revision too long: %d characters (max 255)", len(req.Revision))
	}
	for _, r := range req.Revision {
		if r > unicode.MaxASCII || unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("revision contains invalid character %q", r)
		}
	}
	if !validRevision.MatchString(req.Revision) {
		return fmt.Errorf("invalid revision format: %q (expected hex SHA)", req.Revision)
	}
	if req.Timestamp.IsZero() {
		return errors.New("timestamp must not be zero")
	}
	return nil
}

func newCoverageHandler(store CoverageStore) *CoverageHandler {
	return &CoverageHandler{coverages: store}
}
