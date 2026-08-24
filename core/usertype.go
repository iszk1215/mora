package core

// User types. The user type controls per-user resource limits such as
// the maximum number of trackers a user can create (issue #179).
const (
	UserTypeFree  = "free"
	UserTypePro   = "pro"
	UserTypeAdmin = "admin"
)

// FreeUserMaxTrackers is the maximum number of trackers a free user can own.
const FreeUserMaxTrackers = 5

// IsValidUserType reports whether userType is one of the known user types.
func IsValidUserType(userType string) bool {
	switch userType {
	case UserTypeFree, UserTypePro, UserTypeAdmin:
		return true
	}
	return false
}

// MaxTrackersFor returns the maximum number of trackers a user of the
// given type may own. A negative value means unlimited.
func MaxTrackersFor(userType string) int {
	switch userType {
	case UserTypePro, UserTypeAdmin:
		return -1
	default:
		return FreeUserMaxTrackers
	}
}
