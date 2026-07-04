package tracker

import "time"

type CoverageTimelinePoint struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

type CoverageTimelineProvider interface {
	Timeline(repoID int64, limit int) (map[string][]CoverageTimelinePoint, error)
}
