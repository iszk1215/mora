package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/elliotchance/pie/v2"
	"github.com/iszk1215/mora/mora/profile"
)

type MoraCoverageProvider struct {
	coverages []*Coverage
	store     *CoverageStore
	sync.Mutex
}

func NewMoraCoverageProvider(store *CoverageStore) *MoraCoverageProvider {
	p := &MoraCoverageProvider{}
	p.store = store

	p.coverages = []*Coverage{}

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
func mergeEntry(a, b *CoverageEntry) *CoverageEntry {
	c := &CoverageEntry{Name: a.Name, profiles: map[string]*profile.Profile{}}

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

	merged := &Coverage{
		url:      a.url,
		revision: a.revision,
		time:     a.time,
		entries:  pie.Values(entries),
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

func (p *MoraCoverageProvider) Coverages() []*Coverage {
	return p.coverages
}

func (p *MoraCoverageProvider) Sync() error {
	return p.loadFromStore()
}

func parseScanedCoverage(record ScanedCoverage) (*Coverage, error) {
	var req []*CoverageEntryUploadRequest
	err := json.Unmarshal([]byte(record.Contents), &req)
	if err != nil {
		return nil, err
	}

	entries, err := parseEntries(req)
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

	profiles := map[string]*profile.Profile{}
	for _, p := range req.Profiles {
		profiles[p.FileName] = p
	}

	entry := &CoverageEntry{}
	entry.Name = req.EntryName
	entry.profiles = profiles
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

func (p *MoraCoverageProvider) HandleUploadRequest(req *CoverageUploadRequest) error {
	cov, err := parseCoverage(req)
	if err != nil {
		return err
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
			return err
		}

		err = p.store.Put(*cov, string(contents))
		if err != nil {
			return err
		}
	}

	return nil
}
