package tracker

// CoverageLinkManager links a coverage-type tracker to a repository.
// It is implemented by the coverage package so that tracker_coverage rows
// are managed by coverage rather than tracker.
type CoverageLinkManager interface {
	FindRepoIDByTrackerID(trackerID int64) (*int64, error)
	Link(trackerID, repoID int64) error
	Unlink(trackerID int64) error
}
