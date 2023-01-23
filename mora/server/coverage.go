package server

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
		url      string
		revision string
		time     time.Time
		entries  []*CoverageEntry
	}
)

func (c *Coverage) RepoURL() string {
	return c.url
}

func (c *Coverage) Time() time.Time {
	return c.time
}

func (c *Coverage) Revision() string {
	return c.revision
}

func (c *Coverage) Entries() []*CoverageEntry {
	return c.entries
}

func (c *Coverage) FindEntry(name string) *CoverageEntry {
	for _, e := range c.Entries() {
		if e.Name == name {
			return e
		}
	}

	return nil
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
