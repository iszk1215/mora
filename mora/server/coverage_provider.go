package server

import (
	"encoding/json"
	"sync"

	"github.com/elliotchance/pie/v2"
	"github.com/rs/zerolog/log"
)

type MoraCoverageProvider struct {
	coverages []*Coverage
	store     CoverageStore
	sync.Mutex
}

// TODO: return error
func NewMoraCoverageProvider(store CoverageStore) *MoraCoverageProvider {
	p := &MoraCoverageProvider{}
	p.store = store

	p.coverages = []*Coverage{}

	if p.store != nil {
		coverages, err := loadFromStore(p.store)
		if err != nil {
			log.Error().Err(err).Msg("Ignored")
		} else {
			p.coverages = coverages
		}
	}

	return p
}

func (p *MoraCoverageProvider) Coverages() []*Coverage {
	return p.coverages
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

func loadFromStore(store CoverageStore) ([]*Coverage, error) {
	records, err := store.Scan()
	if err != nil {
		return nil, err
	}

	coverages := []*Coverage{}
	for _, record := range records {
		cov, err := parseScanedCoverage(record)
		if err != nil {
			return nil, err
		}

		coverages = append(coverages, cov)
	}

	return coverages, nil
}

func (p *MoraCoverageProvider) findCoverage(cov *Coverage) int {
	for i, c := range p.coverages {
		if c.RepoURL() == cov.RepoURL() && c.Revision() == cov.Revision() {
			return i
		}
	}

	return -1
}

func (p *MoraCoverageProvider) addOrMergeCoverage(cov *Coverage) *Coverage {
	p.Lock()
	defer p.Unlock()

	idx := p.findCoverage(cov)
	if idx < 0 {
		p.coverages = append(p.coverages, cov)
		return cov
	} else {
		merged, _ := mergeCoverage(p.coverages[idx], cov)
		p.coverages[idx] = merged
		return merged
	}
}

func (p *MoraCoverageProvider) AddCoverage(cov *Coverage) error {
	cov = p.addOrMergeCoverage(cov)

	if p.store == nil {
		return nil
	}

	var requests []*CoverageEntryUploadRequest
	for _, e := range cov.entries {
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
		return err
	}

	scaned := ScanedCoverage{
		RepoURL:  cov.RepoURL(),
		Revision: cov.Revision(),
		Time:     cov.Time(),
		Contents: string(contents),
	}

	return p.store.Put(scaned)
}
