package mora

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/drone/drone/handler/api/render"
	"github.com/drone/go-scm/scm"
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
	revision string
	time     time.Time
	entries  []*entryImpl
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
	covmap map[string][]Coverage
	repos  []string
	sync.Mutex
}

func NewToolCoverageProvider() *ToolCoverageProvider {
	p := &ToolCoverageProvider{}
	p.covmap = map[string][]Coverage{}
	p.repos = []string{}

	return p
}

func (p *ToolCoverageProvider) addCoverage(url string, cov Coverage) {
	p.Lock()
	defer p.Unlock()

	list, ok := p.covmap[url]
	if !ok {
		//log.Print("addCoverage: first coverage")
		list = []Coverage{}
	}

	//log.Print("addCoverage: len(list)=", len(list))
	p.covmap[url] = append(list, cov)
	//log.Print("addCoverage: len(p.covmap[url])=", len(p.covmap[url]))

	repos := []string{}
	for k := range p.covmap {
		repos = append(repos, k)
	}
	p.repos = repos
}

func (p *ToolCoverageProvider) CoveragesFor(repoURL string) []Coverage {
	ret, ok := p.covmap[repoURL]
	for k := range p.covmap {
		log.Print("CoveragesFor: repo=", k, " ", k == repoURL)
	}
	if !ok {
		log.Print("CoveragesFor: no coverage for ", repoURL)
		return []Coverage{}
	}
	log.Print("CoveragesFor: len=", len(ret))
	return ret
}

func (p *ToolCoverageProvider) Repos() []string {
	return p.repos
}

func (p *ToolCoverageProvider) Sync() error {
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

	cov, _, ok := entryImplFrom(r.Context())
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
	hits := 0
	lines := 0
	for _, entry := range cov.entries {
		for _, pr := range entry.profiles {
			files = append(files, &FileResponse{
				FileName: pr.FileName, Lines: pr.Lines, Hits: pr.Hits})
			hits += pr.Hits
			lines += pr.Lines
		}
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
			Hits:        hits,
			Lines:       lines,
		},
	}

	render.JSON(w, resp, http.StatusOK)
}

func WithToken(session *MoraSession, name string) (context.Context, error) {
	token, ok := session.getToken(name)
	if !ok {
		return nil, errorTokenNotFound
	}

	return scm.WithContext(context.Background(), &token), nil
}

func getSourceCode(ctx context.Context, revision, path string) ([]byte, error) {
	repo, _ := RepoFrom(ctx)
	repoPath := repo.Namespace + "/" + repo.Name

	scm, _ := SCMFrom(ctx)
	client := scm.Client()

	sess, _ := MoraSessionFrom(ctx)
	ctx, err := WithToken(sess, scm.Name())
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

	type Response struct {
		FileName string  `json:"filename"`
		Code     string  `json:"code"`
		Blocks   [][]int `json:"blocks"`
	}

	resp := Response{
		FileName: profile.FileName,
		Code:     string(code),
		Blocks:   profile.Blocks,
	}

	render.JSON(w, resp, http.StatusOK)
}

type CoverageUploadRequest struct {
	Format     string     `json:"format"`
	EntryName  string     `json:"entry"`
	RepoURL    string     `json:"repo"`
	Revision   string     `json:"revision"`
	ModuleName string     `json:"module"`
	Time       time.Time  `json:"time"`
	Profiles   []*Profile `json:"profiles"`
}

func parseToCoverage(req *CoverageUploadRequest) (*coverageImpl, error) {
	if req.EntryName == "" || req.RepoURL == "" || req.ModuleName == "" {
		return nil, errors.New("illegal request")
	}

	lines := 0
	hits := 0
	profiles := map[string]*Profile{}
	for _, p := range req.Profiles {
		profiles[p.FileName] = &Profile{
			FileName: p.FileName,
			Hits:     p.Hits,
			Lines:    p.Lines,
			Blocks:   p.Blocks,
		}
		lines += p.Lines
		hits += p.Hits
	}

	entry := &entryImpl{}
	entry.name = req.EntryName
	entry.profiles = profiles
	entry.lines = lines
	entry.hits = hits

	cov := &coverageImpl{}
	cov.revision = req.Revision
	cov.entries = []*entryImpl{entry}
	cov.time = req.Time

	return cov, nil
}

func (p *ToolCoverageProvider) HandleUpload(w http.ResponseWriter, r *http.Request) {
	log.Print("HandleUpload")
	var req CoverageUploadRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Err(err).Msg("HandleUpload")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	cov, err := parseToCoverage(&req)
	if err != nil {
		log.Err(err).Msg("HandleUpload")
		render.NotFound(w, err)
		return
	}

	p.addCoverage(req.RepoURL, cov)
}

// API
func (p *ToolCoverageProvider) Handler() http.Handler {
	r := chi.NewRouter()
	r.Route("/{entry}", func(r chi.Router) {
		r.Use(InjectCoverageEntry)
		r.Get("/files", handleFileList)
		r.Get("/files/*", handleFile)
	})
	return r
}

// Web
func (p *ToolCoverageProvider) WebHandler() http.Handler {
	r := chi.NewRouter()
	r.Get("/{entry}", templateRenderingHandler("coverage/tool_coverage.html"))
	return r
}
