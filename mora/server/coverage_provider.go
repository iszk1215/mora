package server

import (
	"encoding/json"
	"sync"

	"github.com/rs/zerolog/log"
)

type (
	CoverageStore interface {
		Put(*Coverage) error
		Scan() ([]*Coverage, error)
	}

	MoraCoverageProvider struct {
		coverages []*Coverage
		store     CoverageStore
		sync.Mutex
	}
)

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

func (p *MoraCoverageProvider) FindByURLandID(url string, id int64) *Coverage {
	for _, cov := range p.coverages {
		if cov.ID == id && cov.RepoURL == url {
			return cov
		}
	}
	return nil
}

func (p *MoraCoverageProvider) FindByRepoURL(url string) []*Coverage {
	found := []*Coverage{}
	for _, cov := range p.coverages {
		if cov.RepoURL == url {
			found = append(found, cov)
		}
	}
	return found
}

// contents is serialized []CoverageEntryUploadRequest
func parseScanedCoverageContents(contents string) ([]*CoverageEntry, error) {
	var req []*CoverageEntryUploadRequest

	err := json.Unmarshal([]byte(contents), &req)
	if err != nil {
		return nil, err
	}

	entries, err := parseCoverageEntryUploadRequests(req)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

/*
func parseScanedCoverage(record ScanedCoverage) (*Coverage, error) {
	entries, err := parseScanedCoverageContents(record.Contents)
	if err != nil {
		return nil, err
	}

	cov := &Coverage{}
	cov.ID = record.ID
	cov.RepoID = record.RepoID
	cov.RepoURL = record.RepoURL
	cov.Revision = record.Revision
	cov.Entries = entries
	cov.Timestamp = record.Time

	return cov, nil
}
*/

func loadFromStore(store CoverageStore) ([]*Coverage, error) {
	/*
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
	*/
	return store.Scan()
}

func (p *MoraCoverageProvider) findCoverage(cov *Coverage) int {
	for i, c := range p.coverages {
		if c.RepoURL == cov.RepoURL && c.Revision == cov.Revision {
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
		cov.ID = p.coverages[idx].ID
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

	return p.store.Put(cov)

	/*
		var requests []*CoverageEntryUploadRequest
		for _, e := range cov.Entries {
			requests = append(requests,
				&CoverageEntryUploadRequest{
					Name:     e.Name,
					Hits:     e.Hits,
					Lines:    e.Lines,
					Profiles: pie.Values(e.Profiles),
				})
		}

		contents, err := json.Marshal(requests)
		if err != nil {
			return err
		}

		scaned := ScanedCoverage{
			RepoURL:  cov.RepoURL,
			Revision: cov.Revision,
			Time:     cov.Timestamp,
			Contents: string(contents),
		}

		err = p.store.Put(&scaned)
		if err != nil {
			return err
		}

		cov.ID = scaned.ID
		return nil
	*/
}
