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

func (p *MoraCoverageProvider) AddCoverage(cov *Coverage) error {
	log.Print("AddCoverage")

	if p.store == nil {
		return nil
	}

	found, err := p.store.FindRevision(cov.RepoID, cov.Revision)
	if err != nil {
		return err
	}

	if found != nil {
		log.Print("Merge with ", found.ID)
		cov.ID = found.ID
		cov, err = mergeCoverage(found, cov)
		if err != nil {
			return err
		}
	}

	log.Print("AddCoverage: Put: cov.ID=", cov.ID)
	return p.store.Put(cov)
}
