package mora

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/drone/drone/handler/api/render"
	"github.com/elliotchance/pie/v2"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type entryImpl struct {
	name     string
	hits     int
	lines    int
	profiles map[string]*Profile
}

func (e *entryImpl) Name() string {
	return e.name
}

func (e *entryImpl) Lines() int {
	return e.lines
}

func (e *entryImpl) Hits() int {
	return e.hits
}

type coverageImpl struct {
	url      string
	revision string
	time     time.Time
	entries  []*entryImpl
}

func (c *coverageImpl) RepoURL() string {
	return c.url
}

func (c *coverageImpl) Time() time.Time {
	return c.time
}

func (c *coverageImpl) Revision() string {
	return c.revision
}

func (c *coverageImpl) Entries() []CoverageEntry {
	ret := []CoverageEntry{}
	for _, e := range c.entries {
		ret = append(ret, e)
	}
	return ret
}

type ToolCoverageProvider struct {
	coverages []Coverage
	covmap    map[string][]Coverage
	repos     []string
	store     *JSONStore
	sync.Mutex
}

func NewToolCoverageProvider(store *JSONStore) *ToolCoverageProvider {
	p := &ToolCoverageProvider{}
	p.covmap = map[string][]Coverage{}
	p.repos = []string{}
	p.store = store

	p.coverages = []Coverage{}

	return p
}

func (p *ToolCoverageProvider) addCoverage(url string, cov Coverage) {
	log.Print("ToolCoverageProvider.addCoverage: cov=", cov)
	p.Lock()
	defer p.Unlock()

	p.covmap[url] = append(p.covmap[url], cov)
	p.repos = pie.Keys(p.covmap)

	p.coverages = append(p.coverages, cov)
}

func (p *ToolCoverageProvider) Coverages() []Coverage {
	return p.coverages
}

func (p *ToolCoverageProvider) CoveragesFor(repoURL string) []Coverage {
	return p.covmap[repoURL]
}

func (p *ToolCoverageProvider) Repos() []string {
	return p.repos
}

func (p *ToolCoverageProvider) Sync() error {
	return p.loadFromStore()
}

func (p *ToolCoverageProvider) loadFromStore() error {
	rows, err := p.store.Scan()
	if err != nil {
		return err
	}
	for _, text := range rows {
		err = p.processRequestBody([]byte(text))
		if err != nil {
			return err
		}
	}

	return nil
}

func entryImplFrom(ctx context.Context) (*coverageImpl, *entryImpl, bool) {
	tmp0, _ := CoverageFrom(ctx)
	cov, ok0 := tmp0.(*coverageImpl)

	tmp1, _ := CoverageEntryFrom(ctx)
	entry, ok1 := tmp1.(*entryImpl)

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
	for _, pr := range entry.profiles {
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
			RevisionURL: scm.RevisionURL(repo, cov.Revision()),
			Time:        cov.Time(),
			Hits:        entry.hits,
			Lines:       entry.lines,
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

	profile, ok := entry.profiles[file]
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

type CoverageEntryUploadRequest struct {
	EntryName string     `json:"entry"`
	Profiles  []*Profile `json:"profiles"`
	Hits      int        `json:"hits"`
	Lines     int        `json:"lines"`
}

type CoverageUploadRequest struct {
	RepoURL  string                        `json:"repo"`
	Revision string                        `json:"revision"`
	Time     time.Time                     `json:"time"`
	Entries  []*CoverageEntryUploadRequest `json:"entries"`
}

func convertToEntry(req *CoverageEntryUploadRequest) (*entryImpl, error) {
	if req.EntryName == "" {
		return nil, errors.New("entry name is empty")
	}

	profiles := map[string]*Profile{}
	for _, p := range req.Profiles {
		profiles[p.FileName] = p
	}

	entry := &entryImpl{}
	entry.name = req.EntryName
	entry.profiles = profiles
	entry.hits = req.Hits
	entry.lines = req.Lines

	return entry, nil
}

func convertToCoverage(req *CoverageUploadRequest) (*coverageImpl, error) {
	if req.RepoURL == "" {
		return nil, errors.New("repo url is empty")
	}

	entries := []*entryImpl{}
	for _, e := range req.Entries {
		entry, err := convertToEntry(e)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	cov := &coverageImpl{}
	cov.url = req.RepoURL
	cov.revision = req.Revision
	cov.entries = entries
	cov.time = req.Time

	return cov, nil
}

func (p *ToolCoverageProvider) processRequestBody(bytes []byte) error {
	var req CoverageUploadRequest
	err := json.Unmarshal(bytes, &req)
	if err != nil {
		return err
	}

	cov, err := convertToCoverage(&req)
	if err != nil {
		return err
	}

	p.addCoverage(req.RepoURL, cov)
	return nil
}

func (p *ToolCoverageProvider) HandleUpload(w http.ResponseWriter, r *http.Request) {
	log.Print("HandleUpload")

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Err(err).Msg("HandleUpload")
		render.NotFound(w, render.ErrNotFound)
		return
	}
	if p.store != nil {
		err := p.store.Store(string(b))
		if err != nil {
			log.Err(err).Msg("HandleUpload")
			render.NotFound(w, render.ErrNotFound)
			return
		}
	}

	err = p.processRequestBody(b)
	if err != nil {
		log.Err(err).Msg("HandleUpload")
		render.NotFound(w, render.ErrNotFound)
		return
	}
}

// API
func (p *ToolCoverageProvider) Handler() http.Handler {
	r := chi.NewRouter()
	r.Get("/files", handleFileList)
	r.Get("/files/*", handleFile)
	return r
}

// Web
func (p *ToolCoverageProvider) WebHandler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", templateRenderingHandler("coverage/tool_coverage.html"))
	return r
}
