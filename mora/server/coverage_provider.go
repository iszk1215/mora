package server

import (
	"sync"

	"github.com/rs/zerolog/log"
)

type (
	CoverageStore interface {
		Put(*Coverage) error
		Find(int64) (*Coverage, error)
		FindRevision(int64, string) (*Coverage, error)
		List(int64) ([]*Coverage, error)
		ListAll() ([]*Coverage, error)
	}

	MoraCoverageProvider struct {
		store CoverageStore
		sync.Mutex
	}
)

func NewMoraCoverageProvider(store CoverageStore) *MoraCoverageProvider {
	p := &MoraCoverageProvider{}
	p.store = store

	return p
}

func (p *MoraCoverageProvider) addOrMergeCoverage(cov *Coverage) (*Coverage, error) {
	p.Lock()
	defer p.Unlock()

	if p.store == nil {
		return cov, nil
	}

	found, err := p.store.FindRevision(cov.RepoID, cov.Revision)
	if err != nil {
		return nil, err
	}

	if found == nil {
		return cov, nil
	}

	log.Print("Merge with ", found.ID)

	cov.ID = found.ID
	return mergeCoverage(found, cov)
}

func (p *MoraCoverageProvider) AddCoverage(cov *Coverage) error {
	log.Print("AddCoverage")
	cov, err := p.addOrMergeCoverage(cov)
	if err != nil {
		return err
	}

	if p.store == nil {
		return nil
	}

	log.Print("AddCoverage: Put: cov.ID=", cov.ID)

	return p.store.Put(cov)
}
