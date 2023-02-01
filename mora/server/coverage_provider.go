package server

import (
	"sync"

	"github.com/rs/zerolog/log"
)

type (
	CoverageStore interface {
		Put(*Coverage) error
		ListAll() ([]*Coverage, error)
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
		coverages, err := p.store.ListAll()
		if err != nil {
			log.Error().Err(err).Msg("Ignored")
		} else {
			p.coverages = coverages
		}
	}

	return p
}

func (p *MoraCoverageProvider) FindByRepoIDAndID(repo_id int64, id int64) *Coverage {
	for _, cov := range p.coverages {
		if cov.ID == id && cov.RepoID == repo_id {
			return cov
		}
	}
	return nil
}

func (p *MoraCoverageProvider) FindByRepoID(id int64) []*Coverage {
	found := []*Coverage{}
	for _, cov := range p.coverages {
		if cov.RepoID == id {
			found = append(found, cov)
		}
	}
	return found
}

func (p *MoraCoverageProvider) findCoverage(cov *Coverage) int {
	for i, c := range p.coverages {
		if c.RepoID == cov.RepoID && c.Revision == cov.Revision {
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
}
