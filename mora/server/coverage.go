package server

import (
	"time"

	"github.com/iszk1215/mora/mora/profile"
)

type CoverageEntry struct {
	Name     string `json:"name"`
	Hits     int    `json:"hits"`
	Lines    int    `json:"lines"`
	Profiles map[string]*profile.Profile
}

type Coverage struct {
	url      string
	revision string
	time     time.Time
	entries  []*CoverageEntry
}

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
