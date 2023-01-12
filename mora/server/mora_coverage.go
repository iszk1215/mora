package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/drone/drone/handler/api/render"
	"github.com/elliotchance/pie/v2"
	"github.com/iszk1215/mora/mora/profile"
	"github.com/rs/zerolog/log"
)

type entryImpl struct {
	Name     string
	Hits     int
	Lines    int
	profiles map[string]*profile.Profile
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
		ret = append(ret,
			CoverageEntry{Name: e.Name, Hits: e.Hits, Lines: e.Lines})
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
	c := &entryImpl{Name: a.Name, profiles: map[string]*profile.Profile{}}

	for file, p := range a.profiles {
		c.profiles[file] = p
	}

	for file, p := range b.profiles {
		c.profiles[file] = p
	}

	c.Hits = 0
	c.Lines = 0
	for _, p := range c.profiles {
		c.Hits += p.Hits
		c.Lines += p.Lines
	}

	return c
}

func mergeCoverage(a, b *coverageImpl) (*coverageImpl, error) {
	if a.url != b.url || a.revision != b.revision {
		return nil, fmt.Errorf("can not merge two coverages with different urls and/or revisions")
	}

	entries := map[string]*entryImpl{}

	for _, e := range a.entries {
		entries[e.Name] = e
	}

	for _, e := range b.entries {
		ea, ok := entries[e.Name]
		if ok {
			entries[e.Name] = mergeEntry(ea, e)
		} else {
			entries[e.Name] = e
		}
	}

	merged := &coverageImpl{
		url:      a.url,
		revision: a.revision,
		time:     a.time,
		entries:  pie.Values(entries),
	}

	return merged, nil
}

func (p *MoraCoverageProvider) addOrMergeCoverage(cov *coverageImpl) *coverageImpl {
	p.Lock()
	defer p.Unlock()

	idx := p.findCoverage(cov)
	if idx < 0 {
		p.coverages = append(p.coverages, cov)
		return nil
	} else {
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

func parseEntry(req *CoverageEntryUploadRequest) (*entryImpl, error) {
	if req.EntryName == "" {
		return nil, errors.New("entry name is empty")
	}

	profiles := map[string]*profile.Profile{}
	for _, p := range req.Profiles {
		profiles[p.FileName] = p
	}

	entry := &entryImpl{}
	entry.Name = req.EntryName
	entry.profiles = profiles
	entry.Hits = req.Hits
	entry.Lines = req.Lines

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
		var requests []*CoverageEntryUploadRequest
		if merged == nil {
			requests = req.Entries
		} else {
			// rebuild upload request
			for _, e := range merged.entries {
				requests = append(requests,
					&CoverageEntryUploadRequest{
						EntryName: e.Name,
						Hits:      e.Hits,
						Lines:     e.Lines,
						Profiles:  pie.Values(e.profiles),
					})
			}
		}
		contents, err := json.Marshal(requests)
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
