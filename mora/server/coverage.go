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
		URL       string
		Revision  string
		Timestamp time.Time
		Entries   []*CoverageEntry
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
	if a.URL != b.URL || a.Revision != b.Revision {
		return nil, fmt.Errorf("can not merge two coverages with different URLs and/or revisions")
	}

	entries := map[string]*CoverageEntry{}

	for _, e := range a.Entries {
		entries[e.Name] = e
	}

	for _, e := range b.Entries {
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
		URL:       a.URL,
		Revision:  a.Revision,
		Timestamp: a.Timestamp,
		Entries:   tmp,
	}

	return merged, nil
}
