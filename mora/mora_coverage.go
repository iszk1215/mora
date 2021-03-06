package mora

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type MoraCoverageProvider struct {
	coverages []Coverage
	store     *CoverageStore
	sync.Mutex
}

func NewMoraCoverageProvider(store *CoverageStore) *MoraCoverageProvider {
	p := &MoraCoverageProvider{}
	p.store = store

	p.coverages = []Coverage{}

	return p
}

func (p *MoraCoverageProvider) findCoverage(cov Coverage) int {
	for i, c := range p.coverages {
		if c.RepoURL() == cov.RepoURL() && c.Revision() == cov.Revision() {
			return i
		}
	}

	return -1
}

// Profile is not deep-copied because it is read-only
func mergeEntry(a, b *entryImpl) *entryImpl {
	c := &entryImpl{name: a.name, profiles: map[string]*Profile{}}

	for file, p := range a.profiles {
		c.profiles[file] = p
	}

	for file, p := range b.profiles {
		c.profiles[file] = p
	}

	c.hits = 0
	c.lines = 0
	for _, p := range c.profiles {
		c.hits += p.Hits
		c.lines += p.Lines
	}

	return c
}

func mergeCoverage(a, b *coverageImpl) (*coverageImpl, error) {
	log.Print("a=", a)
	if a.url != b.url || a.revision != b.revision {
		return nil, fmt.Errorf("can not merge two coverages those have different urls and/or revisions")
	}

	// c = merge(a, b)

	entries := map[string]*entryImpl{}

	for _, e := range a.entries {
		entries[e.name] = e
	}

	for _, e := range b.entries {
		ea, ok := entries[e.name]
		if ok {
			entries[e.name] = mergeEntry(ea, e)
		} else {
			entries[e.name] = e
		}
	}

	c := &coverageImpl{
		url:      a.url,
		revision: a.revision,
		time:     a.time,
		entries:  pie.Values(entries),
	}

	return c, nil
}

func (p *MoraCoverageProvider) addOrMergeCoverage(cov *coverageImpl) *coverageImpl {
	p.Lock()
	defer p.Unlock()

	idx := p.findCoverage(cov)
	log.Print("idx=", idx)
	if idx < 0 {
		p.coverages = append(p.coverages, cov)
		return nil
	} else {
		log.Print("p.coverages[idx]=", p.coverages[idx])
		merged, _ := mergeCoverage(p.coverages[idx].(*coverageImpl), cov)
		p.coverages[idx] = merged
		return merged
	}
}

func (p *MoraCoverageProvider) Coverages() []Coverage {
	return p.coverages
}

func (p *MoraCoverageProvider) Sync() error {
	return p.loadFromStore()
}

func parseScanedCoverage(record ScanedCoverage) (*coverageImpl, error) {
	var req []*CoverageEntryUploadRequest
	err := json.Unmarshal([]byte(record.Contents), &req)
	if err != nil {
		return nil, err
	}

	entries, err := parseEntries(req)
	if err != nil {
		return nil, err
	}

	cov := &coverageImpl{}
	cov.url = record.RepoURL
	cov.revision = record.Revision
	cov.entries = entries
	cov.time = record.Time

	return cov, nil
}

func (p *MoraCoverageProvider) loadFromStore() error {
	records, err := p.store.Scan()
	if err != nil {
		return err
	}

	for _, record := range records {
		cov, err := parseScanedCoverage(record)
		if err != nil {
			return err
		}

		p.coverages = append(p.coverages, cov)
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

func parseEntry(req *CoverageEntryUploadRequest) (*entryImpl, error) {
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

func parseEntries(req []*CoverageEntryUploadRequest) ([]*entryImpl, error) {
	entries := []*entryImpl{}
	for _, e := range req {
		entry, err := parseEntry(e)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func parseCoverage(req *CoverageUploadRequest) (*coverageImpl, error) {
	if req.RepoURL == "" {
		return nil, errors.New("repo url is empty")
	}

	entries, err := parseEntries(req.Entries)
	if err != nil {
		return nil, err
	}

	cov := &coverageImpl{}
	cov.url = req.RepoURL
	cov.revision = req.Revision
	cov.entries = entries
	cov.time = req.Time

	return cov, nil
}

func parseFromReader(reader io.Reader) (*CoverageUploadRequest, *coverageImpl, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, err
	}

	var req *CoverageUploadRequest
	err = json.Unmarshal(b, &req)
	if err != nil {
		return nil, nil, err
	}

	cov, err := parseCoverage(req)
	if err != nil {
		return nil, nil, err
	}

	return req, cov, nil
}

func (p *MoraCoverageProvider) HandleUpload(w http.ResponseWriter, r *http.Request) {
	log.Print("HandleUpload")

	req, cov, err := parseFromReader(r.Body)
	if err != nil {
		log.Err(err).Msg("HandleUpload")
		render.NotFound(w, render.ErrNotFound)
		return
	}

	merged := p.addOrMergeCoverage(cov)

	if p.store != nil {
		var entries []*CoverageEntryUploadRequest
		if merged == nil {
			entries = req.Entries
		} else {
			// rebuild entries
			for _, e := range merged.entries {
				entries = append(entries,
					&CoverageEntryUploadRequest{
						EntryName: e.name,
						Hits:      e.hits,
						Lines:     e.lines,
						Profiles:  pie.Values(e.profiles),
					})
			}
		}
		contents, err := json.Marshal(entries)
		if err != nil {
			log.Err(err).Msg("HandleUpload")
			render.NotFound(w, render.ErrNotFound)
			return
		}

		err = p.store.Put(cov, string(contents))
		if err != nil {
			log.Err(err).Msg("HandleUpload")
			render.NotFound(w, render.ErrNotFound)
			return
		}
	}
}

// Entry Handler

// API
func (p *MoraCoverageProvider) Handler() http.Handler {
	r := chi.NewRouter()
	r.Get("/files", handleFileList)
	r.Get("/files/*", handleFile)
	return r
}

// Web
func (p *MoraCoverageProvider) WebHandler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", templateRenderingHandler("coverage/mora_coverage.html"))
	return r
}
