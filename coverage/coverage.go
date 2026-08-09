package coverage

import (
	"fmt"
	"sort"
	"time"

	"github.com/elliotchance/pie/v2"
	"github.com/iszk1215/mora/core"
	"github.com/iszk1215/mora/coverage/profile"
)

type (
	CoverageTimelinePoint struct {
		Time  time.Time `json:"time"`
		Value float64   `json:"value"`
	}

	CoverageEntry struct {
		Name     string `json:"name"`
		Hits     int    `json:"hits"`
		Lines    int    `json:"lines"`
		Profiles map[string]*profile.Profile
	}

	Coverage struct {
		ID        int64
		TrackerID int64
		Revision  string
		Timestamp time.Time
		Entries   []*CoverageEntry
	}

	CoverageStore interface {
		Init() error
		Find(id int64) (*Coverage, error)
		FindRevision(id int64, revision string) (*Coverage, error)
		List(id int64) ([]*Coverage, error)
		Put(*Coverage) (int64, error)
		Timeline(trackerID int64, limit int) (map[string][]CoverageTimelinePoint, error)
		FindRepoByTrackerID(trackerID int64) (*core.Repository, error)
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
	if a.TrackerID != b.TrackerID || a.Revision != b.Revision {
		return nil, fmt.Errorf("can not merge two coverages with different trackers and/or revisions")
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
		TrackerID: a.TrackerID,
		Revision:  a.Revision,
		Timestamp: b.Timestamp,
		Entries:   tmp,
	}

	return merged, nil
}
