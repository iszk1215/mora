package coverage

import (
	"fmt"
	"sort"
	"time"

	"github.com/elliotchance/pie/v2"
	"github.com/iszk1215/mora/mora/profile"
)

type (
	CoverageEntry struct {
		Name     string `json:"name"`
		Hits     int    `json:"hits"`
		Lines    int    `json:"lines"`
		Profiles map[string]*profile.Profile
	}

	Coverage struct {
		ID        int64
		RepoID    int64
		Revision  string
		Timestamp time.Time
		Entries   []*CoverageEntry
	}

	CoverageStore interface {
		Init() error
		Find(id int64) (*Coverage, error)
		FindRevision(id int64, revision string) (*Coverage, error)
		List(id int64) ([]*Coverage, error)
		ListAll() ([]*Coverage, error)
		Put(*Coverage) error
	}
)

func (c *Coverage) FindEntry(name string) *CoverageEntry {
	for _, e := range c.Entries {
		if e.Name == name {
			return e
		}
	}

	return nil
}

func mergeCoverage(a, b *Coverage) (*Coverage, error) {
	if a.RepoID != b.RepoID || a.Revision != b.Revision {
		return nil, fmt.Errorf("can not merge two coverages with different URLs and/or revisions")
	}

	entries := map[string]*CoverageEntry{}

	for _, e := range a.Entries {
		entries[e.Name] = e
	}

	for _, e := range b.Entries {
		_, ok := entries[e.Name]
		if ok {
			// want to replace?
			return nil, fmt.Errorf(
				"mergeCoverage: both coverage has the same entry: %s", e.Name)
		}
		entries[e.Name] = e
	}

	tmp := pie.Values(entries)
	sort.Slice(tmp, func(i, j int) bool {
		return tmp[i].Name < tmp[j].Name
	})

	merged := &Coverage{
		RepoID:    a.RepoID,
		Revision:  a.Revision,
		Timestamp: a.Timestamp,
		Entries:   tmp,
	}

	return merged, nil
}
