package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/elliotchance/pie/v2"
	"github.com/iszk1215/mora/mora/profile"
	"github.com/rs/zerolog/log"
)

type MoraCoverageProvider struct {
	coverages []*Coverage
	store     CoverageStore
	sync.Mutex
}

func NewMoraCoverageProvider(store CoverageStore) *MoraCoverageProvider {
	p := &MoraCoverageProvider{}
	p.store = store

	p.coverages = []*Coverage{}

	if p.store != nil {
		err := p.loadFromStore()
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}

	return p
}

func (p *MoraCoverageProvider) Coverages() []*Coverage {
	return p.coverages
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
func mergeEntry(a, b *CoverageEntry) *CoverageEntry {
	c := &CoverageEntry{Name: a.Name, Profiles: map[string]*profile.Profile{}}

	for file, p := range a.Profiles {
		c.Profiles[file] = p
	}

	for file, p := range b.Profiles {
		c.Profiles[file] = p
	}

	c.Hits = 0
	c.Lines = 0
	for _, p := range c.Profiles {
		c.Hits += p.Hits
		c.Lines += p.Lines
	}

	return c
}

func mergeCoverage(a, b *Coverage) (*Coverage, error) {
	if a.url != b.url || a.revision != b.revision {
		return nil, fmt.Errorf("can not merge two coverages with different urls and/or revisions")
	}

	entries := map[string]*CoverageEntry{}

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

	tmp := pie.Values(entries)
	sort.Slice(tmp, func(i, j int) bool {
		return tmp[i].Name < tmp[j].Name
	})

	merged := &Coverage{
		url:      a.url,
		revision: a.revision,
		time:     a.time,
		entries:  tmp,
	}

	return merged, nil
}

func (p *MoraCoverageProvider) addOrMergeCoverage(cov *Coverage) *Coverage {
	p.Lock()
	defer p.Unlock()

	idx := p.findCoverage(*cov)
	if idx < 0 {
		p.coverages = append(p.coverages, cov)
		return nil
	} else {
		merged, _ := mergeCoverage(p.coverages[idx], cov)
		p.coverages[idx] = merged
		return merged
	}
}

func parseScanedCoverageContents(contents string) ([]*CoverageEntry, error) {
	var req []*CoverageEntryUploadRequest

	err := json.Unmarshal([]byte(contents), &req)
	if err != nil {
		return nil, err
	}

	entries, err := parseEntries(req)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func parseScanedCoverage(record ScanedCoverage) (*Coverage, error) {
	entries, err := parseScanedCoverageContents(record.Contents)
	if err != nil {
		return nil, err
	}

	cov := &Coverage{}
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

func parseEntry(req *CoverageEntryUploadRequest) (*CoverageEntry, error) {
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

func parseEntries(req []*CoverageEntryUploadRequest) ([]*CoverageEntry, error) {
	entries := []*CoverageEntry{}
	for _, e := range req {
		entry, err := parseEntry(e)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func parseCoverage(req *CoverageUploadRequest) (*Coverage, error) {
	if req.RepoURL == "" {
		return nil, errors.New("repo url is empty")
	}

	entries, err := parseEntries(req.Entries)
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

func (p *MoraCoverageProvider) makeContents(cov *Coverage) ([]byte, error) {
	merged := p.addOrMergeCoverage(cov)

	var requests []*CoverageEntryUploadRequest
	if merged == nil {
		merged = cov
	}

	// rebuild upload request
	for _, e := range merged.entries {
		requests = append(requests,
			&CoverageEntryUploadRequest{
				EntryName: e.Name,
				Hits:      e.Hits,
				Lines:     e.Lines,
				Profiles:  pie.Values(e.Profiles),
			})
	}
	contents, err := json.Marshal(requests)
	if err != nil {
		return nil, err
	}

	return contents, nil
}

func (p *MoraCoverageProvider) AddCoverage(cov *Coverage) error {
	contents, err := p.makeContents(cov)
	if err != nil {
		return err
	}

	if p.store != nil {
		err = p.store.Put(*cov, string(contents))
	}

	return err

	return nil
}
